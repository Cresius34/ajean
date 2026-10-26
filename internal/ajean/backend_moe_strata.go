package ajean

// Windows uses verified upstream CUDA/HIP releases. The Linux AJEAN MoE path
// keeps its measured settings. Python, DLLs and sources stay inside this data root.
import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Two distinct versions live here and must not be conflated: strataSourceVersion is
// the Strata release we vendor (tag), strataVersion is the engine package version that
// lands in engine/BUILD.json, and strataControlVersion is the control package shipped
// beside it. The preflight check below compares BUILD.json against strataVersion only.
const strataSourceVersion = "0.2.4"
const strataVersion = "0.1.48" // moteur Qwen : inchangé dans 0.2.4 (paquets 0.1.48, octet pour octet)
const strataControlVersion = "0.1.49.2"
const strataSourceCommit = "96ca4383c7b05a8a3e6064c65d9e9977d4b3bb59"
const strataPathToken = "${AJEAN_HOME}/"

//go:embed moe_portable_install.ps1
var strataInstallScript []byte

//go:embed moe_portable_setup.py
var strataSetupWrapper []byte

//go:embed moe_windows_manifest.json
var strataWindowsManifest []byte

func isStrataMoe(c map[string]string) bool {
	return isMoeConfig(c) && c["MOE_RUNTIME"] == "strata-windows"
}
func strataSourceDir(backend, toolkit string) string {
	if backend == "cuda" {
		backend += toolkit
	}
	return filepath.Join(moeHome(), "strata-"+strataVersion+"-"+backend)
}
func strataPath(p string) string {
	if p != "" && !filepath.IsAbs(p) {
		return filepath.Join(AjeanHome(), p)
	}
	return p
}
func strataPresetName(family, quant, backend string) string {
	return moePresetName(family, quant) + " · " + strings.ToUpper(backend)
}

// Replace only paths owned by this copy; never adopt another Strata installation.
func strataPortableJSON(value any, encode bool) (any, error) {
	switch v := value.(type) {
	case map[string]any:
		next := make(map[string]any, len(v))
		for k, x := range v {
			y, err := strataPortableJSON(x, encode)
			if err != nil {
				return nil, err
			}
			next[k] = y
		}
		return next, nil
	case []any:
		next := make([]any, len(v))
		for i, x := range v {
			y, err := strataPortableJSON(x, encode)
			if err != nil {
				return nil, err
			}
			next[i] = y
		}
		return next, nil
	case string:
		if encode && filepath.IsAbs(v) {
			if err := parallelWritablePath(v); err != nil {
				return nil, err
			}
			rel, err := filepath.Rel(AjeanHome(), v)
			if err != nil {
				return nil, err
			}
			return strataPathToken + filepath.ToSlash(rel), nil
		}
		if !encode && strings.HasPrefix(v, strataPathToken) {
			p := filepath.Join(AjeanHome(), filepath.FromSlash(strings.TrimPrefix(v, strataPathToken)))
			if err := parallelWritablePath(p); err != nil {
				return nil, err
			}
			return p, nil
		}
	}
	return value, nil
}

