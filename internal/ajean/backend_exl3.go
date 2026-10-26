package ajean

// backend_exl3.go : moteur ExLlamaV3 (quants EXL3), servi par TabbyAPI, installé
// et piloté par ajean comme llama.cpp.
//
// Déroulé :
//  1. installation (tâche « exl3 », même suivi que llama.cpp) : uv (version figée,
//     vérifiée par SHA-256) installe son propre Python, puis TabbyAPI à un commit
//     donné et ses dépendances précompilées (torch + roue exllamav3, épinglées par
//     TabbyAPI lui-même dans son pyproject) : aucun Python ni compilateur requis
//     sur la machine. L'installation par défaut prend le commit validé ici ; la
//     mise à jour prend le dernier commit de TabbyAPI ;
//  2. un preset ENGINE=exl3 désigne un dossier de modèle EXL3 et ses réglages ;
//  3. `ajean serve` (unité ajean-engine) écrit la config de TabbyAPI et le lance à
//     la place de llama-server, sur le même port et avec la même clé : le reste
//     d'ajean (chat, /health, /v1/*) le voit comme un moteur local ordinaire.
//
// Périmètre : Linux et Windows x86_64, cartes NVIDIA.

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	exl3TabbyRepo = "theroyallab/tabbyAPI"
	// exl3TabbyPinned : commit de TabbyAPI validé sur le serveur de test
	// (2026-10-21, exllamav3 1.6.4, Qwen3.8 27B EXL3 : 55 tok/s, vision, 131k).
	exl3TabbyPinned = "9979d0188c3e687a6ba9ed3ddb7db79048116116"
	exl3UVVersion   = "0.13.4"
	exl3PyVersion   = "3.12"
	// exl3MinArch : cartes plus anciennes que les RTX 20 écartées (comme le moteur MoE).
	exl3MinArch = 75
)

// exl3PinnedLock : versions des dépendances (hors torch/CUDA/exllamav3, que
// TabbyAPI épingle lui-même) validées avec exl3TabbyPinned. Appliquées comme
// contraintes à l'installation par défaut pour qu'elle soit reproductible ; la
// mise à jour (dernier commit) prend les versions du moment.
const exl3PinnedLock = "aiofiles==25.1.0 aiohappyeyeballs==2.7.1 aiohttp==3.14.5 aiosignal==1.4.0 annotated-doc==0.0.5 annotated-types==0.8.0 anyio==4.15.1 async-lru==2.4.0 attrs==26.1.0 certifi==2026.10.19 charset-normalizer==3.5.2 click==8.5.0 einops==0.8.2 fastapi==0.158.1 fastapi-slim==0.129.1 filelock==4.2.4 fla-core==0.5.2 flash-linear-attention==0.5.2 frozenlist==1.8.0 fsspec==2026.9.0 h11==0.16.0 hf-xet==1.7.0 httpcore2==2.16.0 httptools==0.9.0 httpx2==2.16.0 huggingface-hub==2.2.0 idna==3.20 jinja2==3.1.6 llguidance==1.9.1 loguru==0.7.3 marisa-trie==1.4.1 markdown-it-py==4.2.0 markupsafe==3.0.4 mdurl==0.1.2 mpmath==1.3.0 multidict==7.1.0 networkx==3.7 ninja==1.13.2 numpy==2.5.4 opentelemetry-api==1.45.1 packaging==26.3 pillow==13.0.0 propcache==0.5.4 psutil==7.2.2 pydantic==2.14.1 pydantic-core==2.50.1 pygments==2.21.0 pyyaml==6.0.3 regex==2026.9.29 requests==2.34.2 rich==15.0.0 ruamel-yaml==0.19.1 safetensors==0.8.0 setuptools==81.0.0 shellingham==1.5.4 sse-starlette==3.5.0 starlette==1.9.0 sympy==1.14.0 tokenizers==0.23.3 tqdm==4.70.1 transformers==5.20.2 truststore==0.10.5 typer==0.27.3 typing-extensions==4.16.0 typing-inspection==0.4.4 urllib3==2.8.0 uvicorn==0.54.0 uvloop==0.23.0 yarl==1.25.1"

type exl3Asset struct{ Name, SHA string }

// exl3UVAssets : l'archive de uv par système, et son empreinte publiée.
var exl3UVAssets = map[string]exl3Asset{
	"linux":   {"uv-x86_64-unknown-linux-gnu.tar.gz", "2b428396f12572d4429cf210ca234dd599a33b3af0958a4d5ed569d63e3f344f"},
	"windows": {"uv-x86_64-pc-windows-msvc.zip", "ccec7508a07d9933d7cb1261880633efcf1f30d6e6eddab21823f1dae6f23225"},
}

func exl3Home() string       { return filepath.Join(AjeanHome(), "exl3") }
func exl3DLDir() string      { return filepath.Join(exl3Home(), "dl") }
func exl3StateFile() string  { return filepath.Join(exl3Home(), "engine.json") }
func exl3ModelsDir() string  { return filepath.Join(modelsDir(), "exl3") }
func exl3ConfigFile() string { return filepath.Join(exl3Home(), "config-ajean.yml") }

func exl3UVBin() string {
	if runtime.GOOS == "windows" {
		return filepath.Join(exl3Home(), "bin", "uv.exe")
	}
	return filepath.Join(exl3Home(), "bin", "uv")
}

// exl3Python : le Python de l'environnement de TabbyAPI.
func exl3Python(dir string) string {
	if runtime.GOOS == "windows" {
		return filepath.Join(dir, ".venv", "Scripts", "python.exe")
	}
	return filepath.Join(dir, ".venv", "bin", "python")
}

// isExl3Config : la configuration fait tourner le moteur ExLlama.
func isExl3Config(cfg map[string]string) bool { return cfg["ENGINE"] == "exl3" }

// exl3UVEnv : uv range Python et son cache sous le dossier du moteur, et
// n'utilise que son propre Python (jamais celui du système, dont la version ou
// les modules manquants feraient échouer l'installation).
func exl3UVEnv() string {
	return strings.Join([]string{
		"UV_PYTHON_INSTALL_DIR=" + filepath.Join(exl3Home(), "python"),
		"UV_CACHE_DIR=" + filepath.Join(exl3Home(), "cache"),
		"UV_PYTHON_PREFERENCE=only-managed",
		"UV_NO_PROGRESS=1",
		"PYTHONUNBUFFERED=1",
	}, "\x00")
}

// ---------------------------------------------------------------------------
// Machine
// ---------------------------------------------------------------------------

type exl3Env struct {
	Supported  bool     `json:"supported"`
	Reason     string   `json:"reason,omitempty"`
	GPUs       []moeGPU `json:"gpus"`
	Driver     string   `json:"driver,omitempty"`
	CUDA       string   `json:"cuda,omitempty"` // cu12 | cu13 : jeu de roues choisi
	DiskFreeGB float64  `json:"disk_free_gb"`
}

