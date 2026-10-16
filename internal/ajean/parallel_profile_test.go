package ajean

import (
	"bytes"
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
)

func parallelTestHome(t *testing.T) string {
	t.Helper()
	home := testHome(t)
	old := BuildFlavor
	BuildFlavor = "embedding-test"
	t.Cleanup(func() { _ = parallelServiceAction("stop"); BuildFlavor = old })
	for _, p := range []string{home, presetsDir(), modelsDir(), backendsDir()} {
		if err := os.MkdirAll(p, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	return home
}

func TestParallelPresetCRUDAndEmbeddingSettings(t *testing.T) {
	parallelTestHome(t)
	embed := embeddingConfig{Enabled: false, Port: 18081, Threads: 8, Model: "separate.gguf"}
	putJSON(bkState, "memory_embeddings", embed)
	embed = readEmbeddingConfig()
	content := "# NAME=Original\nMODEL=Swift.gguf\nPORT=8080\nHOST=0.0.0.0\nEXTRA_ARGS=--device ROCm0 --port=8080 --host 0.0.0.0 --mmproj \"models/some folder/vision.gguf\" --spec-type draft-mtp\n"
	body, _ := json.Marshal(map[string]string{"name": "Swift de test", "content": content})
	w := httptest.NewRecorder()
	handlePresetSave(w, httptest.NewRequest("POST", "http://127.0.0.1/api/preset/save", bytes.NewReader(body)))
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	var saved struct {
		ID string `json:"id"`
	}
	json.Unmarshal(w.Body.Bytes(), &saved)
	raw, err := ReadPreset(saved.ID)
	if err != nil {
		t.Fatal(err)
	}
	cfg := parseEnv(raw)
	if cfg["PORT"] != "18080" || cfg["HOST"] != "127.0.0.1" {
		t.Fatal(cfg)
	}
	args := splitArgs(cfg["EXTRA_ARGS"])
	if strings.Contains(cfg["EXTRA_ARGS"], "8080") || !reflect.DeepEqual(args, []string{"--device", "ROCm0", "--mmproj", `models/some folder/vision.gguf`, "--spec-type", "draft-mtp"}) {
		t.Fatal(args)
	}
	if err = applyPresetFile(filepath.Join(presetsDir(), saved.ID+".env")); err != nil {
		t.Fatal(err)
	}
	if got := readEmbeddingConfig(); got != embed {
		t.Fatalf("embeddings écrasés par preset : %+v", got)
	}
	SetConfigKey("PORT", "8080")
	SetConfigKey("HOST", "0.0.0.0")
	if LLMPort() != 18080 || ReadConfig()["HOST"] != "127.0.0.1" {
		t.Fatal(ReadConfig())
	}
	// Editing and deleting remain the official handlers, confined to this DB.
	body, _ = json.Marshal(map[string]string{"id": saved.ID, "name": "Swift renommé", "content": content + "CTX=64000\n"})
	w = httptest.NewRecorder()
	handlePresetSave(w, httptest.NewRequest("POST", "http://127.0.0.1/api/preset/save", bytes.NewReader(body)))
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	putStr(bkState, "active_preset", "")
	WriteConfig(map[string]string{"EXTERNAL": "1", "EXTERNAL_MODEL": "other"})
	body, _ = json.Marshal(map[string]string{"id": saved.ID})
	w = httptest.NewRecorder()
	handlePresetDelete(w, httptest.NewRequest("POST", "http://127.0.0.1/api/preset/delete", bytes.NewReader(body)))
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	if _, err = ReadPreset(saved.ID); !os.IsNotExist(err) {
		t.Fatalf("preset non supprimé : %v", err)
	}
}

func TestParallelImportPresetsAndSharedFilesReadOnly(t *testing.T) {
	parallelTestHome(t)
	sourceHome := t.TempDir()
	source := filepath.Join(sourceHome, "presets")
	os.MkdirAll(source, 0o700)
	os.MkdirAll(filepath.Join(sourceHome, "models"), 0o700)
	model := filepath.Join(sourceHome, "models", "Swift.gguf")
	os.WriteFile(model, []byte("weights"), 0o600)
	os.WriteFile(filepath.Join(sourceHome, "template.jinja"), []byte("template"), 0o600)
	os.WriteFile(filepath.Join(sourceHome, "models", "vision.gguf"), []byte("projector"), 0o600)
	bin := filepath.Join(sourceHome, "backend", "llama-server")
	os.MkdirAll(filepath.Dir(bin), 0o700)
	os.WriteFile(bin, []byte("binary"), 0o700)
	original := "# NAME=Swift 1.5\nBIN=" + quoteValue(bin) + "\nMODEL=Swift.gguf\nPORT=8080\nEXTRA_ARGS=--port 8080 --jinja --chat-template-file template.jinja --mmproj=vision.gguf\n"
	path := filepath.Join(source, "Swift 1.5.env")
	os.WriteFile(path, []byte(original), 0o600)
	if err := importParallelPresets([]string{source}); err != nil {
		t.Fatal(err)
	}
	raw, err := ReadPreset("Swift 1.5")
	if err != nil {
		t.Fatal(err)
	}
	cfg := parseEnv(raw)
	wantExtra := "--jinja --chat-template-file " + filepath.Join("files", "template.jinja") + " --mmproj=" + filepath.Join("models", "vision.gguf")
	if cfg["MODEL"] != "Swift.gguf" || filepath.IsAbs(cfg["BIN"]) || cfg["PORT"] != "18080" || cfg["EXTRA_ARGS"] != wantExtra {
		t.Fatal(cfg)
	}
	if b, _ := os.ReadFile(path); string(b) != original {
		t.Fatal("source modifiée")
	}
	for source, target := range map[string]string{model: filepath.Join(modelsDir(), "Swift.gguf"), bin: filepath.Join(AjeanHome(), cfg["BIN"])} {
		equal, err := portableFilesEqual(source, target)
		if err != nil || !equal {
			t.Fatalf("copie portable différente : %s %v", target, err)
		}
	}
	copyPath := filepath.Join(presetsDir(), "Swift 1.5.env")
	os.WriteFile(copyPath, []byte("# copie modifiée à conserver"), 0o600)
	if err = importParallelPresets([]string{source}); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(copyPath); string(b) != "# copie modifiée à conserver" {
		t.Fatal("copie écrasée")
	}
	saveExtraModelDirs([]string{filepath.Dir(model)})
	if _, err = resolveServeModelPath(model); err != nil {
		t.Fatal("lecture GGUF partagé refusée", err)
	}
	if err = deleteModelFile(model); err == nil {
		t.Fatal("suppression du GGUF partagé autorisée")
	}
	if _, err = resolveDownloadDir(filepath.Dir(model)); err == nil {
		t.Fatal("écriture dans le dossier partagé autorisée")
	}
	if b, _ := os.ReadFile(model); string(b) != "weights" {
		t.Fatal("modèle source altéré")
	}
	if err = WriteConfig(map[string]string{"BIN": bin, "MODEL": model}); err == nil {
		t.Fatal("preset avec dépendance à la source accepté")
	}
	replaceKV(bkConfig, map[string]string{"BIN": bin, "MODEL": model}) // stale v1 DB: update still stays local
	if got := llamacppRepoDir(); got != defaultRepoDir() {
		t.Fatalf("mise à jour hors du TEST : %s", got)
	}
	// A symlink inside backends must not expose a production checkout to update.
	link := filepath.Join(backendsDir(), "shared")
	if err = os.Symlink(filepath.Dir(bin), link); err == nil {
		if _, err = backendDir("shared"); err == nil {
			t.Fatal("lien backend partagé accepté en écriture")
		}
	}
}

func TestParallelRemoteControlsNeverAdoptForeignPIDOrPort(t *testing.T) {
	home := parallelTestHome(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"data": []map[string]string{{"id": "production-swift"}}})
	}))
	defer server.Close()
	if err := WriteConfig(map[string]string{"EXTERNAL": "1", "EXTERNAL_URL": server.URL + "/v1"}); err != nil {
		t.Fatal(err)
	}
	pidPath := filepath.Join(home, "ajean-embedding-test.pid")
	os.WriteFile(pidPath, []byte("foreign-pid-marker"), 0o600)
	for _, action := range []string{"start", "restart", "stop"} {
		if err := serviceAction(action); err != nil {
			t.Fatal(err)
		}
	}
	if b, _ := os.ReadFile(pidPath); string(b) != "foreign-pid-marker" {
		t.Fatal("fichier PID étranger modifié")
	}
	if err := connectParallelChat(context.Background(), server.URL, ""); err != nil {
		t.Fatal(err)
	}
	if ReadConfig()[extKeyModel] != "production-swift" || activePresetID() == "" {
		t.Fatal("connexion non conservée comme preset")
	}
	r, err := server.Client().Get(server.URL)
	if err != nil {
		t.Fatal("API étrangère arrêtée", err)
	}
	r.Body.Close()
	ln, err := net.Listen("tcp", "127.0.0.1:18080")
	if err != nil {
		t.Skip("port de test déjà occupé")
	}
	defer ln.Close()
	bin := filepath.Join(home, "fake-bin")
	model := filepath.Join(modelsDir(), "fake.gguf")
	os.WriteFile(bin, []byte("binary"), 0o700)
	os.WriteFile(model, []byte("weights"), 0o600)
	WriteConfig(map[string]string{"BIN": bin, "MODEL": model})
	if err = serviceAction("start"); err == nil || !strings.Contains(err.Error(), "occupé") {
		t.Fatalf("port occupé non refusé : %v", err)
	}
	if parallelEngineActive() {
		t.Fatal("processus étranger adopté")
	}
}

