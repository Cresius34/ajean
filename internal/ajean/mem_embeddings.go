package ajean

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

type memoryDocument struct{ File, Title, Text, Hash, MediaHash string }
type memoryChunk struct {
	File, Title, Text, Key string
	Start, End             int
}
type embeddingIndex struct {
	Fingerprint string
	Hashes      map[string]string
	Vectors     map[string][]float32
	Updated     time.Time
}
type embeddingJob struct {
	Root        string `json:"project_path"`
	Running     bool   `json:"running"`
	Done, Total int
	Error       string `json:"error,omitempty"`
}

// Derived, bounded, RAM-only cache. Neither plaintext snippets nor vectors go
// into the DB/backups. Locking the memory invalidates even in-flight jobs.
var embeddingCache = struct {
	sync.Mutex
	Epoch   uint64
	Indexes map[string]*embeddingIndex
	Job     embeddingJob
	Note    string
}{Indexes: make(map[string]*embeddingIndex)}

func clearEmbeddingCache() {
	embeddingCache.Lock()
	embeddingCache.Epoch++
	embeddingCache.Indexes = make(map[string]*embeddingIndex)
	embeddingCache.Unlock()
}

var ignoredMemoryName = regexp.MustCompile(`(?i)(^|[._ -])(archive[s]?|backup|bak|old|corrupted|copy|copie|temp|tmp)([._ -]|$)`)

func canonicalMemoryName(name string) bool {
	return strings.HasSuffix(strings.ToLower(name), ".md") && !strings.EqualFold(name, "MEMORY.md") &&
		!strings.HasPrefix(name, ".") && !ignoredMemoryName.MatchString(name)
}

func memoryDigest(s string) string { h := sha256.Sum256([]byte(s)); return hex.EncodeToString(h[:]) }

// Read against a captured absolute project directory, never the mutable active
// project during an asynchronous job. Rehash every search, including external
// Drive edits with the same size/timestamp. Symlinks are not followed.
func loadMemoryDocuments(root string) ([]memoryDocument, error) {
	return loadMemoryDocumentsFor(root, readEmbeddingConfig())
}

func loadMemoryDocumentsFor(root string, cfg embeddingConfig) ([]memoryDocument, error) {
	if memLocked() {
		return nil, errMemLocked
	}
	entries, err := os.ReadDir(root)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var docs []memoryDocument
	for _, e := range entries {
		if !canonicalMemoryName(e.Name()) || !e.Type().IsRegular() {
			continue
		}
		fi, err := e.Info()
		if err != nil {
			return nil, err
		}
		if fi.Size() > 2<<20 {
			return nil, fmt.Errorf("page %s trop grande pour cet essai (2 Mio max)", e.Name())
		}
		raw, err := os.ReadFile(filepath.Join(root, e.Name()))
		if err != nil {
			return nil, err
		}
		plain, err := decodeMemContent(raw)
		if err != nil {
			return nil, err
		}
		_, mediaHash, err := embeddingMedia(root, string(plain), cfg)
		if err != nil {
			return nil, err
		}
		docs = append(docs, memoryDocument{e.Name(), titleOf(string(plain)), string(plain), memoryDigest(string(raw) + mediaHash), mediaHash})
	}
	return docs, nil
}