var (
	exl3EnvMu sync.Mutex
	exl3EnvC  *exl3Env
	exl3EnvAt time.Time
)

// exl3Detect : la machine peut-elle faire tourner le moteur, et avec quelles roues.
// Mis en cache 30 s (l'écran l'interroge à chaque ouverture).
func exl3Detect() exl3Env {
	exl3EnvMu.Lock()
	defer exl3EnvMu.Unlock()
	if exl3EnvC != nil && time.Since(exl3EnvAt) < 30*time.Second {
		return *exl3EnvC
	}
	env := exl3DetectNow()
	exl3EnvC, exl3EnvAt = &env, time.Now()
	return env
}

func exl3DetectNow() exl3Env {
	env := exl3Env{}
	if f := diskFree(AjeanHome()); f > 0 {
		env.DiskFreeGB = float64(f) / 1e9
	}
	if (runtime.GOOS != "linux" && runtime.GOOS != "windows") || runtime.GOARCH != "amd64" {
		env.Reason = "ExLlama n'est proposé que sous Linux et Windows (x86_64)"
		return env
	}
	gpus, err := detectGPUs()
	if err != nil {
		env.Reason = "ExLlama demande une carte NVIDIA (" + err.Error() + ")"
		return env
	}
	best := 0
	for _, g := range gpus {
		mib, _ := strconv.ParseFloat(g.MemTotal, 64)
		cap, _ := strconv.ParseFloat(g.Cap, 64)
		arch := int(cap*10 + 0.5)
		env.GPUs = append(env.GPUs, moeGPU{Index: g.Index, Name: g.Name, VRAMGB: mib / 1024, Arch: arch})
		best = max(best, arch)
	}
	if best < exl3MinArch {
		env.Reason = "carte trop ancienne pour ExLlama (RTX 20 ou plus récente)"
		return env
	}
	env.Driver = nvidiaDriverVersion()
	env.CUDA = exl3CudaFlavor(env.Driver)
	if env.CUDA == "" {
		env.Reason = fmt.Sprintf("pilote NVIDIA %s trop ancien : version 570 ou plus récente requise", firstNonEmpty(env.Driver, "inconnu"))
		return env
	}
	env.Supported = true
	return env
}

// nvidiaDriverVersion : « 595.71.05 », ou "" si illisible.
func nvidiaDriverVersion() string {
	out, err := hideCmd(exec.Command("nvidia-smi", "--query-gpu=driver_version", "--format=csv,noheader")).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(strings.SplitN(strings.TrimSpace(string(out)), "\n", 2)[0])
}

// exl3CudaFlavor : le jeu de roues que le pilote peut faire tourner. Les roues
// CUDA 13 (torch 2.11, celles validées ici) demandent un pilote 580 ou plus,
// les roues CUDA 12.8 un pilote 570 ou plus.
func exl3CudaFlavor(driver string) string {
	major, _ := strconv.Atoi(strings.SplitN(driver, ".", 2)[0])
	switch {
	case major >= 580:
		return "cu13"
	case major >= 570:
		return "cu12"
	}
	return ""
}

// ---------------------------------------------------------------------------
// Moteur installé
// ---------------------------------------------------------------------------

// exl3Installed : ce qui est en place (engine.json).
type exl3Installed struct {
	Dir         string `json:"dir"`
	Commit      string `json:"commit"`
	Date        string `json:"date,omitempty"` // date du commit TabbyAPI
	CUDA        string `json:"cuda"`
	Exllama     string `json:"exllamav3,omitempty"`
	Torch       string `json:"torch,omitempty"`
	InstalledAt int64  `json:"installed_at"`
}

func exl3ReadInstalled() *exl3Installed {
	b, err := os.ReadFile(exl3StateFile())
	if err != nil {
		return nil
	}
	var st exl3Installed
	if json.Unmarshal(b, &st) != nil || st.Dir == "" {
		return nil
	}
	if !isFile(exl3Python(st.Dir)) || !isFile(filepath.Join(st.Dir, "main.py")) {
		return nil
	}
	return &st
}

func shortSHA(s string) string {
	if len(s) > 7 {
		return s[:7]
	}
	return s
}

// exl3Fetch télécharge url vers le dossier dl et vérifie l'empreinte si fournie.
func exl3Fetch(rawURL, name, wantSHA string) (string, error) {
	dst := filepath.Join(exl3DLDir(), name)
	if wantSHA != "" {
		if got, err := sha256File(dst); err == nil && got == wantSHA {
			lcAppend(name + " déjà téléchargé")
			return dst, nil
		}
	}
	if err := os.MkdirAll(exl3DLDir(), 0o755); err != nil {
		return "", err
	}
	tmp := dst + ".part"
	if err := downloadWithProgress(rawURL, tmp, 0, lcAppend); err != nil {
		_ = os.Remove(tmp)
		return "", fmt.Errorf("téléchargement de %s : %w", name, err)
	}
	if wantSHA != "" {
		got, err := sha256File(tmp)
		if err != nil {
			return "", err
		}
		if got != wantSHA {
			_ = os.Remove(tmp)
			return "", fmt.Errorf("%s : empreinte SHA-256 inattendue (fichier altéré ou incomplet)", name)
		}
	}
	return dst, os.Rename(tmp, dst)
}

// exl3EnsureUV pose uv (version figée) dans exl3Home()/bin.
func exl3EnsureUV() error {
	if out, err := hideCmd(exec.Command(exl3UVBin(), "--version")).Output(); err == nil && strings.Contains(string(out), exl3UVVersion) {
		return nil
	}
	a, ok := exl3UVAssets[runtime.GOOS]
	if !ok {
		return fmt.Errorf("uv indisponible pour %s", runtime.GOOS)
	}
	lcPhase("téléchargement de uv " + exl3UVVersion + "…")
	arch, err := exl3Fetch("https://github.com/astral-sh/uv/releases/download/"+exl3UVVersion+"/"+a.Name, a.Name, a.SHA)
	if err != nil {
		return err
	}
	tmp := filepath.Join(exl3Home(), "uv-tmp")
	_ = os.RemoveAll(tmp)
	defer os.RemoveAll(tmp)
	if err := extractArchive(arch, tmp); err != nil {
		return fmt.Errorf("décompression de uv : %w", err)
	}
	name := filepath.Base(exl3UVBin())
	var found string
	_ = filepath.Walk(tmp, func(p string, fi os.FileInfo, err error) error {
		if err == nil && !fi.IsDir() && fi.Name() == name && found == "" {
			found = p
		}
		return nil
	})
	if found == "" {
		return fmt.Errorf("uv introuvable dans %s", a.Name)
	}
	if err := os.MkdirAll(filepath.Dir(exl3UVBin()), 0o755); err != nil {
		return err
	}
	_ = os.Remove(exl3UVBin())
	if err := os.Rename(found, exl3UVBin()); err != nil {
		if err := copyFile(found, exl3UVBin()); err != nil {
			return err
		}
	}
	_ = os.Chmod(exl3UVBin(), 0o755)
	return nil
}