func TestParallelConfigurationRebasesAfterMovingFolder(t *testing.T) {
	parallelTestHome(t)
	root := t.TempDir()
	before := filepath.Join(root, "before")
	after := filepath.Join(root, "after")
	home := filepath.Join(before, "data")
	t.Setenv("AJEAN_HOME", home)
	bin := filepath.Join(home, "backends", "runtime", "llama-server.exe")
	model := filepath.Join(home, "models", "Swift.gguf")
	encoder := filepath.Join(before, "encoder", "llama-server.exe")
	embedModel := filepath.Join(home, "models", "embeddinggemma-2-Q8_0.gguf")
	for _, p := range []string{bin, model, encoder, embedModel} {
		os.MkdirAll(filepath.Dir(p), 0o700)
		os.WriteFile(p, []byte("fixture"), 0o700)
	}
	if err := WriteConfig(map[string]string{"BIN": bin, "MODEL": model}); err != nil {
		t.Fatal(err)
	}
	if err := saveEmbeddingConfig(embeddingConfig{Enabled: true, Bin: encoder, Model: embedModel, Port: 18081, Threads: 8}); err != nil {
		t.Fatal(err)
	}
	if stored := cachedKV(bkConfig); filepath.IsAbs(stored["BIN"]) || filepath.IsAbs(stored["MODEL"]) {
		t.Fatal("chemins absolus persistés", stored)
	}
	var stored embeddingConfig
	getJSON(bkState, "memory_embeddings", &stored)
	if filepath.IsAbs(stored.Bin) || filepath.IsAbs(stored.Model) {
		t.Fatal("encodeur non portable", stored)
	}
	if err := os.Rename(before, after); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AJEAN_HOME", filepath.Join(after, "data"))
	cfg := ReadConfig()
	if cfg["BIN"] != filepath.Join(after, "data", "backends", "runtime", "llama-server.exe") {
		t.Fatal(cfg)
	}
	if _, err := resolveServeModelPath(cfg["MODEL"]); err != nil {
		t.Fatal("GGUF dépendant de l’ancien emplacement", err)
	}
	c := readEmbeddingConfig()
	if c.Bin != filepath.Join(after, "encoder", "llama-server.exe") || c.Model != filepath.Join(after, "data", "models", "embeddinggemma-2-Q8_0.gguf") {
		t.Fatal(c)
	}
	if err := validateEmbeddingConfig(c); err != nil {
		t.Fatal("encodeur non relançable après déplacement", err)
	}
}

