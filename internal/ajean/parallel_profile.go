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

// parallelMemoryRevision identifie la génération d'interface servie (champ API
// memory_ui_revision). Il n'apparaît plus comme repère visuel depuis la
// v0.18.4.1 Cresius : le bandeau affiche un intitulé, plus un numéro de revue.
const parallelMemoryRevision = 7

// buildFlavorSuffix est l'identité publique de ce build : la base amont
// (Version, reprise telle quelle du tag nathaninline fusionné) suivie du
// millésime Cresius. Version = "0.18.15" + ".5 Cresius" donne donc
// « 0.18.15.5 Cresius », c'est-à-dire la base 0.18.15 et notre révision 5
// (tag git v0.18.15+rev5) : la révision 1 était le patch correctif, la 2
// ajoute le canal Ajean↔Ajean (pont + outil peer), la 3 rend ce canal honnête —
// le modèle prêté sait enfin qu'il est prêté et qu'il n'a aucun outil, et un
// refus du build portable explique POURQUOI il refuse ; la 4 fait rendre au pont
// un véritable tour d'agent de l'autre côté, sous double autorisation ; la 5
// distingue l'AUTEUR de chaque message du canal — un ordre d'un humain qui vise
// un poste lie l'IA de ce poste, un ordre d'une IA n'en lie aucune, et le tour
// lancé pour un appelant qui raccroche est annulé au lieu de tourner pour
// personne. Doit rester identique à
// cmd/ajean-memory-test/versioninfo.json (StringFileInfo.ProductVersion et
// FileVersion), vérifié par TestVersionFlavorWindowsSuit.
const buildFlavorSuffix = ".5 Cresius"

// uiBadgeLabel : intitulé affiché sous le logo par le build portable, à la
// place du repère « TEST mémoire · R6 ». C'est le nom du patch, pas un numéro.
const uiBadgeLabel = "Patch correctif de Cresius"

func displayVersion() string {
	if parallelBuild() {
		return Version + buildFlavorSuffix
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
	case "app", "web", "help", "-h", "--help", "version", "-v", "--version", "where", "serve", "import-presets", "bridge", "peer":
		return true
	// La clé API machine se définit déjà depuis l'interface (/api/apikey n'est
	// pas bloqué par parallelHTTPHandler). Sans elle, le pont Ajean↔Ajean ne
	// peut tout simplement pas exister : antKeyOK n'admet personne. La CLI
	// portable doit donc pouvoir la poser, comme l'UI le fait déjà.
	case "set-api-key":
		return true
	}
	return false
}

// parallelBlockedRoutes : ce que le build portable refuse, et POURQUOI. Une
// raison par préfixe : un message recopié pour six routes différentes rend un
// refus légitime incompréhensible. Mesuré le 2026-10-27 : qui cliquait sur
// « joignable depuis le réseau local » lisait « utiliser la page Mémoire
// sémantique », deux phrases sans aucun rapport avec le réseau.
var parallelBlockedRoutes = []struct {
	prefix string
	msg    string
}{
	{"/api/update/", "Mise à jour désactivée : cette version portable de Cresius se met à jour à la main (le binaire est livré avec son patch)."},
	{"/api/network", "Exposition du moteur sur le réseau désactivée dans cette version portable : le seul port ouvert vers l'extérieur est celui du pont Ajean↔Ajean (page Réglages, carte « Pont Ajean ↔ Ajean »), verrouillé par la clé API de la machine. Le port du moteur reste sur cette machine."},
	{"/api/cloud/", "GPU cloud désactivé dans cette version portable."},
	{"/api/link/", "Accès distant ajean.link désactivé dans cette version portable : rien ne sort de cette machine hors du réseau local."},
	{"/api/oai/", "Exposition publique (ajean.link) désactivée dans cette version portable."},
	{"/api/backup/", "Sauvegarde à distance désactivée dans cette version portable."},
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
		//
		// Chaque refus porte SA raison. Un message unique recopié pour six préfixes
		// différents faisait répondre, à qui cliquait « joignable depuis le réseau
		// local », « utiliser la page Mémoire sémantique » — aucune des deux choses
		// n'a de rapport avec le réseau (mesuré le 2026-10-27, capture d'écran de
		// l'utilisateur). Le refus est légitime, l'explication doit l'être aussi.
		for _, b := range parallelBlockedRoutes {
			if strings.HasPrefix(p, b.prefix) {
				sendJSON(w, 403, map[string]any{"ok": false, "error": b.msg})
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func cmdParallelApp() error {
	// Le port de l'interface se lit, il ne s'impose plus : deux installations
	// côte à côte (la vivante et une copie de réparation, ou les deux moitiés
	// d'un pont Ajean↔Ajean sur la même machine) doivent pouvoir démarrer sans
	// se voler 18090. AJEAN_UI_PORT ou la clé parallel_ui_port décalent.
	port := parallelUIPort()
	addr := "127.0.0.1:" + port
	url := "http://" + addr + "/?memory-test=" + displayVersion()
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("port d’interface %s occupé ; aucun processus arrêté : %w (décaler avec AJEAN_UI_PORT=18094)", port, err)
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
	// Le moteur doit annoncer le port qu'il écoute vraiment : la config porte
	// PORT, et tout ce qui parle au moteur en local passe par LLMPort().
	if err := ensureParallelEnginePort(); err != nil {
		fmt.Printf("[moteur] port non aligné : %v\n", err)
	}
	// Le pont Ajean↔Ajean s'allume ici, dans le process qui tient déjà la base :
	// la clé API est relue à chaque requête, donc une clé définie après le
	// démarrage est valable immédiatement. Éteint par défaut : rien ne s'expose
	// tout seul. La veille rattache l'écoute au réglage, pour qu'un
	// « ajean bridge on » lancé à côté de cette instance prenne sans redémarrer.
	bridgeStop := make(chan struct{})
	defer close(bridgeStop)
	if parallelBridgeEnabled() {
		if err := startParallelBridge(); err != nil {
			fmt.Printf("[pont ajean] non démarré : %v\n", err)
		}
	}
	go parallelBridgeWatch(bridgeStop)
	fmt.Printf("AJEAN — %s\n%s\nDonnées : %s\nMoteur local : 127.0.0.1:%s ; API existante disponible par preset.\n", displayVersion(), url, AjeanHome(), parallelLocalEnginePort())
	_ = openBrowser(url) // normal browser tab: a separate profile is unnecessary
	runTray(url)
	return nil
}