// Patch de TabbyAPI : la variable TABBY_VISION_DEVICE (cuda:N) charge l'encodeur
// d'images sur une carte choisie. Sans elle TabbyAPI le pose sur la première
// carte, qui déborde au premier long prompt quand elle est déjà pleine (mesuré
// sur une 3070 avec 131k de contexte). Sans effet si le code amont a changé :
// le réglage « carte de la vision » est alors simplement ignoré.
const exl3VisionOrig = `            for value in self.vision_model.load_gen(
                reserve_per_device=None if self.gpu_split else self.autosplit_reserve,
                use_per_device=self.gpu_split or None,
                callback=progress_callback,
            ):`

const exl3VisionPatched = `            # Patch AJEAN : TABBY_VISION_DEVICE=cuda:N charge la vision sur une carte choisie.
            import os as _ajean_os
            _vdev = _ajean_os.environ.get("TABBY_VISION_DEVICE")
            _vkw = {"device": _vdev} if _vdev else {
                "reserve_per_device": None if self.gpu_split else self.autosplit_reserve,
                "use_per_device": self.gpu_split or None,
            }
            for value in self.vision_model.load_gen(callback=progress_callback, **_vkw):`

func exl3PatchVision(dir string) bool {
	p := filepath.Join(dir, "backends", "exllamav3", "model.py")
	b, err := os.ReadFile(p)
	if err != nil {
		return false
	}
	s := strings.ReplaceAll(string(b), "\r\n", "\n")
	if strings.Contains(s, "TABBY_VISION_DEVICE") {
		return true
	}
	if !strings.Contains(s, exl3VisionOrig) {
		return false
	}
	return os.WriteFile(p, []byte(strings.Replace(s, exl3VisionOrig, exl3VisionPatched, 1)), 0o644) == nil
}

// exl3RunInstall : le corps de la tâche d'installation / mise à jour. commit
// vide = dernier commit de TabbyAPI.
func exl3RunInstall(commit string) {
	env := exl3DetectNow()
	if !env.Supported {
		lcFail(fmt.Errorf("%s", env.Reason))
		return
	}
	if env.DiskFreeGB > 0 && env.DiskFreeGB < 12 {
		lcFail(fmt.Errorf("espace disque insuffisant : %.0f Go libres, environ 12 Go nécessaires", env.DiskFreeGB))
		return
	}
	date := ""
	if commit == "" {
		lcPhase("recherche de la dernière version de TabbyAPI…")
		c, d, err := exl3LatestCommit(true)
		if err != nil {
			lcFail(fmt.Errorf("dernière version de TabbyAPI introuvable : %w", err))
			return
		}
		commit, date = c, d
	} else if c, d, err := exl3LatestCommit(false); err == nil && c == commit {
		date = d
	}
	prev := exl3ReadInstalled()
	if err := os.MkdirAll(exl3Home(), 0o755); err != nil {
		lcFail(err)
		return
	}
	if err := exl3EnsureUV(); err != nil {
		lcFail(err)
		return
	}

	// 1. les sources de TabbyAPI à ce commit
	lcPhase("téléchargement de TabbyAPI " + shortSHA(commit) + "…")
	ext := ".tar.gz"
	kind := "tar.gz"
	if runtime.GOOS == "windows" {
		ext, kind = ".zip", "zip"
	}
	arch, err := exl3Fetch("https://codeload.github.com/"+exl3TabbyRepo+"/"+kind+"/"+commit, "tabbyAPI-"+commit+ext, "")
	if err != nil {
		lcFail(err)
		return
	}
	dir := filepath.Join(exl3Home(), "tabby-"+shortSHA(commit))
	if prev != nil && samePath(prev.Dir, dir) {
		// réinstallation du même commit : dossier neuf à côté, l'actuel reste en place
		dir += "-" + strconv.FormatInt(time.Now().Unix(), 36)
	}
	tmp := filepath.Join(exl3Home(), "src-tmp")
	_ = os.RemoveAll(tmp)
	if err := extractArchive(arch, tmp); err != nil {
		lcFail(fmt.Errorf("décompression de TabbyAPI : %w", err))
		return
	}
	entries, _ := os.ReadDir(tmp)
	if len(entries) != 1 || !entries[0].IsDir() {
		lcFail(fmt.Errorf("archive TabbyAPI inattendue"))
		return
	}
	_ = os.RemoveAll(dir)
	if err := os.Rename(filepath.Join(tmp, entries[0].Name()), dir); err != nil {
		lcFail(err)
		return
	}
	_ = os.RemoveAll(tmp)
	_ = os.Remove(arch) // gardé seulement le temps de l'installation
	if exl3PatchVision(dir) {
		lcAppend("choix de la carte pour la vision : activé")
	} else {
		lcAppend("choix de la carte pour la vision : indisponible dans cette version de TabbyAPI")
	}

	// 2. Python + dépendances précompilées (torch, exllamav3, flash-linear-attention…)
	fail := func(step string, err error) {
		_ = os.RemoveAll(dir)
		if buildWasCanceled() {
			lcFail(fmt.Errorf("installation annulée"))
		} else {
			lcFail(fmt.Errorf("%s : %w (voir le journal)", step, err))
		}
	}
	lcPhase("préparation de Python " + exl3PyVersion + "…")
	if err := runStepEnv("environnement Python", dir, exl3UVEnv(), exl3UVBin(), "venv", "--python", exl3PyVersion, ".venv"); err != nil {
		fail("création de l'environnement Python", err)
		return
	}
	lcPhase("installation des dépendances (torch, exllamav3 : ~4 Go)…")
	pipArgs := []string{"pip", "install", "--python", exl3Python(dir)}
	if commit == exl3TabbyPinned {
		lock := filepath.Join(dir, "ajean-constraints.txt")
		if err := os.WriteFile(lock, []byte(strings.ReplaceAll(exl3PinnedLock, " ", "\n")+"\n"), 0o644); err == nil {
			pipArgs = append(pipArgs, "--constraint", lock)
		}
	}
	pipArgs = append(pipArgs, ".["+env.CUDA+"]")
	if err := runStepEnv("dépendances", dir, exl3UVEnv(), exl3UVBin(), pipArgs...); err != nil {
		fail("installation des dépendances", err)
		return
	}

	// 3. contrôle : les modules se chargent et voient la carte
	lcPhase("vérification…")
	torchV, exlV, cudaOK, err := exl3Probe(dir)
	if err != nil {
		fail("vérification", err)
		return
	}
	if !cudaOK {
		lcAppend("⚠ torch ne voit aucune carte CUDA (pilote ?)")
	}
	st := exl3Installed{Dir: dir, Commit: commit, Date: date, CUDA: env.CUDA, Exllama: exlV, Torch: torchV, InstalledAt: time.Now().Unix()}
	b, _ := json.MarshalIndent(st, "", "  ")
	if err := os.WriteFile(exl3StateFile(), b, 0o644); err != nil {
		fail("enregistrement", err)
		return
	}
	// moteur actif : relancé sur la nouvelle version
	if isExl3Config(ReadConfig()) {
		lcPhase("redémarrage du moteur…")
		if err := serviceAction("restart"); err != nil {
			lcAppend("⚠ redémarrage du moteur impossible : " + err.Error())
		}
	}
	if prev != nil && !samePath(prev.Dir, dir) {
		if err := os.RemoveAll(prev.Dir); err != nil {
			lcAppend("ancienne version laissée en place : " + prev.Dir)
		}
	}
	lcDone(fmt.Sprintf("ExLlama installé (exllamav3 %s, TabbyAPI %s)", firstNonEmpty(exlV, "?"), shortSHA(commit)))
}