func chunkMemory(docs []memoryDocument) []memoryChunk {
	var chunks []memoryChunk
	for _, doc := range docs {
		lines := strings.Split(doc.Text, "\n")
		heading := doc.Title
		start, chars := 0, 0
		var buf []string
		flush := func(end int) {
			text := strings.TrimSpace(strings.Join(buf, "\n"))
			if text != "" {
				key := memoryDigest(embeddingProtocol + "\x00" + doc.File + "\x00" + heading + "\x00" + text + doc.MediaHash)
				chunks = append(chunks, memoryChunk{doc.File, heading, text, key, start + 1, end})
			}
			buf, chars = nil, 0
		}
		for i, line := range lines {
			trim := strings.TrimSpace(line)
			if strings.HasPrefix(trim, "#") && len(buf) > 0 {
				flush(i)
			}
			if strings.HasPrefix(trim, "#") {
				heading = strings.TrimSpace(strings.TrimLeft(trim, "#"))
			}
			runes := []rune(line)
			// Also split a very long single line; source ranges remain valid.
			for len(runes) > 1800 {
				if len(buf) > 0 {
					flush(i)
				}
				start = i
				buf = []string{string(runes[:1800])}
				flush(i + 1)
				runes = runes[1800:]
			}
			if chars+len(runes)+1 > 1800 && len(buf) > 0 {
				flush(i)
			}
			if len(buf) == 0 {
				start = i
			}
			buf = append(buf, string(runes))
			chars += len(runes) + 1
		}
		if len(buf) > 0 {
			flush(len(lines))
		}
	}
	return chunks
}

func embeddingDocumentInput(c memoryChunk) string {
	return "title: " + c.File + " — " + c.Title + " | text: " + c.Text
}
func embeddingQueryInput(query string) string {
	return "task: search result | query: " + strings.TrimSpace(query)
}

func snapshotHashes(docs []memoryDocument) map[string]string {
	m := make(map[string]string, len(docs))
	for _, d := range docs {
		m[d.File] = d.Hash
	}
	return m
}
func indexFresh(idx *embeddingIndex, docs []memoryDocument, fp string) bool {
	if idx == nil || idx.Fingerprint != fp || len(idx.Hashes) != len(docs) {
		return false
	}
	for _, d := range docs {
		if idx.Hashes[d.File] != d.Hash {
			return false
		}
	}
	return true
}

// A single worker bounds CPU/RAM use. Capture root/config at launch; a project
// switch cannot redirect it. Unchanged chunks reuse vectors, deleted ones drop.
func queueEmbeddingIndex(root string, cfg embeddingConfig) bool {
	embeddingCache.Lock()
	if embeddingCache.Job.Running {
		embeddingCache.Unlock()
		return false
	}
	epoch := embeddingCache.Epoch
	embeddingCache.Job = embeddingJob{Root: root, Running: true}
	embeddingCache.Unlock()
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
		defer cancel()
		_, err := rebuildEmbeddingIndex(ctx, root, cfg, epoch)
		embeddingCache.Lock()
		embeddingCache.Job.Running = false
		if err != nil {
			embeddingCache.Job.Error = err.Error()
		}
		embeddingCache.Unlock()
	}()
	return true
}

