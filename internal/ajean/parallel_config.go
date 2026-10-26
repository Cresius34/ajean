package ajean

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// parallelEnginePortDef : le port historique du moteur sur ce build. Il n'est
// plus une constante gravée dans le marbre : deux installations côte à côte sur
// la même machine doivent pouvoir avancer chacune de leur côté (l'étude du pont
// Ajean↔Ajean butait là — tout était en dur, donc une seconde instance était
// impossible). Le réglage se lit dans la base, la clé reste « PORT » comme
// partout ailleurs.
const parallelEnginePortDef = "18080"

// parallelEnginePort : port du moteur MoE de CETTE instance. Relu à chaque appel
// (le cache du store est invalidé par l'écriture d'un autre process), donc
// modifiable sans recompiler.
func parallelEnginePort() string {
	if p := strings.TrimSpace(getStr(bkState, "parallel_engine_port")); p != "" {
		if n, err := strconv.Atoi(p); err == nil && n >= 1025 && n <= 65534 {
			return strconv.Itoa(n)
		}
	}
	if p := strings.TrimSpace(os.Getenv("AJEAN_ENGINE_PORT")); p != "" {
		if n, err := strconv.Atoi(p); err == nil && n >= 1025 && n <= 65534 {
			return strconv.Itoa(n)
		}
	}
	return parallelEnginePortDef
}

// parallelUIPort : port de l'interface. 18090 reste le défaut — c'est ce que
// connaissent nos lanceurs — mais il devient réglable, par la base ou par
// l'environnement (AJEAN_UI_PORT), pour qu'une seconde copie démarre sans
// voler le port de la première.
func parallelUIPort() string {
	if p := strings.TrimSpace(os.Getenv("AJEAN_UI_PORT")); p != "" {
		if n, err := strconv.Atoi(p); err == nil && n >= 1025 && n <= 65534 {
			return strconv.Itoa(n)
		}
	}
	if p := strings.TrimSpace(getStr(bkState, "parallel_ui_port")); p != "" {
		if n, err := strconv.Atoi(p); err == nil && n >= 1025 && n <= 65534 {
			return strconv.Itoa(n)
		}
	}
	return parallelUIPortDef
}

const parallelUIPortDef = "18090"

// parallelLocalEnginePort : le port où le moteur de CETTE instance sert
// réellement. La config porte un PORT (écrit à l'import d'un preset, ou par
// ensureParallelEnginePort) ; c'est lui que lit LLMPort(), donc le proxy du pont
// et le --port du lancement. parallelEnginePort() seul serait un réglage
// d'instance qui peut ne pas coller à la config : le pont annoncerait alors un
// port et en frapperait un autre.
func parallelLocalEnginePort() string {
	if p := strings.TrimSpace(ReadConfig()["PORT"]); p != "" {
		if n, err := strconv.Atoi(p); err == nil && n > 0 {
			return strconv.Itoa(n)
		}
	}
	return parallelEnginePort()
}

// ensureParallelEnginePort aligne la config sur le réglage d'instance. Sans ça,
// un dossier sans preset importé a PORT vide → LLMPort() retombe sur 8080, un
// port que nos garde-fous interdisent et que personne n'écoute. Un seul écrit,
// idempotent.
func ensureParallelEnginePort() error {
	want := parallelEnginePort()
	if strings.TrimSpace(ReadConfig()["PORT"]) == want {
		return nil
	}
	return SetConfigKey("PORT", want)
}

// parallelEnginePortFor n'existe pas : un preset n'a PAS son mot à dire sur le
// port du moteur. normalizeParallelConfig l'écrase par celui de l'instance, ce
// qui maintient la garantie d'origine (un preset importé de la production ne
// renvoie jamais le moteur d'essai vers 8080, ni vers un port voisin).

// Presets imported from production must never redirect the test engine to
// port 8080, including a late override hidden in EXTRA_ARGS.
func parallelNetworkArgs(args []string) []string {
	var out []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--host" || a == "--port" || a == "-host" || a == "-port" {
			if i+1 < len(args) {
				i++
			}
			continue
		}
		if strings.HasPrefix(a, "--host=") || strings.HasPrefix(a, "--port=") || strings.HasPrefix(a, "-host=") || strings.HasPrefix(a, "-port=") {
			continue
		}
		out = append(out, a)
	}
	return out
}