// exl3Probe charge torch et exllamav3 dans l'environnement installé.
func exl3Probe(dir string) (torchV, exlV string, cudaOK bool, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	code := "import importlib.metadata as m, torch, exllamav3; print(torch.__version__); print(m.version('exllamav3')); print(torch.cuda.is_available())"
	cmd := hideCmd(exec.CommandContext(ctx, exl3Python(dir), "-c", code))
	cmd.Dir = dir
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	if e := cmd.Run(); e != nil {
		tail := strings.TrimSpace(errb.String())
		if len(tail) > 600 {
			tail = tail[len(tail)-600:]
		}
		lcAppend(tail)
		return "", "", false, fmt.Errorf("exllamav3 ne se charge pas")
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) < 3 {
		return "", "", false, fmt.Errorf("réponse inattendue : %s", out.String())
	}
	return strings.TrimSpace(lines[0]), strings.TrimSpace(lines[1]), strings.TrimSpace(lines[2]) == "True", nil
}

// exl3Uninstall retire le moteur (pas les modèles, rangés dans models/exl3).
func exl3Uninstall() error {
	if isExl3Config(ReadConfig()) {
		return fmt.Errorf("le modèle actif tourne sur ExLlama : activez d'abord un autre modèle")
	}
	return os.RemoveAll(exl3Home())
}

// ---------------------------------------------------------------------------
// Mises à jour
// ---------------------------------------------------------------------------

var (
	exl3LatestMu   sync.Mutex
	exl3LatestSHA  string
	exl3LatestDate string
	exl3LatestAt   time.Time
)

