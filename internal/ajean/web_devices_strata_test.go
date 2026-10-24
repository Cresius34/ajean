package ajean

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// La sortie réelle de `strata-device --list-devices` sur la machine de test
// (moteur MoE Windows, backend HIP), relevée le 2026-10-17.
const strataDeviceSample = `device 0: AMD Radeon AI PRO R9700
  arch gfx1201, 31.9 GiB, wave32
`

// Deux cartes, sans nom commercial (l'arch serv de nom), et une ligne de
// mémoire absente : la liste doit rester exploitable.
const strataDeviceSample2 = `device 0: gfx1100
  arch gfx1100, 16.0 GiB
device 1: NVIDIA GeForce RTX 4060
  arch sm_89, 8.0 GiB
device 2: inconnu
`

func TestParseStrataDevices(t *testing.T) {
	devs := parseStrataDevices(strataDeviceSample)
	if len(devs) != 1 {
		t.Fatalf("1 device attendu, got %d (%+v)", len(devs), devs)
	}
	d := devs[0]
	if d["id"] != "GPU0" {
		t.Errorf("id = %v, attendu GPU0", d["id"])
	}
	if d["name"] != "AMD Radeon AI PRO R9700" {
		t.Errorf("name = %v", d["name"])
	}
	if d["total_mib"] != 32665 { // 31.9 * 1024
		t.Errorf("total_mib = %v, attendu 32665", d["total_mib"])
	}
	if d["free_mib"] != d["total_mib"] {
		t.Errorf("free_mib = %v, attendu = total", d["free_mib"])
	}
}

func TestParseStrataDevicesMulti(t *testing.T) {
	devs := parseStrataDevices(strataDeviceSample2)
	if len(devs) != 3 {
		t.Fatalf("3 devices attendus, got %d (%+v)", len(devs), devs)
	}
	if devs[0]["name"] != "Radeon RX 7900" {
		t.Errorf("sans nom commercial : name = %v, attendu un nom d'arch", devs[0]["name"])
	}
	if devs[1]["name"] != "NVIDIA GeForce RTX 4060" {
		t.Errorf("name = %v", devs[1]["name"])
	}
	if devs[1]["total_mib"] != 8192 {
		t.Errorf("total_mib = %v, attendu 8192", devs[1]["total_mib"])
	}
	if n, _ := devs[2]["total_mib"].(int); n != 0 {
		t.Errorf("sans ligne de mémoire : total_mib = %v, attendu 0", n)
	}
}

func TestParseStrataDevicesVide(t *testing.T) {
	for _, in := range []string{"", "\n", "no devices\n", "usage: strata-device ...\n"} {
		if devs := parseStrataDevices(in); len(devs) != 0 {
			t.Errorf("entrée %q : 0 device attendu, got %+v", in, devs)
		}
	}
}

// fakeStrataEngine fabrique un vrai binaire `strata-device.exe` qui imprime le
// contenu de son fichier `strata-out.txt` (à côté de lui), à côté d'un
// `strata.exe` factice : sous Windows un simple fichier texte n'est pas
// lançable, il faut compiler.
func fakeStrataEngine(t *testing.T, out string) string {
	t.Helper()
	dir := t.TempDir()
	src := filepath.Join(dir, "dev.go")
	body := "package main\n" +
		"import (\"fmt\"\n\"os\"\n\"path/filepath\"\n\"runtime\")\n" +
		"func main() {\n" +
		"  ex, _ := os.Executable()\n" +
		"  b, _ := os.ReadFile(filepath.Join(filepath.Dir(ex), \"strata-out.txt\"))\n" +
		"  fmt.Print(string(b))\n" +
		"  _ = runtime.GOOS\n" +
		"}\n"
	if err := os.WriteFile(src, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "strata-out.txt"), []byte(out), 0o644); err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(dir, "strata-device.exe")
	build := exec.Command("go", "build", "-o", exe, src)
	build.Stdout, build.Stderr = io.Discard, io.Discard
	if err := build.Run(); err != nil {
		t.Skip("go build indisponible pour le faux moteur :", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "strata.exe"), []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	return filepath.Join(dir, "strata.exe")
}