func strataReadConfig(path string) (map[string]any, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out map[string]any
	err = json.NewDecoder(io.LimitReader(f, 2<<20)).Decode(&out)
	return out, err
}
func strataWriteJSON(path string, value any) error {
	b, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	tmp := path + ".ajean-new"
	if err = os.WriteFile(tmp, append(b, '\n'), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func moeStrataPreflight(c map[string]string) (string, map[string]any, error) {
	if c["MOE_BACKEND"] != "hip" && c["MOE_BACKEND"] != "cuda" {
		return "", nil, errors.New("backend MoE invalide")
	}
	path, src, py := strataPath(c["MOE_CONFIG"]), strataPath(c["MOE_SOURCE"]), strataPath(c["MOE_PYTHON"])
	for _, p := range []string{path, src, py} {
		if p == "" {
			return "", nil, errors.New("installation MoE incomplète")
		}
		if err := parallelWritablePath(p); err != nil {
			return "", nil, err
		}
	}
	if !isFile(py) || !isFile(filepath.Join(src, "serve", "server.py")) {
		return "", nil, errors.New("Python ou serveur MoE absent : reprendre l’installation depuis Moteur")
	}
	base, err := strataReadConfig(path)
	if err != nil {
		return "", nil, err
	}
	value, err := strataPortableJSON(base, false)
	if err != nil {
		return "", nil, err
	}
	base = value.(map[string]any)
	exe, _ := base["exe"].(string)
	if !isFile(exe) {
		return "", nil, fmt.Errorf("moteur MoE absent : %s", exe)
	}
	if err := parallelWritablePath(exe); err != nil {
		return "", nil, err
	}
	meta, err := strataReadConfig(filepath.Join(filepath.Dir(exe), "BUILD.json"))
	if err != nil || meta["version"] != strataVersion {
		return "", nil, errors.New("moteur MoE différent de la version figée : reprendre son installation")
	}
	if c["MOE_BACKEND"] == "hip" && meta["backend"] != "hip" {
		return "", nil, errors.New("le preset HIP ne référence pas un moteur HIP")
	}
	toolkit := fmt.Sprint(meta["toolkit"])
	if toolkit == "<nil>" {
		toolkit = strings.Split(fmt.Sprint(meta["cuda"]), ".")[0]
	}
	if c["MOE_BACKEND"] == "cuda" && (meta["backend"] == "hip" || toolkit != c["MOE_TOOLKIT"]) {
		return "", nil, errors.New("le paquet CUDA ne correspond pas au preset")
	}
	if c["MOE_VISION"] == "1" {
		vision, ok := base["vision"].(map[string]any)
		if !ok || !isFile(fmt.Sprint(vision["exe"])) || !isFile(fmt.Sprint(vision["mmproj"])) {
			return "", nil, errors.New("encodeur vision absent ; préparer le modèle avec la vision avant de l’activer")
		}
	}
	return path, base, nil
}

func moeStrataBuildConfig(base map[string]any, c map[string]string, key string) (map[string]any, error) {
	b, err := json.Marshal(base)
	if err != nil {
		return nil, err
	}
	var out map[string]any
	if err = json.Unmarshal(b, &out); err != nil {
		return nil, err
	}
	list, ok := out["args"].([]any)
	if !ok {
		return nil, errors.New("configuration MoE sans arguments")
	}
	args := make([]string, len(list))
	for i, a := range list {
		args[i] = fmt.Sprint(a)
	}
	// Keep upstream's backend recommendations. Apply only explicit user settings.
	for flag, k := range map[string]string{"--max-context": "CTX", "--kv": "MOE_KV", "--spec": "MOE_SPEC", "--vram-reserve-mib": "MOE_VRAM_RESERVE"} {
		if s := c[k]; s != "" {
			args = setArg(args, flag, s)
		}
	}
	if c["MOE_VISION"] != "1" {
		delete(out, "vision")
		args = dropFlag(args, "--vision")
	}
	list = make([]any, len(args))
	for i, a := range args {
		list[i] = a
	}
	out["args"] = list
	out["host"] = "127.0.0.1"
	port := firstNonEmpty(c["PORT"], "8080")
	if parallelBuild() {
		port = parallelEnginePort()
	}
	out["port"], _ = strconv.Atoi(port)
	if key != "" {
		out["api_key"] = key
	} else {
		delete(out, "api_key")
	}
	out["log"] = filepath.Join(moeHome(), "logs", "strata-"+c["MOE_BACKEND"]+".log")
	delete(out, "mcp_servers") // tools are supplied by Ajean in the OpenAI request
	delete(out, "images")      // no unrelated image generator is installed or started
	return out, nil
}

func serveStrataMoe(c map[string]string) error {
	_, base, err := moeStrataPreflight(c)
	if err != nil {
		return err
	}
	final, err := moeStrataBuildConfig(base, c, readAPIKey())
	if err != nil {
		return err
	}
	run := filepath.Join(moeHome(), "run", "strata-"+c["MOE_BACKEND"]+".json")
	if err = strataWriteJSON(run, final); err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Join(moeHome(), "logs"), 0o700); err != nil {
		return err
	}
	if err = os.Chdir(strataPath(c["MOE_SOURCE"])); err != nil {
		return err
	}
	// The upstream server assigns its own actual CUDA/HIP device numbers. A
	// previous llama.cpp preset's visibility mask must not renumber them first.
	_ = os.Unsetenv("CUDA_VISIBLE_DEVICES")
	_ = os.Unsetenv("HIP_VISIBLE_DEVICES")
	py := strataPath(c["MOE_PYTHON"])
	args := []string{py, "-u", "-m", "serve.server", "--engine", "strata", "--config", run, "--host", "127.0.0.1", "--port", fmt.Sprint(final["port"])}
	_ = putBytes(bkState, engineCmdlineKey, []byte(engineCmdline(args)))
	fmt.Fprintf(os.Stderr, "[ajean serve] Strata %s · %s · port %v\n", strataVersion, c["MOE_BACKEND"], final["port"])
	return execServer(py, args)
}

func normalizeStrataInstall(r moeInstallReq) (moeInstallReq, error) {
	if r.Backend == "" {
		r.Backend = "hip"
	}
	if r.Backend != "hip" && r.Backend != "cuda" {
		return r, errors.New("choisir AMD HIP ou NVIDIA CUDA")
	}
	if r.Toolkit == "" {
		r.Toolkit = "13"
	}
	if r.Toolkit != "12" && r.Toolkit != "13" {
		return r, errors.New("version CUDA inconnue")
	}
	if r.Context == 0 {
		r.Context = 86016
	}
	if r.Context < 32768 || r.Context > moeMaxCtx || r.Context%4096 != 0 {
		return r, errors.New("contexte entre 32 768 et 262 144, multiple de 4 096")
	}
	if r.VRAMReserve == nil {
		n := 4096
		r.VRAMReserve = &n
	}
	if *r.VRAMReserve < 0 || *r.VRAMReserve > 65536 {
		return r, errors.New("réserve VRAM hors limites")
	}
	valid := false
	for _, f := range moeFamilies {
		if f.ID == r.Family && moeHas(f.Quants, r.Quant) {
			valid = true
		}
	}
	if !valid {
		return r, errors.New("famille ou quantification inconnue")
	}
	return r, nil
}

func runStrataInstallStep(dir, script string, args ...string) error {
	if err := os.MkdirAll(filepath.Join(moeHome(), "logs"), 0o700); err != nil {
		return err
	}
	log, err := os.OpenFile(filepath.Join(moeHome(), "logs", "installation.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	defer log.Close()
	cmd := hideCmd(exec.Command("powershell.exe", append([]string{"-NoLogo", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-File", script}, args...)...))
	cmd.Dir = dir
	cmd.Env = upsertEnv(os.Environ(), "PYTHONUNBUFFERED=1")
	cmd.Stdout = io.MultiWriter(log, &sinkWriter{})
	cmd.Stderr = cmd.Stdout
	prepareBoundChild(cmd)
	if err = cmd.Start(); err != nil {
		return err
	}
	bindChild(cmd)
	setCurCmd(cmd)
	err = cmd.Wait()
	setCurCmd(nil)
	if buildWasCanceled() {
		return errors.New("installation MoE annulée ; relancer reprend les téléchargements déjà terminés")
	}
	return err
}

func moeStrataInstall(req moeInstallReq) {
	if req.Backend == "" {
		req.Backend = moeStrataEnvironment("").Backend
	}
	r, err := normalizeStrataInstall(req)
	if err != nil {
		lcFail(err)
		return
	}
	env := moeStrataEnvironment(r.Backend)
	if !env.Supported {
		lcFail(errors.New(env.Reason))
		return
	}
	if f := moeFitFor(moeQuants[r.Quant], env, moeHaveGB(r.Family, r.Quant)); !f.OK {
		lcFail(errors.New(f.Why))
		return
	}
	if err = os.MkdirAll(moeHome(), 0o700); err != nil {
		lcFail(err)
		return
	}
	if err = parallelWritablePath(moeHome()); err != nil {
		lcFail(err)
		return
	}
	script := filepath.Join(moeHome(), "installer-r6.ps1")
	wrapper := filepath.Join(moeHome(), "ajean_setup.py")
	if err = os.WriteFile(script, append([]byte{0xef, 0xbb, 0xbf}, strataInstallScript...), 0o600); err != nil {
		lcFail(err)
		return
	}
	if err = os.WriteFile(wrapper, strataSetupWrapper, 0o600); err != nil {
		lcFail(err)
		return
	}
	if err = os.WriteFile(filepath.Join(moeHome(), "windows-manifest.json"), strataWindowsManifest, 0o600); err != nil {
		lcFail(err)
		return
	}
	args := []string{"-MoeHome", moeHome(), "-Backend", r.Backend, "-Toolkit", r.Toolkit, "-Family", r.Family, "-Quant", r.Quant, "-Context", strconv.Itoa(r.Context), "-ReserveMiB", strconv.Itoa(*r.VRAMReserve)}
	if r.Vision {
		args = append(args, "-Vision")
	}
	lcPhase(fmt.Sprintf("Strata %s · %s : préparation portable puis ~%.0f Go de poids", strataVersion, r.Backend, moeQuants[r.Quant].DownloadGB))
	if err = runStrataInstallStep(moeHome(), script, args...); err != nil {
		lcFail(fmt.Errorf("installation MoE : %w ; voir data/moe/logs/installation.log", err))
		return
	}
	src := strataSourceDir(r.Backend, r.Toolkit)
	path := filepath.Join(src, "strata-"+strings.ToLower(moeTag(r.Family, r.Quant))+".json")
	base, err := strataReadConfig(path)
	if err != nil {
		lcFail(err)
		return
	}
	portable, err := strataPortableJSON(base, true)
	if err != nil {
		lcFail(err)
		return
	}
	if err = strataWriteJSON(path, portable); err != nil {
		lcFail(err)
		return
	}
	rel := func(p string) string { v, _ := filepath.Rel(AjeanHome(), p); return v }
	cfg := map[string]string{"ENGINE": "moe", "MOE_RUNTIME": "strata-windows", "MOE_BACKEND": r.Backend, "MOE_TOOLKIT": r.Toolkit,
		"MOE_SOURCE": rel(src), "MOE_PYTHON": rel(filepath.Join(src, "python", "python.exe")), "MOE_CONFIG": rel(path),
		"MOE_KV": "int8", "MOE_SPEC": "3", "MOE_VISION": map[bool]string{true: "1", false: "0"}[r.Vision], "MOE_VRAM_RESERVE": strconv.Itoa(*r.VRAMReserve),
		"CTX": strconv.Itoa(r.Context), "HOST": "127.0.0.1", "PORT": "18080", "REASONING": "on"}
	// Carry the native agent/access and memory defaults; installing a MoE model
	// must not silently turn off the user's existing memory setup.
	for k, v := range ReadConfig() {
		if strings.HasPrefix(k, "MEMORY_") || k == "MEM_MODE" {
			cfg[k] = v
		}
	}
	id := strataPresetName(r.Family, r.Quant, r.Backend)
	if _, err = SavePreset(id, moeFamilyLabel(r.Family)+" "+r.Quant+" · "+strings.ToUpper(r.Backend), formatEnv(cfg)); err != nil {
		lcFail(err)
		return
	}
	lcPhase("modèle préparé ; activation dans le processus du TEST…")
	if err = SwitchToPreset(filepath.Join(presetsDir(), id+".env")); err != nil {
		lcFail(fmt.Errorf("installé, démarrage impossible : %w", err))
		return
	}
	lcDone("MoE installé ; chargement en cours, vérifier son état puis lancer le test du moteur")
}

func moeStrataDetails(c map[string]string) *moeDetails {
	n, _ := strconv.Atoi(c["MOE_VRAM_RESERVE"])
	base, _ := strataReadConfig(strataPath(c["MOE_CONFIG"]))
	_, visionAvailable := base["vision"].(map[string]any)
	return &moeDetails{Version: "Strata " + strataVersion, Ctx: c["CTX"], KV: c["MOE_KV"], Spec: c["MOE_SPEC"], Backend: c["MOE_BACKEND"], VRAMReserve: n, Modern: true,
		Vision: c["MOE_VISION"] == "1", VisionAvailable: visionAvailable, MainGPU: strings.ToUpper(c["MOE_BACKEND"]), Prefill: "auto", ShortRead: "auto", CacheRoot: "défaut du moteur"}
}
func moeStrataSettings(content string, r moeSettingsReq) (string, error) {
	c := parseEnv(content)
	if r.Ctx < 32768 || r.Ctx > moeMaxCtx || r.Ctx%4096 != 0 {
		return "", errors.New("contexte hors limites")
	}
	if r.KV != "int8" && r.KV != "q4_0" && r.KV != "k8v4" {
		return "", errors.New("cache KV inconnu")
	}
	if r.Spec < 2 || r.Spec > 4 {
		return "", errors.New("MTP hors limites")
	}
	if r.Helper || r.Split == "layers" {
		return "", errors.New("ce prototype utilise une carte par modèle ; pas de mélange AMD/NVIDIA")
	}
	if r.Vision && !moeStrataDetails(c).VisionAvailable {
		return "", errors.New("la vision doit être choisie lors de la préparation initiale de ce modèle")
	}
	c["CTX"], c["MOE_KV"], c["MOE_SPEC"] = strconv.Itoa(r.Ctx), r.KV, strconv.Itoa(r.Spec)
	c["MOE_VISION"] = map[bool]string{true: "1", false: "0"}[r.Vision]
	if r.VRAMReserve != nil {
		if *r.VRAMReserve < 0 || *r.VRAMReserve > 65536 {
			return "", errors.New("réserve VRAM hors limites")
		}
		c["MOE_VRAM_RESERVE"] = strconv.Itoa(*r.VRAMReserve)
	}
	return formatEnv(c), nil
}

var strataGPUCache struct {
	sync.Mutex
	At   time.Time
	GPUs []moeGPU
}

// Read-only discovery; no AMD SMI (which may ask for elevation on Windows).
func strataWindowsCards() []moeGPU {
	strataGPUCache.Lock()
	defer strataGPUCache.Unlock()
	if time.Since(strataGPUCache.At) < 30*time.Second {
		return append([]moeGPU(nil), strataGPUCache.GPUs...)
	}
	var cards []moeGPU
	if found, err := detectGPUs(); err == nil {
		for _, g := range found {
			mib, _ := strconv.ParseFloat(g.MemTotal, 64)
			cap, _ := strconv.ParseFloat(g.Cap, 64)
			if cap >= 6 && mib >= 7500 {
				cards = append(cards, moeGPU{Index: g.Index, Name: g.Name, VRAMGB: mib / 1024, Arch: int(cap*10 + .5), Backend: "cuda"})
			}
		}
	}
	if runtime.GOOS == "windows" {
		const ps = `$regs=@(Get-ChildItem 'HKLM:\SYSTEM\CurrentControlSet\Control\Class\{4d36e968-e325-11ce-bfc1-08002be10318}' -ErrorAction SilentlyContinue | Where-Object {$_.PSChildName -match '^\d{4}$'} | ForEach-Object {Get-ItemProperty $_.PSPath -ErrorAction SilentlyContinue}); $items=@(Get-CimInstance Win32_VideoController | Where-Object {$_.PNPDeviceID -match 'VEN_1002'} | ForEach-Object {$card=$_; $size=0; foreach($r in $regs){if($r.DriverDesc -eq $card.Name -and $r.'HardwareInformation.qwMemorySize' -gt $size){$size=$r.'HardwareInformation.qwMemorySize'}}; [PSCustomObject]@{name=$card.Name;pnp=$card.PNPDeviceID;bytes=$size}}); [PSCustomObject]@{items=$items} | ConvertTo-Json -Depth 3 -Compress`
		ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
		defer cancel()
		cmd := hideCmd(exec.CommandContext(ctx, "powershell.exe", "-NoProfile", "-NonInteractive", "-Command", ps))
		if b, err := cmd.Output(); err == nil {
			cards = append(cards, strataParseAMDCards(b)...)
		}
	}
	strataGPUCache.At, strataGPUCache.GPUs = time.Now(), cards
	return append([]moeGPU(nil), cards...)
}
func strataParseAMDCards(b []byte) []moeGPU {
	var input struct {
		Items []struct {
			Name  string
			PNP   string
			Bytes float64
		}
	}
	if json.Unmarshal(b, &input) != nil {
		return nil
	}
	var out []moeGPU
	for i, g := range input.Items {
		upper := strings.ToUpper(g.PNP + " " + g.Name)
		arch := ""
		for id, a := range map[string]string{"744C": "gfx1100", "7448": "gfx1100", "745E": "gfx1100", "747E": "gfx1101", "7480": "gfx1102", "7590": "gfx1200", "7550": "gfx1201", "7551": "gfx1201", "73BF": "gfx1030", "73AF": "gfx1030", "73A5": "gfx1030"} {
			if strings.Contains(upper, "DEV_"+id) {
				arch = a
				break
			}
		}
		if arch == "" && strings.Contains(upper, "R9700") {
			arch = "gfx1201"
		}
		if arch != "" {
			out = append(out, moeGPU{Index: i, Name: g.Name, VRAMGB: g.Bytes / (1 << 30), HIPArch: arch, Backend: "hip"})
		}
	}
	return out
}
func strataSelectEnvironment(cards []moeGPU, backend string, ram, disk float64) moeEnv {
	e := moeEnv{Main: -1, Helper: -1, RAMGB: ram, DiskFreeGB: disk, Backend: backend, Portable: true}
	for _, b := range []string{"hip", "cuda"} {
		for _, g := range cards {
			if g.Backend == b {
				e.Backends = append(e.Backends, b)
				break
			}
		}
	}
	if e.Backend == "" && len(e.Backends) > 0 {
		e.Backend = e.Backends[0]
	}
	for _, g := range cards {
		if g.Backend == e.Backend {
			e.GPUs = append(e.GPUs, g)
		}
	}
	if len(e.GPUs) == 0 {
		e.Visible = true
		e.Reason = "aucun GPU compatible détecté pour ce backend ; pilote requis"
		return e
	}
	sort.SliceStable(e.GPUs, func(i, j int) bool { return e.GPUs[i].VRAMGB > e.GPUs[j].VRAMGB })
	e.Main = e.GPUs[0].Index
	e.Visible = true
	if e.GPUs[0].VRAMGB > 0 && e.GPUs[0].VRAMGB < 7.5 {
		e.Reason = "8 Go de VRAM minimum"
		return e
	}
	e.Supported = true
	return e
}
func moeStrataEnvironment(backend string) moeEnv {
	disk := float64(diskFree(AjeanHome())) / 1e9
	return strataSelectEnvironment(strataWindowsCards(), backend, totalRAMGB(), disk)
}

func handleStrataMoe(w http.ResponseWriter, r *http.Request) {
	e := moeStrataEnvironment(r.URL.Query().Get("backend"))
	type family struct {
		moeFamily
		Fits      []moeFit `json:"fits"`
		Recommend string   `json:"recommend"`
	}
	var families []family
	var installed []string
	list, _ := ListPresets()
	presets := map[string]map[string]string{}
	backends := map[string]string{}
	active := map[string]bool{}
	for _, p := range list {
		raw, err := ReadPreset(p.ID)
		if err != nil {
			continue
		}
		c := parseEnv(raw)
		if isStrataMoe(c) {
			installed = append(installed, p.ID)
			presets[p.ID] = c
			backends[p.ID] = c["MOE_BACKEND"]
			active[p.ID] = p.Active
		}
	}
	for _, f := range moeFamilies {
		v := family{moeFamily: f, Recommend: moeRecommend(f.ID, e)}
		for _, q := range f.Quants {
			fit := moeFitFor(moeQuants[q], e, moeHaveGB(f.ID, q))
			id := strataPresetName(f.ID, q, e.Backend)
			if c, ok := presets[id]; ok {
				fit.Preset = id
				fit.Active = active[id]
				fit.Details = moeStrataDetails(c)
			}
			v.Fits = append(v.Fits, fit)
		}
		families = append(families, v)
	}
	sendJSON(w, 200, map[string]any{"version": "Strata " + strataVersion, "env": e, "families": families, "installed": installed, "preset_backends": backends, "modern": true, "default_context": 86016, "default_vram_reserve_mib": 4096})
}
