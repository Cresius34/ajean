package ajean

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

func prepareParallelMemoryFiles(cfg map[string]string, sourceHome string) (map[string]string, error) {
	next := make(map[string]string, len(cfg))
	for k, v := range cfg {
		next[k] = v
	}
	for _, key := range []string{"MEMORY_EMBED_BIN", "MEMORY_EMBED_MODEL"} {
		p := cfg[key]
		if p == "" {
			continue
		}
		if !filepath.IsAbs(p) {
			p = filepath.Join(sourceHome, p)
			if _, err := os.Stat(p); os.IsNotExist(err) {
				continue
			}
		}
		if err := parallelPackagePath(p); err == nil {
			next[key] = p
			continue
		} else if pathWithin(filepath.Dir(AjeanHome()), p) {
			return nil, err
		}
		var err error
		if key == "MEMORY_EMBED_BIN" {
			next[key], err = importPortableBackend(sourceHome, p)
		} else {
			var fi os.FileInfo
			fi, err = os.Stat(p)
			if err == nil && fi.IsDir() {
				target := filepath.Join(modelsDir(), filepath.Base(p))
				if pathWithin(p, target) {
					return nil, fmt.Errorf("le modèle source ne doit pas contenir le dossier portable")
				}
				if _, err := os.Stat(filepath.Join(p, "config.json")); err != nil {
					return nil, fmt.Errorf("dossier de modèle complet invalide : config.json absent")
				}
				err = filepath.WalkDir(p, func(from string, e fs.DirEntry, walkErr error) error {
					if walkErr != nil {
						return walkErr
					}
					if e.IsDir() {
						if e.Name() == ".cache" {
							return filepath.SkipDir
						}
						return nil
					}
					if !e.Type().IsRegular() {
						return fmt.Errorf("modèle complet : lien ou fichier non régulier")
					}
					rel, err := filepath.Rel(p, from)
					if err != nil {
						return err
					}
					return copyPortableFile(from, filepath.Join(target, rel))
				})
				next[key], _ = filepath.Rel(AjeanHome(), target)
			} else if err == nil {
				var name string
				name, err = importPortableModel(p)
				next[key] = filepath.Join("models", name)
			}
		}
		if err != nil {
			return nil, fmt.Errorf("copie portable %s : %w", key, err)
		}
	}
	return next, nil
}
