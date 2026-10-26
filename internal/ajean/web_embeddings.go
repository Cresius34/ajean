package ajean

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func embeddingStatus() map[string]any {
	executable, _ := os.Executable()
	gb, gm := embeddingDefaultPaths("llama_cpp")
	pb, pm := embeddingDefaultPaths("transformers")
	root, _ := filepath.Abs(memoryDir())
	embeddingServer.Lock()
	running, ready := false, false
	if p := embeddingServer.p; p != nil {
		select {
		case <-p.done:
		default:
			running = true
			ready = p.ready
		}
	}
	embeddingServer.Unlock()
	embeddingCache.Lock()
	idx := embeddingCache.Indexes[root]
	pages, chunks := 0, 0
	if idx != nil {
		pages = len(idx.Hashes)
		chunks = len(idx.Vectors)
	}
	job, note := embeddingCache.Job, embeddingCache.Note
	embeddingCache.Unlock()
	return map[string]any{"config": readEmbeddingConfig(), "running": running, "ready": ready, "pages": pages, "chunks": chunks,
		"job": job, "note": note, "project": activeProjectSlug(), "memory_path": root, "home": AjeanHome(),
		"parallel": parallelBuild(), "version": displayVersion(), "executable": executable, "memory_ui_revision": parallelMemoryRevision, "locked": memLocked(), "cache": "RAM uniquement (reconstruit au lancement)",
		"chat_url": ReadConfig()[extKeyURL], "defaults": readEmbeddingDefaults(), "preset_memory_system": ReadConfig()["MEMORY_SYSTEM"],
		"default_paths": map[string]any{"llama_cpp": map[string]string{"bin": gb, "model": gm}, "transformers": map[string]string{"bin": pb, "model": pm}},
		"providers":     map[string]any{"llama_cpp": map[string]any{"modalities": []string{"text"}}, "transformers": map[string]any{"modalities": []string{"text", "image", "audio", "video"}}}}
}

func decodeEmbeddingRequest(w http.ResponseWriter, r *http.Request, dst any) bool {
	if r.Method != "POST" {
		sendJSON(w, 405, map[string]any{"error": "POST requis"})
		return false
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 32<<10)).Decode(dst); err != nil {
		sendJSON(w, 400, map[string]any{"error": "JSON invalide"})
		return false
	}
	return true
}

func handleEmbeddingSettings(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		w.Header().Set("Cache-Control", "no-store, max-age=0")
		sendJSON(w, 200, embeddingStatus())
		return
	}
	var c embeddingConfig
	if !decodeEmbeddingRequest(w, r, &c) {
		return
	}
	if err := validateEmbeddingConfig(c); err != nil {
		sendJSON(w, 400, map[string]any{"error": err.Error()})
		return
	}
	if err := saveEmbeddingConfig(c); err != nil {
		sendJSON(w, 500, map[string]any{"error": err.Error()})
		return
	}
	stopEmbeddingServer()
	if c = readEmbeddingConfig(); c.Enabled && validateEmbeddingConfig(c) == nil {
		root, _ := filepath.Abs(memoryDir())
		queueEmbeddingIndex(root, c)
	}
	sendJSON(w, 200, embeddingStatus())
}

func handleEmbeddingMode(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Mode string `json:"mode"`
	}
	if !decodeEmbeddingRequest(w, r, &req) {
		return
	}
	if req.Mode != "classic" && req.Mode != "hybrid" {
		sendJSON(w, 400, map[string]any{"error": "choisir classic ou hybrid"})
		return
	}
	c := readEmbeddingDefaults()
	c.Enabled = req.Mode == "hybrid"
	if err := saveEmbeddingConfig(c); err != nil {
		sendJSON(w, 400, map[string]any{"error": err.Error()})
		return
	}
	stopEmbeddingServer()
	c = readEmbeddingConfig()
	if c.Enabled && validateEmbeddingConfig(c) == nil {
		root, _ := filepath.Abs(memoryDir())
		queueEmbeddingIndex(root, c)
	}
	sendJSON(w, 200, embeddingStatus())
}

func handleEmbeddingAction(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Action string `json:"action"`
	}
	if !decodeEmbeddingRequest(w, r, &req) {
		return
	}
	switch req.Action {
	case "start", "index":
		c := readEmbeddingConfig()
		if !c.Enabled {
			sendJSON(w, 400, map[string]any{"error": "Activer puis enregistrer l’encodeur."})
			return
		}
		if memLocked() {
			sendJSON(w, 423, map[string]any{"error": "Déverrouiller la mémoire dans l’interface de chat."})
			return
		}
		root, _ := filepath.Abs(memoryDir())
		if !queueEmbeddingIndex(root, c) {
			sendJSON(w, 409, map[string]any{"error": "Une indexation est déjà en cours."})
			return
		}
	case "stop":
		stopEmbeddingServer()
	default:
		sendJSON(w, 400, map[string]any{"error": "Action inconnue"})
		return
	}
	sendJSON(w, 200, embeddingStatus())
}

func handleEmbeddingSearch(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Query string `json:"query"`
		Limit int    `json:"limit"`
	}
	if !decodeEmbeddingRequest(w, r, &req) {
		return
	}
	if len([]rune(req.Query)) > 4000 {
		sendJSON(w, 400, map[string]any{"error": "Requête trop longue"})
		return
	}
	start := time.Now()
	result := SearchMemory(r.Context(), req.Query, req.Limit)
	lim := req.Limit
	if lim <= 0 || lim > 30 {
		lim = 8
	}
	sendJSON(w, 200, map[string]any{"result": result, "lexical": safeLexicalMemorySearch(memoryDir(), req.Query, lim), "elapsed_ms": time.Since(start).Milliseconds()})
}

