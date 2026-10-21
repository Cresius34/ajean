package ajean

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestEmbeddingClientPrefixesAndValidation(t *testing.T) {
	chunk := memoryChunk{File: "chaos.md", Title: "Ancrage", Text: "Texte"}
	if got := embeddingDocumentInput(chunk); got != "title: chaos.md — Ancrage | text: Texte" {
		t.Fatal(got)
	}
	if got := embeddingQueryInput("  bâtiment debout  "); got != "task: search result | query: bâtiment debout" {
		t.Fatal(got)
	}
	for _, tc := range []struct {
		name       string
		dimensions int
		bad        string
		wantError  bool
	}{
		{"normalisé", 768, "", false}, {"mauvais modèle", 384, "", true}, {"nul", 768, "zero", true}, {"réponse incomplète", 768, "missing", true}, {"index dupliqué", 768, "duplicate", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/v1/embeddings" || r.Header.Get("Authorization") != "Bearer secret" {
					t.Error("endpoint/auth")
				}
				var body struct {
					Input []string `json:"input"`
				}
				json.NewDecoder(r.Body).Decode(&body)
				if len(body.Input) != 1 || body.Input[0] != embeddingQueryInput("bâtiment") {
					t.Error(body)
				}
				v := make([]float32, tc.dimensions)
				if tc.bad != "zero" {
					v[0] = 3
					v[1] = 4
				}
				data := []map[string]any{{"index": 0, "embedding": v}}
				if tc.bad == "missing" {
					data = nil
				}
				if tc.bad == "duplicate" {
					data = append(data, data[0])
				}
				json.NewEncoder(w).Encode(map[string]any{"data": data})
			}))
			defer s.Close()
			c := &embeddingHTTPClient{URL: s.URL, Key: "secret", HTTP: s.Client()}
			v, err := c.embed(context.Background(), []string{embeddingQueryInput("bâtiment")})
			if (err != nil) != tc.wantError {
				t.Fatalf("vectors=%v err=%v", len(v), err)
			}
			if err == nil && (v[0][0] < .599 || v[0][0] > .601 || v[0][1] < .799 || v[0][1] > .801) {
				t.Fatal("vecteur non normalisé")
			}
		})
	}
}

func TestEmbeddingClientDeadlineAndNonfinite(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.RawQuery, "nan") {
			ioJSON := `{"data":[{"index":0,"embedding":[NaN]}]}`
			w.Write([]byte(ioJSON))
			return
		}
		select {
		case <-r.Context().Done():
		case <-time.After(120 * time.Millisecond):
		}
	}))
	defer s.Close()
	c := &embeddingHTTPClient{URL: s.URL, HTTP: s.Client()}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	if _, err := c.embed(ctx, []string{"test"}); err == nil {
		t.Fatal("timeout ignoré")
	}
	// JSON cannot legally carry NaN. A malformed provider must fail closed.
	c.URL = s.URL + "?nan"
	ctx2, cancel2 := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel2()
	if _, err := c.embed(ctx2, []string{"test"}); err == nil {
		t.Fatal("NaN accepté")
	}
}