func rebuildEmbeddingIndex(ctx context.Context, root string, cfg embeddingConfig, epoch uint64) (*embeddingIndex, error) {
	if memLocked() {
		return nil, errMemLocked
	}
	startCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	client, err := startEmbeddingServer(startCtx, cfg)
	cancel()
	if err != nil {
		return nil, err
	}
	docs, err := loadMemoryDocumentsFor(root, cfg)
	if err != nil {
		return nil, err
	}
	chunks := chunkMemory(docs)
	if len(chunks) > 12000 {
		return nil, errors.New("plus de 12000 blocs : réduire la mémoire de ce projet pour cet essai")
	}
	embeddingCache.Lock()
	old := embeddingCache.Indexes[root]
	embeddingCache.Job.Total = len(chunks)
	embeddingCache.Unlock()
	idx := &embeddingIndex{Fingerprint: client.Fingerprint, Hashes: snapshotHashes(docs), Vectors: make(map[string][]float32), Updated: time.Now()}
	for i, c := range chunks {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		embeddingCache.Lock()
		valid := embeddingCache.Epoch == epoch
		embeddingCache.Unlock()
		if !valid || memLocked() {
			return nil, errors.New("indexation annulée (verrouillage ou configuration modifiée)")
		}
		if old != nil && old.Fingerprint == client.Fingerprint {
			idx.Vectors[c.Key] = old.Vectors[c.Key]
		}
		if idx.Vectors[c.Key] == nil {
			input, _, err := embeddingMedia(root, c.Text, cfg)
			if err != nil {
				return nil, err
			}
			input.Text = "title: " + c.File + " — " + c.Title + " | text: " + input.Text
			v, err := client.embedInput(ctx, input)
			if err != nil {
				return nil, err
			}
			idx.Vectors[c.Key] = v[0]
		}
		embeddingCache.Lock()
		embeddingCache.Job.Done = i + 1
		embeddingCache.Unlock()
	}
	// Only publish if the source still matches. No stale snippets or vectors can
	// become current just because a Drive edit occurred during encoding.
	current, err := loadMemoryDocumentsFor(root, cfg)
	if err != nil {
		return nil, err
	}
	if !indexFresh(idx, current, client.Fingerprint) {
		return nil, errors.New("fichiers modifiés pendant l’indexation ; relancer")
	}
	embeddingCache.Lock()
	defer embeddingCache.Unlock()
	if epoch != embeddingCache.Epoch || memLocked() {
		return nil, errors.New("indexation annulée")
	}
	// At most four projects are retained in this first RAM-only implementation.
	if len(embeddingCache.Indexes) >= 4 {
		embeddingCache.Indexes = make(map[string]*embeddingIndex)
	}
	embeddingCache.Indexes[root] = idx
	return idx, nil
}

type memorySearchResult struct {
	Hits []MemHit `json:"hits"`
	Mode string   `json:"mode"`
	Note string   `json:"note,omitempty"`
}

func SearchMemory(ctx context.Context, query string, limit int) memorySearchResult {
	root, _ := filepath.Abs(memoryDir()) // capture project BEFORE any background work
	cfg := readEmbeddingConfig()
	lex := safeLexicalMemorySearch(root, query, 30)
	if limit <= 0 || limit > 30 {
		limit = 8
	}
	if !cfg.Enabled {
		if len(lex) > limit {
			lex = lex[:limit]
		}
		return memorySearchResult{Hits: lex, Mode: "lexical"}
	}
	filtered := lex[:0]
	for _, h := range lex {
		if canonicalMemoryName(h.File) {
			filtered = append(filtered, h)
		}
	}
	lex = filtered
	fallback := func(note string) memorySearchResult {
		if memLocked() {
			lex = safeLexicalMemorySearch(root, query, 30)
		}
		embeddingCache.Lock()
		embeddingCache.Note = note
		embeddingCache.Unlock()
		hits := lex
		if len(hits) > limit {
			hits = hits[:limit]
		}
		return memorySearchResult{Hits: hits, Mode: "lexical", Note: note}
	}
	if memLocked() {
		clearEmbeddingCache()
		return fallback("mémoire verrouillée ; recherche par nom seulement")
	}
	if strings.TrimSpace(query) == "" {
		return fallback("requête vide")
	}
	client, err := readyEmbeddingClient(cfg)
	if err != nil {
		if validateEmbeddingConfig(cfg) == nil {
			queueEmbeddingIndex(root, cfg)
		}
		return fallback(err.Error())
	}
	docs, err := loadMemoryDocumentsFor(root, cfg)
	if err != nil {
		return fallback(err.Error())
	}
	embeddingCache.Lock()
	idx := embeddingCache.Indexes[root]
	epoch := embeddingCache.Epoch
	embeddingCache.Unlock()
	if !indexFresh(idx, docs, client.Fingerprint) {
		queueEmbeddingIndex(root, cfg)
		return fallback("index absent ou modifié ; actualisation en arrière-plan")
	}
	queryCtx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	v, err := client.embed(queryCtx, []string{embeddingQueryInput(query)})
	if err != nil {
		return fallback("encodeur indisponible : " + err.Error())
	}
	// Recheck after the network operation so neither locking nor external edits
	// can expose a previous snippet. File changes fall back to fresh lexical data.
	if memLocked() {
		clearEmbeddingCache()
		return memorySearchResult{Mode: "lexical", Note: "mémoire verrouillée"}
	}
	current, err := loadMemoryDocumentsFor(root, cfg)
	embeddingCache.Lock()
	valid := epoch == embeddingCache.Epoch
	embeddingCache.Unlock()
	if err != nil || !valid || !indexFresh(idx, current, client.Fingerprint) {
		return memorySearchResult{Hits: safeLexicalMemorySearch(root, query, limit), Mode: "lexical", Note: "source modifiée pendant la recherche ; relancer"}
	}
	hits := rankHybrid(lex, chunkMemory(current), idx, v[0], limit)
	embeddingCache.Lock()
	embeddingCache.Note = ""
	embeddingCache.Unlock()
	return memorySearchResult{Hits: hits, Mode: "hybrid"}
}

