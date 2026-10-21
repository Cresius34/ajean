package ajean

import (
	"crypto/sha256"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

func portableFilesEqual(a, b string) (bool, error) {
	fa, err := os.Open(a)
	if err != nil {
		return false, err
	}
	defer fa.Close()
	fb, err := os.Open(b)
	if err != nil {
		return false, err
	}
	defer fb.Close()
	sa, err := fa.Stat()
	if err != nil {
		return false, err
	}
	sb, err := fb.Stat()
	if err != nil {
		return false, err
	}
	if sa.Size() != sb.Size() {
		return false, nil
	}
	ha, hb := sha256.New(), sha256.New()
	if _, err = io.Copy(ha, fa); err != nil {
		return false, err
	}
	if _, err = io.Copy(hb, fb); err != nil {
		return false, err
	}
	return string(ha.Sum(nil)) == string(hb.Sum(nil)), nil
}

// Copy to a temporary sibling, then reserve the final name exclusively. An
// existing file is reused only after a full SHA-256 comparison, never replaced.
func copyPortableFile(source, target string) error {
	if err := parallelWritablePath(target); err != nil {
		return err
	}
	if fi, err := os.Lstat(target); err == nil {
		if !fi.Mode().IsRegular() {
			return fmt.Errorf("destination non régulière : %s", target)
		}
		equal, err := portableFilesEqual(source, target)
		if err != nil {
			return err
		}
		if !equal {
			return fmt.Errorf("fichier différent déjà présent, conservé : %s", target)
		}
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	fi, err := in.Stat()
	if err != nil {
		return err
	}
	if !fi.Mode().IsRegular() {
		return fmt.Errorf("source non régulière : %s", source)
	}
	if err = os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		return err
	}
	out, err := os.CreateTemp(filepath.Dir(target), filepath.Base(target)+".part-")
	if err != nil {
		return err
	}
	temp := out.Name()
	defer os.Remove(temp)
	if fi.Size() >= 128<<20 {
		fmt.Printf("Copie %s (%.2f Gio)…\n", filepath.Base(source), float64(fi.Size())/(1<<30))
	}
	n, err := io.Copy(out, in)
	if err == nil {
		err = out.Chmod(fi.Mode().Perm())
	}
	if err == nil {
		err = out.Sync()
	}
	closeErr := out.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	after, err := in.Stat()
	if err != nil {
		return err
	}
	if n != fi.Size() || after.Size() != fi.Size() || !after.ModTime().Equal(fi.ModTime()) {
		return fmt.Errorf("source modifiée pendant la copie : %s", source)
	}
	reservation, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, fi.Mode().Perm())
	if os.IsExist(err) {
		equal, e := portableFilesEqual(source, target)
		if e != nil {
			return e
		}
		if equal {
			return nil
		}
		return fmt.Errorf("collision de copie : %s", target)
	}
	if err != nil {
		return err
	}
	reservation.Close()
	if err = os.Rename(temp, target); err != nil {
		os.Remove(target)
		return err
	}
	return nil
}

func resolveImportFile(sourceHome, key, value string) (string, error) {
	if filepath.IsAbs(value) {
		return filepath.Clean(value), nil
	}
	candidates := []string{filepath.Join(sourceHome, value)}
	if key != "BIN" {
		candidates = append([]string{filepath.Join(sourceHome, "models", value)}, candidates...)
	}
	for _, p := range candidates {
		if fi, err := os.Stat(p); err == nil && fi.Mode().IsRegular() {
			return p, nil
		}
	}
	return "", fmt.Errorf("%s introuvable : %s", key, value)
}

func importPortableModel(source string) (string, error) {
	names := shardFamily(filepath.Base(source))
	if len(names) == 0 {
		names = []string{filepath.Base(source)}
	}
	for _, name := range names {
		if err := copyPortableFile(filepath.Join(filepath.Dir(source), name), filepath.Join(modelsDir(), name)); err != nil {
			return "", err
		}
	}
	return filepath.Base(source), nil
}