// Le parcours complet de l'encart « appareils » avec un moteur MoE :
// /api/backends/devices, bin = strata.exe. C'est la seule voie d'énumération
// sur une machine AMD sous Windows (ni nvidia-smi, ni amd-smi/rocm-smi).
func TestStrataDeviceTool(t *testing.T) {
	bin := fakeStrataEngine(t, strataDeviceSample)

	tool := strataDeviceTool(bin)
	if tool == "" {
		t.Fatal("strataDeviceTool n'a pas trouvé strata-device.exe")
	}
	if filepath.Base(tool) != "strata-device.exe" {
		t.Errorf("tool = %v", tool)
	}

	body, _ := json.Marshal(map[string]any{"bin": bin})
	req := httptest.NewRequest("POST", "/api/backends/devices", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handleBackendDevices(rec, req)

	if rec.Code != 200 {
		t.Fatalf("code %d", rec.Code)
	}
	var resp struct {
		OK      bool             `json:"ok"`
		Err     string           `json:"error"`
		Devices []map[string]any `json:"devices"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("corps illisible : %v — %s", err, rec.Body.String())
	}
	if !resp.OK {
		t.Fatalf("ok=false : %s", resp.Err)
	}
	t.Logf("corps : %s", rec.Body.String())
	if len(resp.Devices) != 1 {
		t.Fatalf("1 device attendu, got %+v", resp.Devices)
	}
	d := resp.Devices[0]
	if d["name"] != "AMD Radeon AI PRO R9700" {
		t.Errorf("name = %v", d["name"])
	}
	if d["id"] != "GPU0" {
		t.Errorf("id = %v", d["id"])
	}
	// Un nombre JSON se décode en float64, pas en int.
	if n, _ := d["total_mib"].(float64); n != 32665 {
		t.Errorf("total_mib = %v, attendu 32665 (31.9 GiB)", d["total_mib"])
	}
	if n, _ := d["free_mib"].(float64); n != 32665 {
		t.Errorf("free_mib = %v, attendu = total", d["free_mib"])
	}
}

// TestDevicesRouteOnMux : le parcours complet, du client HTTP jusqu'au moteur
// MoE, sur le vrai mux de l'application (route enregistrée + middleware
// d'authentification). C'est ce qu'emprunte l'encart « appareils » de l'UI.
// Un AJEAN_HOME neuf n'a pas de clé de pilotage : requireWebAuth laisse passer.
func TestDevicesRouteOnMux(t *testing.T) {
	t.Setenv("AJEAN_HOME", t.TempDir())
	bin := fakeStrataEngine(t, strataDeviceSample)

	srv := httptest.NewServer(newWebMux())
	defer srv.Close()

	body, _ := json.Marshal(map[string]any{"bin": bin})
	resp, err := http.Post(srv.URL+"/api/backends/devices", "application/json", strings.NewReader(string(body)))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		t.Fatalf("code %d — %s", resp.StatusCode, raw)
	}
	var out struct {
		OK      bool             `json:"ok"`
		Err     string           `json:"error"`
		Devices []map[string]any `json:"devices"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("corps illisible : %v — %s", err, raw)
	}
	if !out.OK || len(out.Devices) != 1 {
		t.Fatalf("réponse inattendue : %s", raw)
	}
	if name, _ := out.Devices[0]["name"].(string); name != "AMD Radeon AI PRO R9700" {
		t.Errorf("name = %v", name)
	}
	if n, _ := out.Devices[0]["total_mib"].(float64); n != 32665 {
		t.Errorf("total_mib = %v, attendu 32665", out.Devices[0]["total_mib"])
	}

	// Sans moteur choisi, la route répond « choisissez d'abord un moteur » :
	// c'est le message que verra l'UI, pas une exécution aveugle.
	resp2, err := http.Post(srv.URL+"/api/backends/devices", "application/json", strings.NewReader(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp2.Body.Close()
	raw2, _ := io.ReadAll(resp2.Body)
	var out2 struct {
		OK  bool   `json:"ok"`
		Err string `json:"error"`
	}
	if json.Unmarshal(raw2, &out2) != nil {
		t.Fatalf("corps illisible : %s", raw2)
	}
	if out2.OK || !strings.Contains(out2.Err, "moteur") {
		t.Errorf("attendu une invitation à choisir un moteur, got %s", raw2)
	}
}

// TestStrataDeviceToolRealEngine : la même voie, exécutée contre le moteur MoE
// réellement installé sur cette machine. Non bloquant (skip) quand le moteur
// n'est pas là : il prouve que le format analysé est bien celui de la vraie
// sortie, pas celui d'un binaire fabriqué pour le test.
func TestStrataDeviceToolRealEngine(t *testing.T) {
	bin := os.Getenv("AJEAN_TEST_STRATA_BIN")
	if bin == "" {
		bin = `C:\ProgramData\Ajean-Memoire-MoE-Test\data\moe\strata-0.1.48-hip\engine\strata.exe`
	}
	if !isFile(bin) {
		t.Skip("moteur MoE non installé ici :", bin)
	}
	tool := strataDeviceTool(bin)
	if tool == "" {
		t.Fatalf("strata-device.exe absent à côté de %v", bin)
	}
	body, _ := json.Marshal(map[string]any{"bin": bin})
	req := httptest.NewRequest("POST", "/api/backends/devices", strings.NewReader(string(body)))
	rec := httptest.NewRecorder()
	handleBackendDevices(rec, req)

	t.Logf("réponse du moteur réel : %s", rec.Body.String())
	var resp struct {
		OK      bool             `json:"ok"`
		Err     string           `json:"error"`
		Devices []map[string]any `json:"devices"`
	}
	if json.Unmarshal(rec.Body.Bytes(), &resp) != nil {
		t.Fatalf("corps illisible : %s", rec.Body.String())
	}
	if !resp.OK {
		t.Fatalf("ok=false : %s", resp.Err)
	}
	if len(resp.Devices) == 0 {
		t.Fatal("aucun device renvoyé par le moteur réel")
	}
	for _, d := range resp.Devices {
		if name, _ := d["name"].(string); name == "" {
			t.Errorf("device sans nom : %+v", d)
		}
		if n, _ := d["total_mib"].(float64); n <= 0 {
			t.Errorf("device sans mémoire : %+v", d)
		}
	}
}

// Un moteur llama-server ordinaire garde sa voie --list-devices ; un chemin
// qui n'est ni Strata ni llama-server est refusé sans exécution.
func TestStrataDeviceToolIgnoresLlama(t *testing.T) {
	dir := t.TempDir()
	llama := filepath.Join(dir, "llama-server.exe")
	_ = os.WriteFile(llama, []byte("x"), 0o755)
	if tool := strataDeviceTool(llama); tool != "" {
		t.Errorf("un llama-server ne doit pas déclencher la voie Strata, got %v", tool)
	}

	other := filepath.Join(dir, "queque-chose.exe")
	_ = os.WriteFile(other, []byte("x"), 0o755)
	body, _ := json.Marshal(map[string]any{"bin": other})
	req := httptest.NewRequest("POST", "/api/backends/devices", strings.NewReader(string(body)))
	rec := httptest.NewRecorder()
	handleBackendDevices(rec, req)
	if rec.Code != 400 {
		t.Errorf("code %d, attendu 400 pour un binaire non autorisé", rec.Code)
	}
}