// In particular, mixed clear/encrypted files during a migration must not let
// a locked vault expose cached or still-clear snippets through semantic search.
func safeLexicalMemorySearch(root, query string, limit int) []MemHit {
	if !memLocked() {
		return memSearchIn(root, query, limit)
	}
	entries, _ := os.ReadDir(root)
	terms := uniqueTerms(strings.ToLower(query))
	var hits []MemHit
	for _, e := range entries {
		if !e.Type().IsRegular() || !canonicalMemoryName(e.Name()) {
			continue
		}
		matched := len(terms) == 0
		for _, term := range terms {
			if strings.Contains(strings.ToLower(e.Name()), term) {
				matched = true
			}
		}
		if matched {
			hits = append(hits, MemHit{File: e.Name(), Title: "🔒 (mémoire verrouillée)"})
		}
		if len(hits) >= limit {
			break
		}
	}
	return hits
}

func rankHybrid(lex []MemHit, chunks []memoryChunk, idx *embeddingIndex, q []float32, limit int) []MemHit {
	type scored struct {
		hit   MemHit
		score float64
	}
	best := map[string]scored{}
	for _, c := range chunks {
		v := idx.Vectors[c.Key]
		if len(v) != len(q) {
			continue
		}
		sim := 0.0
		for i, x := range q {
			sim += float64(x) * float64(v[i])
		}
		if sim <= 0 {
			continue
		}
		if prev, ok := best[c.File]; ok && prev.score >= sim {
			continue
		}
		snippet := []rune(strings.Join(strings.Fields(c.Text), " "))
		if len(snippet) > 320 {
			snippet = append(snippet[:320], '…')
		}
		best[c.File] = scored{MemHit{File: c.File, Title: c.Title, Snippet: string(snippet), StartLine: c.Start, EndLine: c.End}, sim}
	}
	semantic := make([]scored, 0, len(best))
	for _, s := range best {
		semantic = append(semantic, s)
	}
	sort.Slice(semantic, func(i, j int) bool {
		if semantic[i].score == semantic[j].score {
			return semantic[i].hit.File < semantic[j].hit.File
		}
		return semantic[i].score > semantic[j].score
	})
	if len(semantic) > 30 {
		semantic = semantic[:30]
	}
	fused := map[string]scored{}
	for i, h := range lex {
		fused[h.File] = scored{h, 1 / float64(60+i+1)}
	}
	for i, s := range semantic {
		f := fused[s.hit.File]
		f.score += 1 / float64(60+i+1)
		f.hit = s.hit
		fused[s.hit.File] = f
	}
	all := make([]scored, 0, len(fused))
	for _, s := range fused {
		all = append(all, s)
	}
	sort.Slice(all, func(i, j int) bool {
		if all[i].score == all[j].score {
			return all[i].hit.File < all[j].hit.File
		}
		return all[i].score > all[j].score
	})
	var out []MemHit
	for i, s := range all {
		if i >= limit {
			break
		}
		out = append(out, s.hit)
	}
	return out
}