// Copy the runtime directory only: DLLs/shared libraries and adjacent runtime
// files. A source checkout's absolute CMake/Ninja build cache is never imported.
func importPortableBackend(sourceHome, bin string) (string, error) {
	runtimeDir := filepath.Dir(bin)
	name := "imported-" + filepath.Base(runtimeDir)
	if rel, err := filepath.Rel(filepath.Join(sourceHome, "backends"), runtimeDir); err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		name = strings.Split(rel, string(filepath.Separator))[0]
		if name == "llama.cpp" || name == "llama.cpp-prebuilt" {
			name += "-runtime-" + filepath.Base(runtimeDir)
		}
	}
	name = sanitizeBackendName(name)
	targetRoot, err := backendDir(name)
	if err != nil {
		return "", err
	}
	if pathWithin(runtimeDir, targetRoot) {
		return "", fmt.Errorf("le dossier portable ne doit pas être placé dans le moteur source")
	}
	realRuntime, err := filepath.EvalSymlinks(runtimeDir)
	if err != nil {
		return "", err
	}
	err = filepath.WalkDir(runtimeDir, func(path string, e fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if e.IsDir() {
			if e.Name() == ".git" || e.Name() == "CMakeFiles" {
				return filepath.SkipDir
			}
			return nil
		}
		switch e.Name() {
		case "CMakeCache.txt", "build.ninja", "Makefile", "cmake_install.cmake", "compile_commands.json":
			return nil
		}
		real := path
		if e.Type()&os.ModeSymlink != 0 {
			var err error
			real, err = filepath.EvalSymlinks(path)
			if err != nil {
				return err
			}
			if !pathWithin(realRuntime, real) {
				return fmt.Errorf("dépendance externe au runtime : %s", path)
			}
		}
		fi, err := os.Stat(real)
		if err != nil {
			return err
		}
		if !fi.Mode().IsRegular() {
			return fmt.Errorf("runtime non régulier : %s", path)
		}
		rel, err := filepath.Rel(runtimeDir, path)
		if err != nil {
			return err
		}
		return copyPortableFile(real, filepath.Join(targetRoot, rel))
	})
	if err != nil {
		return "", err
	}
	return filepath.Rel(AjeanHome(), filepath.Join(targetRoot, filepath.Base(bin)))
}

func importPortableExtraArgs(sourceHome, extra string) (string, error) {
	args := parallelNetworkArgs(splitArgs(extra))
	fileOptions := map[string]bool{"--mmproj": true, "--model-draft": true, "-md": true, "--chat-template-file": true, "--grammar-file": true, "--lora": true, "--lora-scaled": true}
	for i, arg := range args {
		prefix, value := "", arg
		option := ""
		if i > 0 {
			option = args[i-1]
		}
		if strings.HasPrefix(arg, "-") {
			if p, v, ok := strings.Cut(arg, "="); ok {
				prefix, value = p+"=", v
				option = p
			}
		}
		if !filepath.IsAbs(value) && !fileOptions[option] {
			continue
		}
		source, err := resolveImportFile(sourceHome, "EXTRA_ARGS", value)
		if err != nil {
			return "", err
		}
		if strings.EqualFold(filepath.Ext(source), ".gguf") {
			name, err := importPortableModel(source)
			if err != nil {
				return "", err
			}
			args[i] = prefix + filepath.Join("models", name)
		} else {
			rel := filepath.Join("files", filepath.Base(source))
			if err := copyPortableFile(source, filepath.Join(AjeanHome(), rel)); err != nil {
				return "", err
			}
			args[i] = prefix + rel
		}
	}
	for i := range args {
		args[i] = quoteValue(args[i])
	}
	return strings.Join(args, " "), nil
}