func TestEmbeddingChunksScopeAndExternalEdits(t *testing.T) {
	home := testHome(t)
	a := filepath.Join(home, "memory", "a")
	b := filepath.Join(home, "memory", "b")
	os.MkdirAll(a, 0o700)
	os.MkdirAll(b, 0o700)
	for name, text := range map[string]string{"MEMORY.md": "# Routeur", "note.bak.md": "ancien", "x-OLD.md": "ancien", "chaos.md": "# Titre\n## Ancrage\nUne base immobile.\n" + strings.Repeat("z", 4000)} {
		if err := os.WriteFile(filepath.Join(a, name), []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	os.WriteFile(filepath.Join(b, "secret.md"), []byte("secret projet b"), 0o600)
	// Capturing a root remains correct even after active-project mutation.
	setProjectOverride("b")
	defer setProjectOverride("")
	docs, err := loadMemoryDocuments(a)
	if err != nil || len(docs) != 1 || docs[0].File != "chaos.md" {
		t.Fatalf("docs=%+v err=%v", docs, err)
	}
	chunks := chunkMemory(docs)
	for _, c := range chunks {
		if c.Start < 1 || c.End < c.Start || len([]rune(c.Text)) > 1800 {
			t.Fatalf("bloc invalide : %+v", c)
		}
	}
	idx := &embeddingIndex{Fingerprint: "f", Hashes: snapshotHashes(docs)}
	if !indexFresh(idx, docs, "f") || indexFresh(idx, docs, "changed-model") {
		t.Fatal("empreinte ignorée")
	}
	fi, _ := os.Stat(filepath.Join(a, "chaos.md"))
	raw, _ := os.ReadFile(filepath.Join(a, "chaos.md"))
	raw[len(raw)-1] = 'y'
	os.WriteFile(filepath.Join(a, "chaos.md"), raw, 0o600)
	os.Chtimes(filepath.Join(a, "chaos.md"), fi.ModTime(), fi.ModTime())
	changed, _ := loadMemoryDocuments(a)
	if indexFresh(idx, changed, "f") {
		t.Fatal("édition Drive de même taille/date manquée")
	}
	os.Remove(filepath.Join(a, "chaos.md"))
	deleted, _ := loadMemoryDocuments(a)
	if indexFresh(idx, deleted, "f") {
		t.Fatal("suppression non détectée")
	}
}

func TestEmbeddingDisabledAndLockFallback(t *testing.T) {
	testHome(t)
	clearEmbeddingCache()
	defer clearEmbeddingCache()
	if err := MemAdd("confidentiel.md", "# Secret\nLa phrase privée confidentielle."); err != nil {
		t.Fatal(err)
	}
	want := MemSearch("confidentiel", 8)
	got := SearchMemory(context.Background(), "confidentiel", 8)
	if got.Mode != "lexical" || got.Note != "" || !reflect.DeepEqual(got.Hits, want) {
		t.Fatalf("repli désactivé : %+v", got)
	}
	putJSON(bkState, "memory_embeddings", embeddingConfig{Enabled: true, Port: 18081, Threads: 8})
	got = SearchMemory(context.Background(), "confidentiel", 8)
	if got.Mode != "lexical" || got.Note == "" || len(got.Hits) != 1 {
		t.Fatalf("encodeur absent : %+v", got)
	}
	embeddingCache.Lock()
	embeddingCache.Indexes["test"] = &embeddingIndex{Vectors: map[string][]float32{"secret": {1}}}
	embeddingCache.Unlock()
	SetConfigKey("MEM_ENCRYPTED", "1")
	clearMemDEK()
	got = SearchMemory(context.Background(), "confidentiel", 8)
	for _, h := range got.Hits {
		if h.Snippet != "" || strings.Contains(h.Title, "Secret") {
			t.Fatalf("fuite verrouillée : %+v", h)
		}
	}
	embeddingCache.Lock()
	n := len(embeddingCache.Indexes)
	embeddingCache.Unlock()
	if n != 0 {
		t.Fatal("cache non purgé")
	}
	SetConfigKey("MEM_ENCRYPTED", "")
}

func TestEmbeddingHybridDeduplicatesAndUsesSourceLines(t *testing.T) {
	chunks := []memoryChunk{{File: "ancrage.md", Title: "Ancrage", Text: "Base immobile", Key: "a", Start: 4, End: 8}, {File: "ancrage.md", Title: "Autre", Text: "Bloc secondaire", Key: "b", Start: 9, End: 10}, {File: "couleur.md", Title: "Teinte", Text: "Texture", Key: "c", Start: 1, End: 2}}
	idx := &embeddingIndex{Vectors: map[string][]float32{"a": {1, 0}, "b": {.8, .6}, "c": {0, 1}}}
	hits := rankHybrid([]MemHit{{File: "ancrage.md", Title: "Ancrage"}}, chunks, idx, []float32{1, 0}, 8)
	if len(hits) != 1 || hits[0].File != "ancrage.md" || hits[0].StartLine != 4 || hits[0].EndLine != 8 {
		t.Fatalf("%+v", hits)
	}
}

func TestParallelProfileBlocksProductionControls(t *testing.T) {
	if parallelHome(filepath.Join("parent", "Ajean-Test", "app.exe")) != filepath.Join("parent", "Ajean-Test", "data") {
		t.Fatal("home")
	}
	for _, cmd := range []string{"install", "uninstall", "update", "start", "stop", "restart", "ui", "link"} {
		if parallelCommandAllowed(cmd) {
			t.Fatal(cmd)
		}
	}
	for _, path := range []string{"/api/update/apply", "/api/network", "/api/cloud/accounts/add", "/api/link/start"} {
		hit := false
		h := parallelHTTPHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hit = true }))
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("POST", path, strings.NewReader("{}")))
		if hit || w.Code != 403 {
			t.Fatalf("commande non bloquée %s : %d", path, w.Code)
		}
	}
	for _, path := range []string{"/api/presets", "/api/preset/save", "/api/preset/delete", "/api/start", "/api/stop", "/api/restart", "/api/llamacpp", "/api/llamacpp/install", "/api/llamacpp/update", "/api/llamacpp/prebuilt", "/api/llamacpp/use", "/api/switch", "/api/config"} {
		hit := false
		h := parallelHTTPHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hit = true }))
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("POST", path, strings.NewReader("{}")))
		if !hit {
			t.Fatalf("gestion native encore bloquée : %s (%d)", path, w.Code)
		}
	}
	args := strings.Join(embeddingArgs(embeddingConfig{Model: "m", Threads: 8, Port: 18081}, "key"), " ")
	if !strings.Contains(args, "--device none --n-gpu-layers 0") || strings.Contains(args, "mmproj") {
		t.Fatal(args)
	}
}