func handleEmbeddingPage(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		w.WriteHeader(405)
		return
	}
	b, err := uiFS.ReadFile("ui/semantic-memory.html")
	if err != nil {
		http.Error(w, "Page indisponible", 500)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Write(b)
}

// A read-only HTTP connection to the existing chat engine, with model-id
// discovery. No main process PID, service, model file or config is touched.
func connectParallelChat(ctx context.Context, base, key string) error {
	if !parallelBuild() {
		return errors.New("réservé à la version parallèle")
	}
	u, err := url.Parse(strings.TrimRight(strings.TrimSpace(base), "/"))
	if err != nil || u.User != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return errors.New("URL HTTP locale requise")
	}
	switch strings.ToLower(u.Hostname()) {
	case "localhost", "127.0.0.1", "::1":
	default:
		return errors.New("utiliser une adresse de boucle locale")
	}
	// Se brancher sur soi-même est une boucle, pas une connexion. Les ports
	// comparés sont ceux de CETTE instance (plus des nombres figés) : une copie
	// décalée sur 18094 doit être protégée exactement comme l'installation type.
	enc := readEmbeddingDefaults().Port
	switch u.Port() {
	case parallelUIPort(), parallelEnginePort(), parallelBridgePort(), strconv.Itoa(enc):
		return errors.New("utiliser l'URL d'un autre Ajean ou d'un autre moteur, pas celle de celui-ci")
	}
	u.RawQuery, u.Fragment = "", ""
	if !strings.HasSuffix(u.Path, "/v1") {
		u.Path = strings.TrimRight(u.Path, "/") + "/v1"
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, "GET", u.String()+"/models", nil)
	if key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}
	client := &http.Client{Timeout: 5 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("Swift inaccessible : %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("Swift HTTP %d (clé API éventuelle à renseigner)", resp.StatusCode)
	}
	var models struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err = json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&models); err != nil || len(models.Data) == 0 || models.Data[0].ID == "" {
		return errors.New("aucun modèle dans /v1/models")
	}
	c := ReadConfig()
	c[extKeyFlag], c[extKeyURL], c[extKeyModel], c[extKeyToken] = "1", u.String(), models.Data[0].ID, key
	delete(c, extKeyService)
	if err := WriteConfig(c); err != nil {
		return err
	}
	if err := saveParallelSharedPreset(ReadConfig()); err != nil {
		return err
	}
	return parallelServiceAction("stop")
}

func handleEmbeddingConnect(w http.ResponseWriter, r *http.Request) {
	var req struct {
		URL string `json:"url"`
		Key string `json:"key"`
	}
	if !decodeEmbeddingRequest(w, r, &req) {
		return
	}
	if err := connectParallelChat(r.Context(), req.URL, req.Key); err != nil {
		sendJSON(w, 400, map[string]any{"error": err.Error()})
		return
	}
	sendJSON(w, 200, map[string]any{"ok": true, "model": ReadConfig()[extKeyModel]})
}

func handleEmbeddingDemo(w http.ResponseWriter, r *http.Request) {
	var req struct{}
	if !decodeEmbeddingRequest(w, r, &req) {
		return
	}
	if !parallelBuild() {
		sendJSON(w, 403, map[string]any{"error": "réservé à l’essai parallèle"})
		return
	}
	if memEncActive() {
		sendJSON(w, 400, map[string]any{"error": "Utiliser des notes de test avant d’activer le chiffrement."})
		return
	}
	root := memoryDir()
	samples := map[string]string{
		"embedding-demo-physique.md":  "# DÉMONSTRATION fictive — Chaos\n## Maintenir une construction debout\nL’ancrage des clusters d’une GeometryCollection conserve la base immobile. Les seuils de rupture et les contraintes déterminent la résistance aux impacts. Vérifier le résultat en simulation PIE.\nCeci est une donnée synthétique, pas une décision du projet Evolve.\n",
		"embedding-demo-materiaux.md": "# DÉMONSTRATION fictive — Apparence\nUn Material Instance permet de modifier la teinte, la rugosité et les textures. L’éclairage change l’aspect final dans la scène.\nCeci est une donnée synthétique, pas une décision du projet Evolve.\n",
		"embedding-demo-stockage.md":  "# DÉMONSTRATION fictive — Persistance\nSauvegarder explicitement les assets modifiés et la carte concernée. Recharger les fichiers pour vérifier leur persistance.\nCeci est une donnée synthétique, pas une décision du projet Evolve.\n",
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		sendJSON(w, 500, map[string]any{"error": err.Error()})
		return
	}
	for name, content := range samples {
		f, err := os.OpenFile(filepath.Join(root, name), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if errors.Is(err, os.ErrExist) {
			continue
		}
		if err == nil {
			_, err = f.WriteString(content)
			closeErr := f.Close()
			if err == nil {
				err = closeErr
			}
		}
		if err != nil {
			sendJSON(w, 500, map[string]any{"error": err.Error()})
			return
		}
	}
	sendJSON(w, 200, map[string]any{"ok": true, "memory_path": root})
}