// exl3LatestCommit : dernier commit de la branche principale de TabbyAPI (mis en
// cache 30 min ; fresh force la lecture).
func exl3LatestCommit(fresh bool) (string, string, error) {
	exl3LatestMu.Lock()
	defer exl3LatestMu.Unlock()
	if !fresh && exl3LatestSHA != "" && time.Since(exl3LatestAt) < 30*time.Minute {
		return exl3LatestSHA, exl3LatestDate, nil
	}
	req, _ := http.NewRequest("GET", "https://api.github.com/repos/"+exl3TabbyRepo+"/commits/main", nil)
	req.Header.Set("Accept", "application/vnd.github+json")
	c := &http.Client{Timeout: 10 * time.Second}
	resp, err := c.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", "", fmt.Errorf("GitHub : HTTP %d", resp.StatusCode)
	}
	var v struct {
		SHA    string `json:"sha"`
		Commit struct {
			Committer struct {
				Date string `json:"date"`
			} `json:"committer"`
		} `json:"commit"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&v); err != nil || v.SHA == "" {
		return "", "", fmt.Errorf("réponse GitHub illisible")
	}
	exl3LatestSHA, exl3LatestDate, exl3LatestAt = v.SHA, v.Commit.Committer.Date, time.Now()
	return v.SHA, v.Commit.Committer.Date, nil
}

// ---------------------------------------------------------------------------
// Modèles
// ---------------------------------------------------------------------------

type exl3Model struct {
	Name   string  `json:"name"`
	Path   string  `json:"path"`
	SizeGB float64 `json:"size_gb"`
	BPW    float64 `json:"bpw,omitempty"`
	Vision bool    `json:"vision,omitempty"`
}

// exl3ReadModel : dir est-il un modèle EXL3 (config.json avec une
// quantization_config exl3) ? Lit aussi le débit et la présence de la vision.
func exl3ReadModel(dir string) (exl3Model, bool) {
	b, err := os.ReadFile(filepath.Join(dir, "config.json"))
	if err != nil {
		return exl3Model{}, false
	}
	var c struct {
		Quant struct {
			Method string  `json:"quant_method"`
			BPW    float64 `json:"bits"`
		} `json:"quantization_config"`
		Vision json.RawMessage `json:"vision_config"`
	}
	if json.Unmarshal(b, &c) != nil || !strings.EqualFold(c.Quant.Method, "exl3") {
		return exl3Model{}, false
	}
	m := exl3Model{Name: filepath.Base(dir), Path: dir, BPW: c.Quant.BPW}
	// la vision n'est utilisable que si ses poids ont été gardés dans le quant
	// (config.json la décrit même quand ils ont été retirés)
	if len(c.Vision) > 0 && string(c.Vision) != "null" {
		m.Vision = exl3HasVisionWeights(dir)
	}
	entries, _ := os.ReadDir(dir)
	var size int64
	for _, e := range entries {
		if fi, err := e.Info(); err == nil && !e.IsDir() {
			size += fi.Size()
		}
	}
	m.SizeGB = float64(size) / 1e9
	return m, true
}

// exl3HasVisionWeights : des tenseurs de l'encodeur d'images figurent dans les
// poids (index des tranches, ou en-tête d'un fichier unique).
func exl3HasVisionWeights(dir string) bool {
	isVis := func(k string) bool { return strings.Contains(k, "visual") || strings.Contains(k, "vision_tower") }
	if b, err := os.ReadFile(filepath.Join(dir, "model.safetensors.index.json")); err == nil {
		var idx struct {
			WeightMap map[string]string `json:"weight_map"`
		}
		if json.Unmarshal(b, &idx) == nil {
			for k := range idx.WeightMap {
				if isVis(k) {
					return true
				}
			}
		}
		return false
	}
	f, err := os.Open(filepath.Join(dir, "model.safetensors"))
	if err != nil {
		return false
	}
	defer f.Close()
	var n [8]byte
	if _, err := io.ReadFull(f, n[:]); err != nil {
		return false
	}
	size := int64(n[0]) | int64(n[1])<<8 | int64(n[2])<<16 | int64(n[3])<<24 | int64(n[4])<<32 | int64(n[5])<<40
	if size <= 0 || size > 64<<20 {
		return false
	}
	hdr := make([]byte, size)
	if _, err := io.ReadFull(f, hdr); err != nil {
		return false
	}
	var m map[string]json.RawMessage
	if json.Unmarshal(hdr, &m) != nil {
		return false
	}
	for k := range m {
		if isVis(k) {
			return true
		}
	}
	return false
}

// exl3ListModels : les modèles EXL3 des dossiers de modèles (racine et sous-dossier
// exl3), plus ceux que les presets désignent ailleurs.
func exl3ListModels() []exl3Model {
	seen := map[string]bool{}
	out := []exl3Model{}
	add := func(dir string) {
		k := normDir(dir)
		if seen[k] {
			return
		}
		if m, ok := exl3ReadModel(dir); ok {
			seen[k] = true
			out = append(out, m)
		}
	}
	scan := func(root string) {
		entries, _ := os.ReadDir(root)
		for _, e := range entries {
			if e.IsDir() {
				add(filepath.Join(root, e.Name()))
			}
		}
	}
	for _, d := range modelDirs() {
		scan(d)
		scan(filepath.Join(d, "exl3"))
	}
	if list, err := ListPresets(); err == nil {
		for _, p := range list {
			if c, err := ReadPreset(p.ID); err == nil {
				if cfg := parseEnv(c); isExl3Config(cfg) {
					if dir, err := exl3ResolveModel(cfg["EXL3_MODEL"]); err == nil {
						add(dir)
					}
				}
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name) })
	return out
}

// exl3ResolveModel : EXL3_MODEL est un chemin absolu ou un nom de dossier cherché
// dans les dossiers de modèles (et leur sous-dossier exl3).
func exl3ResolveModel(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("aucun modèle choisi (EXL3_MODEL)")
	}
	if filepath.IsAbs(name) {
		if isDir(name) {
			return name, nil
		}
		return "", fmt.Errorf("dossier du modèle introuvable : %s", name)
	}
	for _, d := range modelDirs() {
		for _, p := range []string{filepath.Join(d, "exl3", name), filepath.Join(d, name)} {
			if isDir(p) {
				return p, nil
			}
		}
	}
	return "", fmt.Errorf("modèle %s introuvable dans les dossiers de modèles", name)
}

var reHFTree = regexp.MustCompile(`^https?://huggingface\.co/([^/]+/[^/]+)(?:/tree/([^?#]+))?/?$`)

// exl3ParseHF : « https://huggingface.co/owner/repo/tree/branche », « owner/repo »
// ou « owner/repo:branche » → dépôt et révision (main par défaut).
func exl3ParseHF(s string) (repo, rev string, err error) {
	s = strings.TrimRight(strings.TrimSpace(s), "/")
	if m := reHFTree.FindStringSubmatch(s); m != nil {
		repo, rev = m[1], m[2]
	} else if strings.Count(s, "/") == 1 && !strings.Contains(s, "://") {
		repo = s
		if i := strings.Index(s, ":"); i > 0 {
			repo, rev = s[:i], s[i+1:]
		}
	} else {
		return "", "", fmt.Errorf("lien Hugging Face attendu (https://huggingface.co/<dépôt>/tree/<branche>)")
	}
	if rev == "" {
		rev = "main"
	}
	if u, e := url.PathUnescape(rev); e == nil {
		rev = u
	}
	return repo, rev, nil
}

var reSafeName = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

// exl3DefaultName : « Qwen3.8-27B-exl3 » + « SC_4.00bpw_H5_V6 ».
func exl3DefaultName(repo, rev string) string {
	n := repo[strings.Index(repo, "/")+1:]
	if rev != "" && rev != "main" {
		n += "-" + rev
	}
	return strings.Trim(reSafeName.ReplaceAllString(n, "_"), "._")
}

type hfFile struct {
	Type string `json:"type"`
	Path string `json:"path"`
	Size int64  `json:"size"`
}

// exl3ListHF : les fichiers d'une révision d'un dépôt Hugging Face.
func exl3ListHF(repo, rev string) ([]hfFile, error) {
	next := "https://huggingface.co/api/models/" + repo + "/tree/" + url.PathEscape(rev) + "?recursive=true"
	var files []hfFile
	c := &http.Client{Timeout: 30 * time.Second}
	for next != "" {
		req, _ := http.NewRequest("GET", next, nil)
		if t := strings.TrimSpace(os.Getenv("HF_TOKEN")); t != "" {
			req.Header.Set("Authorization", "Bearer "+t)
		}
		resp, err := c.Do(req)
		if err != nil {
			return nil, err
		}
		if resp.StatusCode != 200 {
			resp.Body.Close()
			return nil, fmt.Errorf("Hugging Face : HTTP %d (dépôt ou branche introuvable ?)", resp.StatusCode)
		}
		var page []hfFile
		err = json.NewDecoder(resp.Body).Decode(&page)
		resp.Body.Close()
		if err != nil {
			return nil, err
		}
		for _, f := range page {
			if f.Type == "file" {
				files = append(files, f)
			}
		}
		next = ""
		if m := regexp.MustCompile(`<([^>]+)>;\s*rel="next"`).FindStringSubmatch(resp.Header.Get("Link")); m != nil {
			next = m[1]
		}
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("aucun fichier dans %s (%s)", repo, rev)
	}
	return files, nil
}

// Arrêt d'un téléchargement de modèle : aucune commande externe ne tourne, le
// bouton « arrêter » de la tâche lève ce drapeau (voir handleLlamacppJobCancel).
var (
	exl3DLMu      sync.Mutex
	exl3DLActive  bool
	exl3DLStopped bool
)

// exl3CancelDownload : demande l'arrêt du téléchargement en cours, s'il y en a un.
func exl3CancelDownload() bool {
	exl3DLMu.Lock()
	defer exl3DLMu.Unlock()
	if !exl3DLActive {
		return false
	}
	exl3DLStopped = true
	return true
}

func exl3DLCanceled() bool {
	exl3DLMu.Lock()
	defer exl3DLMu.Unlock()
	return exl3DLStopped
}

// exl3RunDownload : télécharge une révision complète d'un dépôt EXL3.
func exl3RunDownload(repo, rev, name string) {
	exl3DLMu.Lock()
	exl3DLActive, exl3DLStopped = true, false
	exl3DLMu.Unlock()
	defer func() {
		exl3DLMu.Lock()
		exl3DLActive = false
		exl3DLMu.Unlock()
	}()
	lcPhase("liste des fichiers de " + repo + " (" + rev + ")…")
	files, err := exl3ListHF(repo, rev)
	if err != nil {
		lcFail(err)
		return
	}
	dest := filepath.Join(exl3ModelsDir(), name)
	var total, todo int64
	for _, f := range files {
		total += f.Size
		if fi, err := os.Stat(filepath.Join(dest, filepath.FromSlash(f.Path))); err != nil || fi.Size() != f.Size {
			todo += f.Size
		}
	}
	if free := diskFree(modelsDir()); free > 0 && todo > free-2e9 {
		lcFail(fmt.Errorf("espace disque insuffisant : %.1f Go à télécharger, %.1f Go libres", float64(todo)/1e9, float64(free)/1e9))
		return
	}
	lcAppend(fmt.Sprintf("%d fichiers, %.1f Go (dont %.1f Go à télécharger) → %s", len(files), float64(total)/1e9, float64(todo)/1e9, dest))
	var done int64
	for i, f := range files {
		if exl3DLCanceled() {
			lcFail(fmt.Errorf("téléchargement annulé"))
			return
		}
		p := filepath.Join(dest, filepath.FromSlash(f.Path))
		if rel, err := filepath.Rel(dest, p); err != nil || strings.HasPrefix(rel, "..") {
			lcFail(fmt.Errorf("nom de fichier suspect : %s", f.Path))
			return
		}
		if fi, err := os.Stat(p); err == nil && fi.Size() == f.Size {
			done += f.Size
			continue
		}
		lcPhase(fmt.Sprintf("téléchargement %d/%d : %s (%.0f %%)", i+1, len(files), f.Path, float64(done)*100/float64(max(total, 1))))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			lcFail(err)
			return
		}
		u := "https://huggingface.co/" + repo + "/resolve/" + url.PathEscape(rev) + "/" + f.Path
		if err := exl3DownloadFile(u, p+".part", f.Size); err != nil {
			if exl3DLCanceled() {
				lcFail(fmt.Errorf("téléchargement annulé (les fichiers déjà reçus sont gardés)"))
				return
			}
			lcFail(fmt.Errorf("%s : %w", f.Path, err))
			return
		}
		if err := os.Rename(p+".part", p); err != nil {
			lcFail(err)
			return
		}
		done += f.Size
	}
	if _, ok := exl3ReadModel(dest); !ok {
		lcAppend("⚠ ce dossier ne ressemble pas à un modèle EXL3 (config.json sans quantization_config exl3)")
	}
	lcDone("modèle téléchargé : " + name)
}