// Real GGUF integration is opt-in to avoid a 310 MB download in ordinary CI.
// The distributed prototype is validated with this test and the official CPU
// engine. It covers actual vectors, ranking, synchronized edits and deletion.
func TestEmbeddingGGUFIntegration(t *testing.T) {
	bin, model := os.Getenv("AJEAN_TEST_EMBED_BIN"), os.Getenv("AJEAN_TEST_EMBED_MODEL")
	if bin == "" || model == "" {
		t.Skip("set AJEAN_TEST_EMBED_BIN and AJEAN_TEST_EMBED_MODEL")
	}
	testHome(t)
	stopEmbeddingServer()
	defer stopEmbeddingServer()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()
	cfg := embeddingConfig{Enabled: true, Bin: bin, Model: model, Port: port, Threads: 4}
	if err = putJSON(bkState, "memory_embeddings", cfg); err != nil {
		t.Fatal(err)
	}
	MemAdd("resistance.md", "# Tenue de la structure\nLes ancrages fixent les clusters inférieurs de la GeometryCollection. Les seuils de rupture définissent la résistance aux impacts. Le socle reste immobile pendant la simulation Chaos.")
	MemAdd("apparence.md", "# Apparence de surface\nLe Material Instance règle les textures, la rugosité et la couleur des murs.")
	MemAdd("disque.md", "# Écriture sur disque\nSauvegarder les assets puis recharger la carte pour contrôler leur persistance.")
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	root, _ := filepath.Abs(memoryDir())
	embeddingCache.Lock()
	epoch := embeddingCache.Epoch
	embeddingCache.Unlock()
	idx, err := rebuildEmbeddingIndex(ctx, root, cfg, epoch)
	if err != nil {
		t.Fatal(err)
	}
	if len(idx.Vectors) != 3 {
		t.Fatalf("vecteurs : %d", len(idx.Vectors))
	}
	r := SearchMemory(ctx, "Comment empêcher un bâtiment de s’écrouler après un choc ?", 5)
	if r.Mode != "hybrid" || len(r.Hits) == 0 || r.Hits[0].File != "resistance.md" || r.Hits[0].StartLine < 1 {
		t.Fatalf("recherche réelle : %+v", r)
	}
	t.Logf("GGUF réel : mode=%s premier=%s lignes=%d-%d", r.Mode, r.Hits[0].File, r.Hits[0].StartLine, r.Hits[0].EndLine)
	// A synchronous external edit cannot reuse the stale semantic index.
	os.WriteFile(filepath.Join(root, "resistance.md"), []byte("# Révision externe\nUne information totalement différente."), 0o600)
	r = SearchMemory(ctx, "bâtiment", 5)
	if r.Mode != "lexical" || !strings.Contains(r.Note, "actualisation") {
		t.Fatalf("édition externe : %+v", r)
	}
	deadline := time.Now().Add(30 * time.Second)
	for {
		embeddingCache.Lock()
		running := embeddingCache.Job.Running
		jobErr := embeddingCache.Job.Error
		embeddingCache.Unlock()
		if !running {
			if jobErr != "" {
				t.Fatal(jobErr)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("indexation trop longue")
		}
		time.Sleep(20 * time.Millisecond)
	}
	os.Remove(filepath.Join(root, "resistance.md"))
	r = SearchMemory(ctx, "bâtiment", 5)
	for _, h := range r.Hits {
		if h.File == "resistance.md" {
			t.Fatal("note supprimée encore trouvée")
		}
	}
	// Let the automatic worker finish before testHome tears down its directory.
	for {
		embeddingCache.Lock()
		running := embeddingCache.Job.Running
		embeddingCache.Unlock()
		if !running {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("worker")
		}
		time.Sleep(20 * time.Millisecond)
	}
}