func normalizeParallelConfig(cfg map[string]string) (map[string]string, error) {
	if isCloudConfig(cfg) || (isMoeConfig(cfg) && !isStrataMoe(cfg)) {
		return nil, fmt.Errorf("Cloud et les anciens moteurs MoE système restent désactivés ; utiliser le moteur MoE Windows portable")
	}
	next := make(map[string]string, len(cfg)+2)
	for k, v := range cfg {
		next[k] = v
	}
	next["HOST"], next["PORT"] = "127.0.0.1", parallelEnginePort()
	if isStrataMoe(next) {
		for _, key := range []string{"MOE_CONFIG", "MOE_SOURCE", "MOE_PYTHON"} {
			p := next[key]
			if p == "" {
				return nil, fmt.Errorf("%s absent du preset MoE portable", key)
			}
			if !filepath.IsAbs(p) {
				p = filepath.Join(AjeanHome(), p)
			}
			if err := parallelWritablePath(p); err != nil {
				return nil, err
			}
			rel, err := filepath.Rel(AjeanHome(), p)
			if err != nil {
				return nil, err
			}
			next[key] = rel
		}
		if next["MOE_BACKEND"] != "hip" && next["MOE_BACKEND"] != "cuda" {
			return nil, fmt.Errorf("backend MoE Windows invalide")
		}
	}
	if err := validateEmbeddingPreset(next); err != nil {
		return nil, err
	}
	for _, key := range []string{"MEMORY_EMBED_BIN", "MEMORY_EMBED_MODEL"} {
		if p := next[key]; p != "" {
			if !filepath.IsAbs(p) {
				p = filepath.Join(AjeanHome(), p)
			}
			if err := parallelPackagePath(p); err != nil {
				return nil, err
			}
			rel, err := filepath.Rel(AjeanHome(), p)
			if err != nil {
				return nil, err
			}
			next[key] = rel
		}
	}
	delete(next, extKeyService)
	{
		for _, key := range []string{"BIN", "MODEL", "MMPROJ", "MODEL_DRAFT"} {
			if p := next[key]; filepath.IsAbs(p) {
				if err := parallelWritablePath(p); err != nil {
					if isExternalConfig(next) {
						delete(next, key)
						continue
					}
					return nil, fmt.Errorf("%s : utiliser Copier-Presets pour créer une copie portable (%w)", key, err)
				}
				rel, err := filepath.Rel(AjeanHome(), p)
				if err != nil {
					return nil, err
				}
				next[key] = rel
			}
		}
	}
	if extra := next["EXTRA_ARGS"]; extra != "" {
		args := parallelNetworkArgs(splitArgs(extra))
		for i := range args {
			prefix, value := "", args[i]
			if strings.HasPrefix(value, "-") {
				if p, v, ok := strings.Cut(value, "="); ok {
					prefix, value = p+"=", v
				}
			}
			if filepath.IsAbs(value) {
				if err := parallelWritablePath(value); err != nil {
					return nil, fmt.Errorf("EXTRA_ARGS : copier le fichier dans le dossier portable : %w", err)
				}
				rel, err := filepath.Rel(AjeanHome(), value)
				if err != nil {
					return nil, err
				}
				args[i] = prefix + rel
			}
			args[i] = quoteValue(args[i])
		}
		next["EXTRA_ARGS"] = strings.Join(args, " ")
	}
	return next, nil
}

// Disk records use relative paths; runtime/UI callers get paths rebased on the
// current data root. No dependency on the directory from which Windows starts.
func resolveParallelConfig(cfg map[string]string) map[string]string {
	next := make(map[string]string, len(cfg))
	for k, v := range cfg {
		next[k] = v
	}
	for _, key := range []string{"BIN", "MODEL", "MMPROJ", "MODEL_DRAFT", "MOE_CONFIG", "MOE_SOURCE", "MOE_PYTHON"} {
		p := next[key]
		if p != "" && !filepath.IsAbs(p) && (key == "BIN" || strings.ContainsAny(p, `/\`)) {
			next[key] = filepath.Join(AjeanHome(), p)
		}
	}
	return next
}

// Existing shared binaries and weights may be read/executed, but management
// operations can only change files physically inside this copy's data folder.
func parallelWritablePath(path string) error {
	return parallelPathUnder(AjeanHome(), path)
}

func parallelPackagePath(path string) error {
	return parallelPathUnder(filepath.Dir(AjeanHome()), path)
}

func parallelPathUnder(base, path string) error {
	root, err := filepath.Abs(base)
	if err != nil {
		return err
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	if !pathWithin(root, abs) {
		return fmt.Errorf("chemin partagé en lecture seule dans le TEST : %s", path)
	}
	realRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return err
	}
	probe := abs
	var missing []string
	for {
		real, e := filepath.EvalSymlinks(probe)
		if e == nil {
			for i := len(missing) - 1; i >= 0; i-- {
				real = filepath.Join(real, missing[i])
			}
			if !pathWithin(realRoot, real) {
				return fmt.Errorf("lien vers un dossier partagé en lecture seule : %s", path)
			}
			return nil
		}
		if !os.IsNotExist(e) {
			return e
		}
		parent := filepath.Dir(probe)
		if parent == probe {
			return e
		}
		missing = append(missing, filepath.Base(probe))
		probe = parent
	}
}

// Give the already working API connection a real, editable preset. Do not
// overwrite a user's local preset when the quick-connect form is used later.
func saveParallelSharedPreset(cfg map[string]string) error {
	id := getStr(bkState, "parallel_shared_preset")
	if id != "" {
		body, err := ReadPreset(id)
		if err != nil || !isExternalConfig(parseEnv(body)) {
			id = ""
		}
	}
	id, err := SavePreset(id, "Swift · API existante", formatEnv(cfg))
	if err != nil {
		return err
	}
	if err = putStr(bkState, "parallel_shared_preset", id); err != nil {
		return err
	}
	return putStr(bkState, "active_preset", id)
}