// exl3DownloadFile : un fichier, annulable (le bouton « arrêter » de la tâche).
func exl3DownloadFile(u, dest string, size int64) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		t := time.NewTicker(time.Second)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				if exl3DLCanceled() {
					cancel()
					return
				}
			}
		}
	}()
	req, _ := http.NewRequestWithContext(ctx, "GET", u, nil)
	if t := strings.TrimSpace(os.Getenv("HF_TOKEN")); t != "" {
		req.Header.Set("Authorization", "Bearer "+t)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer f.Close()
	buf := make([]byte, 1<<20)
	var got, last int64
	for {
		n, rerr := resp.Body.Read(buf)
		if n > 0 {
			if _, werr := f.Write(buf[:n]); werr != nil {
				return werr
			}
			got += int64(n)
			if got-last >= 200<<20 && size > 0 {
				last = got
				lcAppend(fmt.Sprintf("⬇ %d / %d Mo", got/1_000_000, size/1_000_000))
			}
		}
		if rerr == io.EOF {
			return nil
		}
		if rerr != nil {
			return rerr
		}
	}
}

// ---------------------------------------------------------------------------
// Lancement
// ---------------------------------------------------------------------------

// exl3Preflight : ce sans quoi le moteur ne peut pas démarrer.
func exl3Preflight(cfg map[string]string) (*exl3Installed, string, error) {
	st := exl3ReadInstalled()
	if st == nil {
		return nil, "", fmt.Errorf("moteur ExLlama non installé : installez-le depuis la section Moteur")
	}
	dir, err := exl3ResolveModel(cfg["EXL3_MODEL"])
	if err != nil {
		return nil, "", err
	}
	if !isFile(filepath.Join(dir, "config.json")) {
		return nil, "", fmt.Errorf("%s n'est pas un modèle (config.json absent)", dir)
	}
	return st, dir, nil
}

func yq(s string) string { return strconv.Quote(s) }

// exl3Num : un entier positif du preset, ou def.
func exl3Num(cfg map[string]string, key string, def int) int {
	if n, err := strconv.Atoi(strings.TrimSpace(cfg[key])); err == nil && n > 0 {
		return n
	}
	return def
}

// exl3FloatList : « 5.5, 11 » → [5.5 11] (vide ou illisible → nil).
func exl3FloatList(s string) []float64 {
	var out []float64
	for _, p := range strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == ';' || r == ' ' }) {
		f, err := strconv.ParseFloat(strings.TrimSpace(p), 64)
		if err != nil || f < 0 {
			return nil
		}
		out = append(out, f)
	}
	return out
}

func yamlFloats(v []float64) string {
	s := make([]string, len(v))
	for i, f := range v {
		s[i] = strconv.FormatFloat(f, 'f', -1, 64)
	}
	return "[" + strings.Join(s, ", ") + "]"
}

// exl3CacheModes : les modes de cache acceptés (clé EXL3_CACHE_MODE).
var exl3CacheModes = map[string]bool{"FP16": true, "8,8": true, "6,6": true, "4,4": true, "8,6": true, "6,4": true, "Q8": true, "Q6": true, "Q4": true}