func TestParallelDoesNotInstallSystemBuildTools(t *testing.T) {
	parallelTestHome(t)
	if err := requireTools("ajean-proof-tool-that-is-not-installed"); err == nil || !strings.Contains(err.Error(), "portable") {
		t.Fatalf("installation système non bloquée : %v", err)
	}
}

func TestParallelNativePresetSaveCopiesExternalModel(t *testing.T) {
	parallelTestHome(t)
	source := t.TempDir()
	model := filepath.Join(source, "Swift.gguf")
	projector := filepath.Join(source, "vision.gguf")
	template := filepath.Join(source, "template.jinja")
	for path, content := range map[string]string{model: "original weights", projector: "original vision", template: "original template"} {
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	bin := filepath.Join(backendsDir(), "chosen-rocm", "llama-server.exe")
	os.MkdirAll(filepath.Dir(bin), 0o700)
	os.WriteFile(bin, []byte("copied runtime"), 0o700)
	content := formatEnv(map[string]string{"BIN": bin, "MODEL": model, "MMPROJ": projector, "EXTRA_ARGS": "--device ROCm0 --chat-template-file " + quoteValue(template)})
	// The screenshot's case: engine is already inside TEST, MODEL still points
	// at the original installation. Reproduce the actual native save request.
	body, _ := json.Marshal(map[string]string{"name": "Swift", "content": content})
	w := httptest.NewRecorder()
	handlePresetSave(w, httptest.NewRequest("POST", "/api/preset/save", bytes.NewReader(body)))
	if w.Code != http.StatusOK {
		t.Fatal(w.Body.String())
	}
	var response struct{ ID string }
	json.Unmarshal(w.Body.Bytes(), &response)
	raw, err := ReadPreset(response.ID)
	if err != nil {
		t.Fatal(err)
	}
	cfg := parseEnv(raw)
	if cfg["MODEL"] != "Swift.gguf" || cfg["MMPROJ"] != "vision.gguf" || filepath.IsAbs(cfg["BIN"]) || strings.Contains(raw, source) {
		t.Fatalf("preset dépendant de la source : %s", raw)
	}
	for from, to := range map[string]string{model: filepath.Join(modelsDir(), "Swift.gguf"), projector: filepath.Join(modelsDir(), "vision.gguf"), template: filepath.Join(AjeanHome(), "files", "template.jinja")} {
		if equal, err := portableFilesEqual(from, to); err != nil || !equal {
			t.Fatalf("copie incorrecte : %s : %v", to, err)
		}
	}
	if err := os.Rename(source, source+"-hidden"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(source + "-hidden") })
	if _, err := SavePreset(response.ID, "Swift renommé", raw); err != nil {
		t.Fatal("enregistrement encore dépendant de la source", err)
	}
	if err := applyPresetFile(filepath.Join(presetsDir(), response.ID+".env")); err != nil {
		t.Fatal("sélection dépendante de la source", err)
	}
	if !samePath(ReadConfig()["BIN"], bin) {
		t.Fatal("moteur choisi non conservé", ReadConfig())
	}
}

