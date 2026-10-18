package ajean

// Set by the dedicated portable entry point. Keeping this build
// identity in the executable also protects a direct double-click (no launcher
// or inherited AJEAN_HOME is required to keep production isolated).
import (
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"
)

var BuildFlavor string

const parallelMemoryRevision = 6

func displayVersion() string {
	if parallelBuild() {
		return fmt.Sprintf("%s-embedding-test.%d", Version, parallelMemoryRevision)
	}
	return Version
}

func parallelBuild() bool                   { return BuildFlavor == "embedding-test" }
func parallelHome(executable string) string { return filepath.Join(filepath.Dir(executable), "data") }

func initParallelProfile() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	// Intentionally ignore an inherited AJEAN_HOME pointing at production.
	if err = os.Setenv("AJEAN_HOME", parallelHome(exe)); err != nil {
		return err
	}
	if err = os.Setenv("AJEAN_SERVICE", "ajean-embedding-test"); err != nil {
		return err
	}
	for _, d := range dataDirs() {
		if err = os.MkdirAll(d, 0o755); err != nil {
			return err
		}
	}
	if len(ReadConfig()) == 0 {
		cfg := defaultConfig()
		cfg["MEM_MODE"] = "ondemand"
		if err = WriteConfig(cfg); err != nil {
			return err
		}
	}
	// Upgrade revision 1 in place: same DB/notes, fixed test-only listener and
	// an editable preset for the already working shared API connection.
	if err = WriteConfig(ReadConfig()); err != nil {
		return err
	}
	if presets, _ := ListPresets(); len(presets) == 0 && isExternalConfig(ReadConfig()) {
		if err = saveParallelSharedPreset(ReadConfig()); err != nil {
			return err
		}
	}
	var saved embeddingConfig
	if !getJSON(bkState, "memory_embeddings", &saved) {
		model := filepath.Join(AjeanHome(), "models", "embeddinggemma-2-Q8_0.gguf")
		_, e := os.Stat(model)
		binName := "llama-server"
		if strings.EqualFold(filepath.Ext(exe), ".exe") {
			binName += ".exe"
		}
		if err = saveEmbeddingConfig(embeddingConfig{
			Enabled: e == nil, Bin: filepath.Join(filepath.Dir(exe), "encoder", binName), Model: model, Port: 18081, Threads: 8,
		}); err != nil {
			return err
		}
	}
	// Migrate v1's default absolute paths and persist relative paths for moves.
	if err = saveEmbeddingConfig(readEmbeddingDefaults()); err != nil {
		return err
	}
	return nil
}

func parallelCommandAllowed(cmd string) bool {
	switch cmd {
	case "app", "web", "help", "-h", "--help", "version", "-v", "--version", "where", "serve", "import-presets":
		return true
	}
	return false
}

func parallelHTTPHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := r.URL.Path
		if p == "/api/update" {
			sendJSON(w, 200, map[string]any{"current": Version, "available": false, "parallel_test": true})
			return
		}
		if strings.HasPrefix(p, "/api/moe/") && runtime.GOOS != "windows" {
			sendJSON(w, 403, map[string]any{"ok": false, "error": "MoE portable : ce chemin d’installation est réservé à Windows x64."})
			return
		}
		// Presets and llama.cpp management use their normal handlers. Services,
		// registry/network settings and updating this fork stay out of scope.
		blocked := []string{"/api/update/", "/api/network", "/api/cloud/", "/api/link/", "/api/oai/", "/api/backup/"}
		for _, prefix := range blocked {
			if strings.HasPrefix(p, prefix) {
				sendJSON(w, 403, map[string]any{"ok": false, "error": "Commande désactivée dans la version parallèle ; utiliser la page Mémoire sémantique."})
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func cmdParallelApp() error {
	const addr = "127.0.0.1:18090"
	url := "http://127.0.0.1:18090/?memory-test=" + displayVersion()
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("port d’essai 18090 occupé ; aucun processus arrêté : %w", err)
	}
	defer ln.Close()
	defer stopEmbeddingServer()
	defer parallelServiceAction("stop")
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(signals)
	go func() { <-signals; stopEmbeddingServer(); _ = parallelServiceAction("stop"); os.Exit(0) }()
	mux := newWebMux()
	srv := &http.Server{Handler: parallelHTTPHandler(gzipHandler(mux)), ReadHeaderTimeout: 10 * time.Second}
	go func() { _ = srv.Serve(ln) }()
	if c := readEmbeddingConfig(); c.Enabled {
		root, _ := filepath.Abs(memoryDir())
		queueEmbeddingIndex(root, c)
	}
	fmt.Printf("AJEAN mémoire — %s\n%s\nDonnées : %s\nMoteur local du TEST : 127.0.0.1:18080 ; API existante disponible par preset.\n", displayVersion(), url, AjeanHome())
	_ = openBrowser(url) // normal browser tab: a separate profile is unnecessary
	runTray(url)
	return nil
}