// exl3BuildConfig écrit la config de TabbyAPI pour ce preset (YAML, valeurs
// citées en JSON, qui est du YAML valide).
func exl3BuildConfig(cfg map[string]string, modelDir, apiKey string) string {
	get := func(k, def string) string {
		if v := strings.TrimSpace(cfg[k]); v != "" {
			return v
		}
		return def
	}
	ctx := exl3Num(cfg, "CTX", 32768)
	cache := max(exl3Num(cfg, "EXL3_CACHE", ctx), ctx)
	// exllamav3 alloue le cache par pages de 256 jetons
	cache = (cache + 255) / 256 * 256
	mode := get("EXL3_CACHE_MODE", "8,8")
	if !exl3CacheModes[mode] {
		mode = "8,8"
	}
	var b strings.Builder
	w := func(format string, a ...any) { fmt.Fprintf(&b, format+"\n", a...) }
	w("# Écrit par ajean à chaque lancement (preset ENGINE=exl3) : ne pas modifier.")
	w("network:")
	w("  host: %s", yq(get("HOST", "0.0.0.0")))
	w("  port: %d", exl3Num(cfg, "PORT", 8080))
	w("  disable_auth: %t", apiKey == "")
	w("  send_tracebacks: false")
	w("  web_ui: false")
	w("logging:")
	w("  log_prompt: false")
	w("  log_generation_params: false")
	w("  log_requests: false")
	w("model:")
	w("  model_dir: %s", yq(filepath.Dir(modelDir)))
	w("  model_name: %s", yq(filepath.Base(modelDir)))
	w("  max_seq_len: %d", ctx)
	w("  cache_size: %d", cache)
	w("  cache_mode: %s", yq(mode))
	w("  chunk_size: %d", exl3Num(cfg, "EXL3_CHUNK", 2048))
	w("  max_batch_size: %d", exl3Num(cfg, "EXL3_BATCH", 1))
	tp := cfg["EXL3_TP"] == "1"
	w("  tensor_parallel: %t", tp)
	if tp && cfg["EXL3_TP_BACKEND"] == "nccl" {
		w("  tensor_parallel_backend: nccl")
	}
	if split := exl3FloatList(cfg["EXL3_GPU_SPLIT"]); len(split) > 0 {
		w("  gpu_split_auto: false")
		w("  gpu_split: %s", yamlFloats(split))
	} else {
		w("  gpu_split_auto: true")
	}
	if r := exl3FloatList(cfg["EXL3_RESERVE"]); len(r) > 0 {
		w("  autosplit_reserve: %s", yamlFloats(r))
	}
	vision := cfg["EXL3_VISION"] == "1"
	w("  vision: %t", vision)
	vdev := strings.TrimSpace(cfg["EXL3_VISION_DEVICE"])
	w("  vision_offload: %t", vision && vdev == "cpu")
	w("  reasoning: true")
	if r := strings.TrimSpace(cfg["REASONING"]); r != "" && !reasoningActive(r) {
		// raisonnement coupé : imposé au gabarit, quel que soit le client
		w("  template_vars_force:")
		w("    enable_thinking: false")
	}
	if n := exl3Num(cfg, "REASONING_BUDGET", 0); n > 0 {
		w("  reasoning_budget_tokens: %d", n)
	}
	w("draft_model:")
	switch d := get("EXL3_DRAFT", "disabled"); d {
	case "mtp", "ngram":
		w("  draft_mode: %s", d)
		w("  draft_num_tokens: %d", exl3Num(cfg, "EXL3_DRAFT_TOKENS", 3))
	case "model":
		if dd, err := exl3ResolveModel(cfg["EXL3_DRAFT_MODEL"]); err == nil {
			w("  draft_mode: model")
			w("  draft_model_dir: %s", yq(filepath.Dir(dd)))
			w("  draft_model_name: %s", yq(filepath.Base(dd)))
			w("  draft_cache_mode: %s", yq(mode))
			w("  draft_num_tokens: %d", exl3Num(cfg, "EXL3_DRAFT_TOKENS", 4))
		} else {
			w("  draft_mode: disabled")
		}
	default:
		w("  draft_mode: disabled")
	}
	// MTP adaptatif : la longueur des brouillons suit le taux d'acceptation
	// (plafond = jetons anticipés). Mesuré moins bon à 4 utilisateurs : en option.
	if d := get("EXL3_DRAFT", "disabled"); cfg["EXL3_DRAFT_DYNAMIC"] == "1" && (d == "mtp" || d == "model") {
		w("  dynamic_draft: true")
		if c, err := strconv.ParseFloat(strings.TrimSpace(cfg["EXL3_DRAFT_CONFIDENCE"]), 64); err == nil && c > 0 && c < 1 {
			w("  draft_confidence: %s", strconv.FormatFloat(c, 'f', -1, 64))
		}
	}
	// Recopie : un passage du contexte répété mot pour mot avance de N jetons d'un coup.
	if n := exl3Num(cfg, "EXL3_HYBRID", 0); n > 0 {
		w("  hybrid_draft_tokens: %d", n)
	}
	return b.String()
}

// serveExl3 : la branche ExLlama de `ajean serve`. Écrit la config et la clé de
// TabbyAPI puis lui passe la main (systemd supervise directement le moteur).
func serveExl3(cfg map[string]string) error {
	st, modelDir, err := exl3Preflight(cfg)
	if err != nil {
		return err
	}
	key := readAPIKey()
	if key == "" {
		key = strings.TrimSpace(cfg["API_KEY"])
	}
	if err := os.WriteFile(exl3ConfigFile(), []byte(exl3BuildConfig(cfg, modelDir, key)), 0o600); err != nil {
		return err
	}
	// TabbyAPI lit ses clés dans api_tokens.yml, dans son dossier
	tokens := filepath.Join(st.Dir, "api_tokens.yml")
	if key != "" {
		body := "api_key: " + yq(key) + "\nadmin_key: " + yq(key) + "\n"
		if err := os.WriteFile(tokens, []byte(body), 0o600); err != nil {
			return err
		}
	} else {
		_ = os.Remove(tokens)
	}
	_ = os.Setenv("CUDA_DEVICE_ORDER", "PCI_BUS_ID")
	// Les cartes du preset (EXL3_GPUS, vide = toutes) font foi. Ni la clé machine
	// CUDA_VISIBLE_DEVICES (réglée pour llama.cpp, souvent « 1,0 ») ni la valeur de
	// l'unité systemd ne s'appliquent : elles inversaient ou masquaient les cartes.
	// Une clé à part, car CUDA_VISIBLE_DEVICES est reportée d'un preset à l'autre
	// (softPreservedKeys) : l'écrire ici cassait ensuite les presets llama.cpp.
	if v := strings.TrimSpace(cfg["EXL3_GPUS"]); v != "" {
		_ = os.Setenv("CUDA_VISIBLE_DEVICES", v)
	} else {
		_ = os.Unsetenv("CUDA_VISIBLE_DEVICES")
	}
	if v := strings.TrimSpace(cfg["EXL3_VISION_DEVICE"]); strings.HasPrefix(v, "cuda:") && cfg["EXL3_VISION"] == "1" {
		_ = os.Setenv("TABBY_VISION_DEVICE", v)
	} else {
		_ = os.Unsetenv("TABBY_VISION_DEVICE")
	}
	_ = os.Setenv("PYTHONUNBUFFERED", "1")
	port := strconv.Itoa(exl3Num(cfg, "PORT", 8080))
	if err := waitPortFree(firstNonEmpty(cfg["HOST"], "0.0.0.0"), port, 5*time.Second); err != nil {
		return err
	}
	py := exl3Python(st.Dir)
	args := []string{py, "main.py", "--config", exl3ConfigFile()}
	_ = os.Chdir(st.Dir)
	fmt.Fprintf(os.Stderr, "[ajean serve] ExLlama (exllamav3 %s, TabbyAPI %s)  model=%s  port=%s\n",
		firstNonEmpty(st.Exllama, "?"), shortSHA(st.Commit), filepath.Base(modelDir), port)
	_ = putBytes(bkState, engineCmdlineKey, []byte(engineCmdline(args)))
	return execServer(py, args)
}

// exl3VisionActive : le preset actif tourne sur ExLlama avec la vision.
func exl3VisionActive() bool {
	cfg := ReadConfig()
	return isExl3Config(cfg) && cfg["EXL3_VISION"] == "1"
}