func TestParallelPresetSelectionCopiesStalePathsAndPreservesCollisions(t *testing.T) {
	parallelTestHome(t)
	source := t.TempDir()
	bin := filepath.Join(source, "rocm", "llama-server.exe")
	model := filepath.Join(source, "Swift.gguf")
	os.MkdirAll(filepath.Dir(bin), 0o700)
	os.WriteFile(bin, []byte("engine"), 0o700)
	os.WriteFile(filepath.Join(filepath.Dir(bin), "rocm.dll"), []byte("dependency"), 0o600)
	os.WriteFile(model, []byte("weights"), 0o600)
	// Old presets can be selected directly, without saving them first.
	stale := filepath.Join(presetsDir(), "old.env")
	content := formatEnv(map[string]string{"BIN": bin, "MODEL": model})
	os.WriteFile(stale, []byte(content), 0o600)
	if err := applyPresetFile(stale); err != nil {
		t.Fatal(err)
	}
	cfg := ReadConfig()
	if strings.Contains(cfg["BIN"], source) || cfg["MODEL"] != "Swift.gguf" {
		t.Fatal(cfg)
	}
	if b, err := os.ReadFile(filepath.Join(filepath.Dir(cfg["BIN"]), "rocm.dll")); err != nil || string(b) != "dependency" {
		t.Fatal("DLL non copiée", err)
	}
	if b, _ := os.ReadFile(stale); strings.Contains(string(b), source) {
		t.Fatal("ancien chemin source conservé dans le preset", string(b))
	} else {
		content = string(b)
	}
	if err := os.Rename(source, source+"-hidden"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(source + "-hidden") })
	if err := applyPresetFile(stale); err != nil {
		t.Fatal("resélection dépendante de la source", err)
	}
	// A different same-named model never replaces the portable copy or preset.
	other := filepath.Join(t.TempDir(), "Swift.gguf")
	os.WriteFile(other, []byte("different weights"), 0o600)
	if _, err := SavePreset("old", "changed", "MODEL="+quoteValue(other)+"\n"); err == nil {
		t.Fatal("collision différente acceptée")
	}
	if b, _ := os.ReadFile(stale); string(b) != content {
		t.Fatal("preset changé malgré échec de copie")
	}
	if b, _ := os.ReadFile(filepath.Join(modelsDir(), "Swift.gguf")); string(b) != "weights" {
		t.Fatal("modèle portable écrasé")
	}
	if b, _ := os.ReadFile(other); string(b) != "different weights" {
		t.Fatal("source altérée")
	}
}

func TestParallelPresetCopiesNamedModelFromExternalDirectory(t *testing.T) {
	parallelTestHome(t)
	source := t.TempDir()
	os.WriteFile(filepath.Join(source, "Swift.gguf"), []byte("weights"), 0o600)
	saveExtraModelDirs([]string{source})
	if _, err := SavePreset("", "Swift", "MODEL=Swift.gguf\n"); err != nil {
		t.Fatal(err)
	}
	if b, err := os.ReadFile(filepath.Join(modelsDir(), "Swift.gguf")); err != nil || string(b) != "weights" {
		t.Fatal("modèle externe référencé par nom non copié", err)
	}
}
