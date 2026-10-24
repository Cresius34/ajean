package ajean

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func strataFixture(t *testing.T, backend string) map[string]string {
	t.Helper()
	parallelTestHome(t)
	src := strataSourceDir(backend, "13")
	c := map[string]string{"ENGINE": "moe", "MOE_RUNTIME": "strata-windows", "MOE_BACKEND": backend, "MOE_TOOLKIT": "13", "MOE_CONFIG": filepath.Join(src, "strata-swift-iq3_xxs.json"), "MOE_SOURCE": src, "MOE_PYTHON": filepath.Join(src, "python", "python.exe"), "CTX": "86016", "MOE_KV": "int8", "MOE_SPEC": "3", "MOE_VRAM_RESERVE": "4096", "MOE_VISION": "0", "MEMORY_SYSTEM": "classic"}
	exe := filepath.Join(src, "engine", "strata.exe")
	for _, p := range []string{exe, c["MOE_PYTHON"], filepath.Join(src, "serve", "server.py")} {
		if err := os.MkdirAll(filepath.Dir(p), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("fixture, never executed"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	meta := map[string]any{"version": strataVersion, "cuda": "13.0", "source": "release"}
	if backend == "hip" {
		meta["backend"] = "hip"
	}
	if err := strataWriteJSON(filepath.Join(filepath.Dir(exe), "BUILD.json"), meta); err != nil {
		t.Fatal(err)
	}
	base := map[string]any{"exe": exe, "args": []any{"--pack", filepath.Join(moeDataDir(), "packs", "swift-iq3_xxs"), "--max-context", "86016", "--prefill", "16384"}, "env": map[string]any{"STRATA_RESIDENT_PIN": "0"}}
	encoded, err := strataPortableJSON(base, true)
	if err != nil {
		t.Fatal(err)
	}
	if err = strataWriteJSON(c["MOE_CONFIG"], encoded); err != nil {
		t.Fatal(err)
	}
	return c
}

func TestStrataInstallValidation(t *testing.T) {
	base := moeInstallReq{Family: "swift", Quant: "IQ3_XXS"}
	r, err := normalizeStrataInstall(base)
	if err != nil {
		t.Fatal(err)
	}
	if r.Backend != "hip" || r.Toolkit != "13" || r.Context != 86016 || *r.VRAMReserve != 4096 {
		t.Fatal(r)
	}
	zero := 0
	base.VRAMReserve = &zero
	if r, err = normalizeStrataInstall(base); err != nil || *r.VRAMReserve != 0 {
		t.Fatal("explicit zero lost", err)
	}
	for _, change := range []func(*moeInstallReq){func(r *moeInstallReq) { r.Backend = "vulkan" }, func(r *moeInstallReq) { r.Toolkit = "14" }, func(r *moeInstallReq) { r.Context = 86000 }, func(r *moeInstallReq) { r.Context = 4096 }, func(r *moeInstallReq) { n := -1; r.VRAMReserve = &n }, func(r *moeInstallReq) { r.Quant = "Q8_0" }} {
		r = base
		change(&r)
		if _, err = normalizeStrataInstall(r); err == nil {
			t.Fatalf("invalid request accepted: %+v", r)
		}
	}
}
func TestStrataAMDDiscoveryAndBackendSeparation(t *testing.T) {
	cards := strataParseAMDCards([]byte(`{"items":[{"name":"AMD Radeon Graphics","pnp":"PCI\\VEN_1002&DEV_164E","bytes":536870912},{"name":"AMD Radeon AI PRO R9700","pnp":"PCI\\VEN_1002&DEV_7551","bytes":34359738368}]}`))
	if len(cards) != 1 || cards[0].HIPArch != "gfx1201" || cards[0].VRAMGB != 32 {
		t.Fatal(cards)
	}
	cards = append(cards, moeGPU{Index: 0, Name: "NVIDIA", Backend: "cuda", VRAMGB: 24})
	for _, backend := range []string{"hip", "cuda"} {
		e := strataSelectEnvironment(cards, backend, 128, 1000)
		if !e.Supported || len(e.GPUs) != 1 || e.GPUs[0].Backend != backend || e.Helper != -1 {
			t.Fatal(e)
		}
	}
	if e := strataSelectEnvironment(nil, "hip", 128, 1000); e.Supported || !e.Visible || e.Reason == "" {
		t.Fatal(e)
	}
}
func TestStrataConfigKeepsBackendRecommendations(t *testing.T) {
	parallelTestHome(t)
	base := map[string]any{"args": []any{"--prefill", "16384", "--route-tail-skip", "2", "--vision"}, "vision": map[string]any{"exe": "v"}, "env": map[string]any{"STRATA_RESIDENT_PIN": "0"}, "images": map[string]any{}, "mcp_servers": []any{}}
	before, _ := json.Marshal(base)
	c := map[string]string{"MOE_BACKEND": "hip", "CTX": "86016", "MOE_KV": "k8v4", "MOE_SPEC": "4", "MOE_VRAM_RESERVE": "4096", "PORT": "8080"}
	out, err := moeStrataBuildConfig(base, c, "secret")
	if err != nil {
		t.Fatal(err)
	}
	args := moeArgs(t, out)
	for flag, want := range map[string]string{"--prefill": "16384", "--route-tail-skip": "2", "--max-context": "86016", "--kv": "k8v4", "--spec": "4", "--vram-reserve-mib": "4096"} {
		if argAfter(args, flag) != want {
			t.Fatalf("%s: %v", flag, args)
		}
	}
	if out["port"] != 18080 || out["host"] != "127.0.0.1" || out["api_key"] != "secret" || hasArg(args, "--vision") {
		t.Fatal(out)
	}
	for _, key := range []string{"vision", "images", "mcp_servers"} {
		if _, ok := out[key]; ok {
			t.Fatal("unrequested component", key)
		}
	}
	if !reflect.DeepEqual(out["env"], base["env"]) {
		t.Fatal("backend environment overwritten")
	}
	after, _ := json.Marshal(base)
	if string(before) != string(after) {
		t.Fatal("base mutated")
	}
}
func TestStrataPathsMoveAndRejectEscapes(t *testing.T) {
	home := parallelTestHome(t)
	original := map[string]any{"exe": filepath.Join(home, "moe", "engine", "strata.exe"), "args": []any{"--pack", filepath.Join(home, "moe", "data", "pack")}}
	encoded, err := strataPortableJSON(original, true)
	if err != nil {
		t.Fatal(err)
	}
	other := t.TempDir()
	t.Setenv("AJEAN_HOME", other)
	decoded, err := strataPortableJSON(encoded, false)
	if err != nil {
		t.Fatal(err)
	}
	if got := decoded.(map[string]any)["exe"]; got != filepath.Join(other, "moe", "engine", "strata.exe") {
		t.Fatal(got)
	}
	for _, value := range []string{strataPathToken + "../production/secret", strataPathToken + "../../outside"} {
		if _, err = strataPortableJSON(value, false); err == nil {
			t.Fatal("escape accepted", value)
		}
	}
	outside := t.TempDir()
	if err = os.Symlink(outside, filepath.Join(other, "linked")); err == nil {
		if _, err = strataPortableJSON(strataPathToken+"linked/data", false); err == nil {
			t.Fatal("symlink escape accepted")
		}
	}
}
func TestStrataPresetSaveSettingsAndReload(t *testing.T) {
	c := strataFixture(t, "hip")
	id := strataPresetName("swift", "IQ3_XXS", "hip")
	if _, err := SavePreset(id, "Swift MoE", formatEnv(c)); err != nil {
		t.Fatal(err)
	}
	raw, err := ReadPreset(id)
	if err != nil {
		t.Fatal(err)
	}
	saved := parseEnv(raw)
	if filepath.IsAbs(saved["MOE_PYTHON"]) || saved["MEMORY_SYSTEM"] != "classic" {
		t.Fatal(raw)
	}
	n := 6144
	raw, err = moeStrataSettings(raw, moeSettingsReq{Ctx: 65536, KV: "q4_0", Spec: 2, VRAMReserve: &n})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = SavePreset(id, "Swift MoE", raw); err != nil {
		t.Fatal(err)
	}
	if err = applyPresetFile(filepath.Join(presetsDir(), id+".env")); err != nil {
		t.Fatal(err)
	}
	current := ReadConfig()
	if current["ENGINE"] != "moe" || current["MEMORY_SYSTEM"] != "classic" || current["CTX"] != "65536" || current["MOE_VRAM_RESERVE"] != "6144" {
		t.Fatal(current)
	}
	if _, _, err = moeStrataPreflight(current); err != nil {
		t.Fatal(err)
	}
	if _, err = moeStrataSettings(raw, moeSettingsReq{Ctx: 65536, KV: "int8", Spec: 3, Vision: true}); err == nil {
		t.Fatal("HIP vision accepted")
	}
}

// TestStrataHipVisionPackage : le paquet engine 0.1.48 de Strata 0.2.4 embarque le
// lecteur d'images AMD (BUILD.json vision "cpu+hip", strata-vision-hip.exe a cote de
// l'engine). La vision n'est donc plus refusee sur HIP quand le modele a ete prepare
// avec elle ; elle reste refusee quand le paquet ne la porte pas.
func TestStrataHipVisionPackage(t *testing.T) {
	c := strataFixture(t, "hip")
	if _, err := moeStrataSettings(formatEnv(c), moeSettingsReq{Ctx: 86016, KV: "int8", Spec: 3, Vision: true}); err == nil {
		t.Fatal("vision acceptee alors que le fixture ne porte pas d'encodeur")
	}
	// prepare the same model with the AMD picture reader, as setup does with a 0.1.48 package
	cfgPath := c["MOE_CONFIG"]
	base, err := strataReadConfig(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	base["vision"] = map[string]any{"exe": filepath.Join(strataSourceDir("hip", "13"), "engine", "strata-vision-hip.exe"), "mmproj": filepath.Join(moeDataDir(), "mmproj.gguf")}
	base["args"] = append(base["args"].([]any), "--vision") // setup writes it beside the vision section
	if err = strataWriteJSON(cfgPath, base); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{fmt.Sprint(base["vision"].(map[string]any)["exe"]), fmt.Sprint(base["vision"].(map[string]any)["mmproj"])} {
		if err = os.MkdirAll(filepath.Dir(p), 0700); err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(p, []byte("fixture, never executed"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	c["MOE_VISION"] = "1"
	if _, _, err = moeStrataPreflight(c); err != nil {
		t.Fatal("encodeur vision HIP refuse :", err)
	}
	if !moeStrataDetails(c).VisionAvailable {
		t.Fatal("encodeur vision HIP non signale disponible")
	}
	out, err := moeStrataSettings(formatEnv(c), moeSettingsReq{Ctx: 86016, KV: "int8", Spec: 3, Vision: true})
	if err != nil {
		t.Fatal("vision HIP refusee a la reconfiguration :", err)
	}
	built, err := moeStrataBuildConfig(base, parseEnv(out), "")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := built["vision"]; !ok {
		t.Fatal("section vision perdue pour un preset HIP")
	}
	if !hasArg(moeArgs(t, built), "--vision") {
		t.Fatal("--vision absent des arguments HIP")
	}
}
func TestStrataPreflightChecksPinnedRuntime(t *testing.T) {
	c := strataFixture(t, "cuda")
	_, base, err := moeStrataPreflight(c)
	if err != nil {
		t.Fatal("CUDA13 metadata without toolkit field rejected", err)
	}
	exe := base["exe"].(string)
	if err = strataWriteJSON(filepath.Join(filepath.Dir(exe), "BUILD.json"), map[string]any{"version": strataVersion, "cuda": "12.9", "toolkit": 12}); err != nil {
		t.Fatal(err)
	}
	if _, _, err = moeStrataPreflight(c); err == nil {
		t.Fatal("wrong CUDA runtime accepted")
	}
	c["MOE_BACKEND"] = "hip"
	if _, _, err = moeStrataPreflight(c); err == nil {
		t.Fatal("CUDA used as HIP")
	}
	c["MOE_CONFIG"] = filepath.Join(t.TempDir(), "foreign.json")
	if _, _, err = moeStrataPreflight(c); err == nil {
		t.Fatal("foreign config accepted")
	}
}
func TestMoeProbeRequiresGeneratedResponseAndValidToolArguments(t *testing.T) {
	for _, mode := range []string{"success", "wrong-text", "wrong-arguments", "no-tool", "http-error"} {
		t.Run(mode, func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Authorization") != "Bearer test-key" {
					t.Error("authentication missing")
				}
				if r.URL.Path == "/health" {
					w.Write([]byte(`{"status":"ok"}`))
					return
				}
				if r.URL.Path == "/v1/models" {
					w.Write([]byte(`{"data":[{"id":"fixture"}]}`))
					return
				}
				calls++
				var payload map[string]any
				json.NewDecoder(r.Body).Decode(&payload)
				if mode == "http-error" {
					w.WriteHeader(503)
					return
				}
				messages := payload["messages"].([]any)
				text := messages[0].(map[string]any)["content"].(string)
				token := text[strings.Index(text, "AJEAN_"):]
				token = strings.Split(token, ".")[0]
				if calls == 1 {
					if mode == "wrong-text" {
						token = "wrong"
					}
					json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": token}}}})
					return
				}
				if mode == "wrong-arguments" {
					token = "wrong"
				}
				args, _ := json.Marshal(map[string]string{"token": token})
				tools := []any{map[string]any{"function": map[string]string{"name": "ajean_probe", "arguments": string(args)}}}
				if mode == "no-tool" {
					tools = nil
				}
				json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]any{"tool_calls": tools}}}})
			}))
			defer server.Close()
			result := probeMoe(context.Background(), server.Client(), server.URL, "test-key")
			if result.OK != (mode == "success") || !result.Health {
				t.Fatal(result)
			}
			if mode != "success" && result.Error == "" {
				t.Fatal("failure without explanation")
			}
		})
	}
}

func TestStrataZeroReserveAndMissingCUDAVision(t *testing.T) {
	c := strataFixture(t, "cuda")
	c["MOE_VRAM_RESERVE"] = "0"
	details := moeStrataDetails(c)
	b, err := json.Marshal(details)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"vram_reserve_mib":0`) {
		t.Fatal("explicit zero absent from API", string(b))
	}
	if _, err = moeStrataSettings(formatEnv(c), moeSettingsReq{Ctx: 86016, KV: "int8", Spec: 3, Vision: true}); err == nil {
		t.Fatal("vision enabled without prepared assets")
	}
	if details.VisionAvailable {
		t.Fatal("missing encoder advertised as available")
	}
}