// A native preset edit may still carry an absolute path from revision 1, or
// select a model from an additional model directory. Stage those dependencies
// when the user saves/selects the preset, rather than rejecting the edit. Keep
// ordinary config writes strict: startup and status reads must not copy files.
func prepareParallelPreset(cfg map[string]string) (map[string]string, error) {
	if err := validateEmbeddingPreset(cfg); err != nil {
		return nil, err
	}
	var err error
	cfg, err = prepareParallelMemoryFiles(cfg, AjeanHome())
	if err != nil {
		return nil, err
	}
	if isExternalConfig(cfg) || isCloudConfig(cfg) || isMoeConfig(cfg) {
		return normalizeParallelConfig(cfg)
	}
	next := make(map[string]string, len(cfg))
	for k, v := range cfg {
		next[k] = v
	}
	for _, key := range []string{"BIN", "MODEL", "MMPROJ", "MODEL_DRAFT"} {
		p := next[key]
		if p == "" {
			continue
		}
		if !filepath.IsAbs(p) {
			// Bare GGUF names may resolve to a user-declared external directory.
			// An incomplete preset is still editable if the file is not present.
			if key == "BIN" || strings.ContainsAny(p, `/\`) {
				continue
			}
			resolved, err := resolveServeModelPath(p)
			if err != nil {
				continue
			}
			if fi, err := os.Stat(resolved); err != nil || !fi.Mode().IsRegular() {
				continue
			}
			p = resolved
		}
		if err := parallelWritablePath(p); err == nil {
			continue
		} else if pathWithin(AjeanHome(), p) {
			return nil, fmt.Errorf("%s : %w", key, err)
		}
		var err error
		if key == "BIN" {
			next[key], err = importPortableBackend(AjeanHome(), p)
		} else {
			next[key], err = importPortableModel(p)
		}
		if err != nil {
			return nil, fmt.Errorf("copie portable %s : %w", key, err)
		}
	}
	// Import the same file-valued arguments handled by Copier-Presets, but only
	// if this edit actually references an external absolute file.
	for _, arg := range splitArgs(next["EXTRA_ARGS"]) {
		value := arg
		if strings.HasPrefix(arg, "-") {
			if _, v, ok := strings.Cut(arg, "="); ok {
				value = v
			}
		}
		if filepath.IsAbs(value) && !pathWithin(AjeanHome(), value) {
			var err error
			next["EXTRA_ARGS"], err = importPortableExtraArgs(AjeanHome(), next["EXTRA_ARGS"])
			if err != nil {
				return nil, fmt.Errorf("copie portable EXTRA_ARGS : %w", err)
			}
			break
		}
	}
	return normalizeParallelConfig(next)
}

func importParallelPresets(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage : ajean-memoire-test import-presets <dossier source>")
	}
	source, err := filepath.Abs(args[0])
	if err != nil {
		return err
	}
	if samePath(source, presetsDir()) {
		return fmt.Errorf("la source doit être différente des presets du TEST")
	}
	entries, err := os.ReadDir(source)
	if err != nil {
		return err
	}
	sourceHome := filepath.Dir(source)
	copied, existing := 0, 0
	for _, e := range entries {
		if !e.Type().IsRegular() || !strings.HasSuffix(strings.ToLower(e.Name()), ".env") {
			continue
		}
		target := filepath.Join(presetsDir(), e.Name())
		if _, err = os.Lstat(target); err == nil {
			existing++
			continue
		}
		if !os.IsNotExist(err) {
			return err
		}
		raw, err := os.ReadFile(filepath.Join(source, e.Name()))
		if err != nil {
			return err
		}
		cfg := parseEnv(string(raw))
		if isCloudConfig(cfg) || isMoeConfig(cfg) {
			fmt.Printf("ignoré %s : moteur système Cloud/MoE (hors phase 1)\n", e.Name())
			continue
		}
		fmt.Printf("Import portable : %s\n", e.Name())
		if !isExternalConfig(cfg) {
			if v := cfg["BIN"]; v != "" {
				p, err := resolveImportFile(sourceHome, "BIN", v)
				if err != nil {
					return err
				}
				cfg["BIN"], err = importPortableBackend(sourceHome, p)
				if err != nil {
					return err
				}
			}
			for _, key := range []string{"MODEL", "MMPROJ", "MODEL_DRAFT"} {
				if v := cfg[key]; v != "" {
					p, err := resolveImportFile(sourceHome, key, v)
					if err != nil {
						return err
					}
					cfg[key], err = importPortableModel(p)
					if err != nil {
						return err
					}
				}
			}
			if v := cfg["EXTRA_ARGS"]; v != "" {
				cfg["EXTRA_ARGS"], err = importPortableExtraArgs(sourceHome, v)
				if err != nil {
					return err
				}
			}
		} else {
			for _, key := range []string{"BIN", "MODEL", "MMPROJ", "MODEL_DRAFT", "EXTRA_ARGS"} {
				delete(cfg, key)
			}
		}
		cfg, err = prepareParallelMemoryFiles(cfg, sourceHome)
		if err != nil {
			return err
		}
		cfg, err = normalizeParallelConfig(cfg)
		if err != nil {
			return err
		}
		if err = parallelWritablePath(target); err != nil {
			return err
		}
		f, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if os.IsExist(err) {
			existing++
			continue
		}
		if err != nil {
			return err
		}
		_, err = f.WriteString(withDisplayName(formatEnv(cfg), presetDisplayName(string(raw), strings.TrimSuffix(e.Name(), ".env"))))
		closeErr := f.Close()
		if err != nil {
			return err
		}
		if closeErr != nil {
			return closeErr
		}
		copied++
	}
	fmt.Printf("Presets et fichiers copiés : %d ; presets déjà présents conservés : %d. Source lue uniquement.\n", copied, existing)
	return nil
}