// exl3QuantLabel : « EXL3 4.0 » pour les étiquettes de preset.
func exl3QuantLabel(cfg map[string]string) string {
	dir, err := exl3ResolveModel(cfg["EXL3_MODEL"])
	if err == nil {
		if m, ok := exl3ReadModel(dir); ok && m.BPW > 0 {
			return "EXL3 " + strconv.FormatFloat(m.BPW, 'f', -1, 64)
		}
	}
	return "EXL3"
}

// ---------------------------------------------------------------------------
// API
// ---------------------------------------------------------------------------

// handleExl3 (GET) : machine, moteur installé, mise à jour, modèles.
func handleExl3(w http.ResponseWriter, r *http.Request) {
	env := exl3Detect()
	out := map[string]any{"env": env, "pinned": exl3TabbyPinned, "models": exl3ListModels(), "models_dir": exl3ModelsDir()}
	if st := exl3ReadInstalled(); st != nil {
		out["installed"] = st
		if r.URL.Query().Get("check") == "1" {
			if sha, date, err := exl3LatestCommit(false); err == nil {
				out["latest"] = map[string]any{"commit": sha, "date": date, "available": sha != st.Commit}
			} else {
				out["latest_error"] = err.Error()
			}
		}
	}
	sendJSON(w, 200, out)
}

// handleExl3Install (POST {latest}) : installe (commit validé) ou met à jour
// (dernier commit). Une seule tâche à la fois, partagée avec llama.cpp.
func handleExl3Install(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		sendJSON(w, 405, map[string]any{"ok": false, "error": "POST attendu"})
		return
	}
	var req struct {
		Latest bool `json:"latest"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	commit := exl3TabbyPinned
	action := "exl3"
	if req.Latest {
		commit, action = "", "exl3-update"
	}
	if err := startLcJob(action, func() { exl3RunInstall(commit) }); err != nil {
		sendJSON(w, 409, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	sendJSON(w, 200, map[string]any{"ok": true})
}

// handleExl3Uninstall (POST) : retire le moteur, garde les modèles.
func handleExl3Uninstall(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		sendJSON(w, 405, map[string]any{"ok": false, "error": "POST attendu"})
		return
	}
	if err := exl3Uninstall(); err != nil {
		sendJSON(w, 409, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	sendJSON(w, 200, map[string]any{"ok": true})
}

// handleExl3Download (POST {url, name}) : télécharge un modèle EXL3 (une
// branche complète d'un dépôt Hugging Face) dans models/exl3.
func handleExl3Download(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		sendJSON(w, 405, map[string]any{"ok": false, "error": "POST attendu"})
		return
	}
	var req struct {
		URL  string `json:"url"`
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendJSON(w, 400, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	repo, rev, err := exl3ParseHF(req.URL)
	if err != nil {
		sendJSON(w, 400, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	name := strings.Trim(reSafeName.ReplaceAllString(strings.TrimSpace(req.Name), "_"), "._")
	if name == "" {
		name = exl3DefaultName(repo, rev)
	}
	if err := startLcJob("exl3-model", func() { exl3RunDownload(repo, rev, name) }); err != nil {
		sendJSON(w, 409, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	sendJSON(w, 200, map[string]any{"ok": true, "name": name})
}

// ---------------------------------------------------------------------------
// Ligne de commande : ajean exl3 install | update | download <lien> | status | uninstall
// ---------------------------------------------------------------------------

func cmdExl3(args []string) error {
	sub := "status"
	if len(args) > 0 {
		sub = args[0]
	}
	switch sub {
	case "install", "update":
		commit := exl3TabbyPinned
		action := "exl3"
		if sub == "update" {
			commit, action = "", "exl3-update"
		}
		return exl3RunCLI(action, func() { exl3RunInstall(commit) })
	case "download":
		if len(args) < 2 {
			return fmt.Errorf("usage : ajean exl3 download <https://huggingface.co/<dépôt>/tree/<branche>> [nom]")
		}
		repo, rev, err := exl3ParseHF(args[1])
		if err != nil {
			return err
		}
		name := exl3DefaultName(repo, rev)
		if len(args) > 2 {
			name = strings.Trim(reSafeName.ReplaceAllString(args[2], "_"), "._")
		}
		return exl3RunCLI("exl3-model", func() { exl3RunDownload(repo, rev, name) })
	case "uninstall":
		if err := exl3Uninstall(); err != nil {
			return err
		}
		fmt.Println("ExLlama désinstallé (modèles conservés dans " + exl3ModelsDir() + ")")
		return nil
	case "status":
		env := exl3DetectNow()
		if !env.Supported {
			fmt.Println("machine : " + env.Reason)
		} else {
			fmt.Printf("machine : pilote %s, roues %s, %.0f Go libres\n", env.Driver, env.CUDA, env.DiskFreeGB)
		}
		if st := exl3ReadInstalled(); st != nil {
			fmt.Printf("installé : exllamav3 %s, TabbyAPI %s, torch %s (%s)\n", st.Exllama, shortSHA(st.Commit), st.Torch, st.Dir)
			if sha, date, err := exl3LatestCommit(true); err == nil && sha != st.Commit {
				fmt.Printf("mise à jour disponible : TabbyAPI %s (%s) → ajean exl3 update\n", shortSHA(sha), date)
			}
		} else {
			fmt.Println("non installé → ajean exl3 install")
		}
		for _, m := range exl3ListModels() {
			fmt.Printf("modèle : %s  %.1f Go  %.2f bpw  vision=%t  %s\n", m.Name, m.SizeGB, m.BPW, m.Vision, m.Path)
		}
		return nil
	}
	return fmt.Errorf("sous-commande inconnue : %s (install | update | download | status | uninstall)", sub)
}

// exl3RunCLI déroule une tâche dans le terminal : la sortie des commandes y
// arrive directement, les étapes et le résultat sont affichés au fil de l'eau.
// La tâche est aussi visible dans l'interface web (même journal sur disque).
func exl3RunCLI(action string, run func()) error {
	if err := startLcJob(action, run); err != nil {
		return err
	}
	seen := 0
	for {
		time.Sleep(500 * time.Millisecond)
		snap := lcJobSnapshot(seen, true)
		if lines, ok := snap["lines"].([]string); ok {
			for _, l := range lines {
				if strings.HasPrefix(l, "▶ ") || strings.HasPrefix(l, "✓ ") || strings.HasPrefix(l, "✗ ") || strings.HasPrefix(l, "⬇ ") || strings.HasPrefix(l, "⚠") {
					fmt.Println(l)
				}
			}
			seen = snap["next"].(int)
		}
		if snap["running"] == false {
			if e, _ := snap["error"].(string); e != "" {
				return fmt.Errorf("%s", e)
			}
			return nil
		}
	}
}
