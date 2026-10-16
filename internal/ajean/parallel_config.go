package ajean

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const parallelChatPort = "18080"

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
	if isCloudConfig(cfg) || isMoeConfig(cfg) {
		return nil, fmt.Errorf("cette copie gère llama.cpp local et les API externes ; Cloud/MoE système restent désactivés")
	}
	next := make(map[string]string, len(cfg)+2)
	for k, v := range cfg {
		next[k] = v
	}
	next["HOST"], next["PORT"] = "127.0.0.1", parallelChatPort
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
	for _, key := range []string{"BIN", "MODEL", "MMPROJ", "MODEL_DRAFT"} {
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
