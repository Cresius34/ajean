package ajean

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExl3ParseHF(t *testing.T) {
	cases := []struct{ in, repo, rev string }{
		{"https://huggingface.co/turboderp/Qwen3.8-27B-exl3/tree/SC_4.00bpw_H5_V6", "turboderp/Qwen3.8-27B-exl3", "SC_4.00bpw_H5_V6"},
		{"https://huggingface.co/turboderp/Qwen3.8-27B-exl3", "turboderp/Qwen3.8-27B-exl3", "main"},
		{"https://huggingface.co/turboderp/Qwen3.8-27B-exl3/tree/3.50bpw/", "turboderp/Qwen3.8-27B-exl3", "3.50bpw"},
		{"turboderp/Qwen3.8-27B-exl3:4.0bpw", "turboderp/Qwen3.8-27B-exl3", "4.0bpw"},
		{"turboderp/Qwen3.8-27B-exl3", "turboderp/Qwen3.8-27B-exl3", "main"},
	}
	for _, c := range cases {
		repo, rev, err := exl3ParseHF(c.in)
		if err != nil || repo != c.repo || rev != c.rev {
			t.Errorf("%s → %q %q %v, attendu %q %q", c.in, repo, rev, err, c.repo, c.rev)
		}
	}
	if _, _, err := exl3ParseHF("https://example.com/x/y"); err == nil {
		t.Error("lien hors Hugging Face accepté")
	}
	if n := exl3DefaultName("turboderp/Qwen3.8-27B-exl3", "SC_4.00bpw_H5_V6"); n != "Qwen3.8-27B-exl3-SC_4.00bpw_H5_V6" {
		t.Errorf("nom par défaut : %s", n)
	}
}

func TestExl3CudaFlavor(t *testing.T) {
	for drv, want := range map[string]string{"595.71.05": "cu13", "580.10": "cu13", "575.51": "cu12", "560.1": "", "": ""} {
		if got := exl3CudaFlavor(drv); got != want {
			t.Errorf("%s → %q, attendu %q", drv, got, want)
		}
	}
}

func TestExl3BuildConfig(t *testing.T) {
	cfg := map[string]string{
		"ENGINE": "exl3", "CTX": "131072", "EXL3_CACHE_MODE": "8,8", "EXL3_TP": "1",
		"EXL3_GPU_SPLIT": "5.5, 11", "EXL3_DRAFT": "mtp", "EXL3_DRAFT_TOKENS": "3",
		"EXL3_CHUNK": "4096", "EXL3_VISION": "1", "EXL3_VISION_DEVICE": "cuda:1",
		"REASONING": "off", "PORT": "8080",
	}
	y := exl3BuildConfig(cfg, filepath.Join("/m", "Qwen3.8-27B-SC4.0-V"), "k")
	for _, want := range []string{
		"max_seq_len: 131072", "cache_size: 131072", `cache_mode: "8,8"`, "tensor_parallel: true",
		"gpu_split_auto: false", "gpu_split: [5.5, 11]", "draft_mode: mtp", "draft_num_tokens: 3",
		"chunk_size: 4096", "vision: true", "vision_offload: false", "enable_thinking: false",
		`model_name: "Qwen3.8-27B-SC4.0-V"`, "disable_auth: false", "port: 8080",
	} {
		if !strings.Contains(y, want) {
			t.Errorf("config sans %q :\n%s", want, y)
		}
	}
	// valeurs par défaut : pas de découpage imposé, pas de brouillon, raisonnement libre
	y = exl3BuildConfig(map[string]string{"ENGINE": "exl3", "CTX": "1000"}, "/m/x", "")
	for _, want := range []string{"gpu_split_auto: true", "draft_mode: disabled", "disable_auth: true", "cache_size: 1024"} {
		if !strings.Contains(y, want) {
			t.Errorf("config par défaut sans %q :\n%s", want, y)
		}
	}
	if strings.Contains(y, "enable_thinking") {
		t.Error("raisonnement coupé sans REASONING=off")
	}
	// options : MTP adaptatif, recopie, backend NCCL
	y = exl3BuildConfig(map[string]string{"EXL3_DRAFT": "mtp", "EXL3_DRAFT_DYNAMIC": "1", "EXL3_DRAFT_CONFIDENCE": "0.3",
		"EXL3_HYBRID": "16", "EXL3_TP": "1", "EXL3_TP_BACKEND": "nccl"}, "/m/x", "")
	for _, want := range []string{"dynamic_draft: true", "draft_confidence: 0.3", "hybrid_draft_tokens: 16", "tensor_parallel_backend: nccl"} {
		if !strings.Contains(y, want) {
			t.Errorf("config sans %q :\n%s", want, y)
		}
	}
	if y := exl3BuildConfig(map[string]string{"EXL3_DRAFT_DYNAMIC": "1"}, "/m/x", ""); strings.Contains(y, "dynamic_draft") {
		t.Error("MTP adaptatif écrit sans décodage spéculatif")
	}
	// vision en RAM
	y = exl3BuildConfig(map[string]string{"EXL3_VISION": "1", "EXL3_VISION_DEVICE": "cpu"}, "/m/x", "")
	if !strings.Contains(y, "vision_offload: true") {
		t.Error("vision sur le CPU non traduite en vision_offload")
	}
	// mode de cache inconnu : repli sur 8,8
	y = exl3BuildConfig(map[string]string{"EXL3_CACHE_MODE": "n'importe quoi"}, "/m/x", "")
	if !strings.Contains(y, `cache_mode: "8,8"`) {
		t.Error("mode de cache inconnu accepté")
	}
}

func TestExl3ReadModel(t *testing.T) {
	dir := t.TempDir()
	m := filepath.Join(dir, "Qwen-exl3")
	_ = os.MkdirAll(m, 0o755)
	_ = os.WriteFile(filepath.Join(m, "config.json"), []byte(`{"quantization_config":{"quant_method":"exl3","bits":4.0},"vision_config":{"depth":27}}`), 0o644)
	if got, _ := exl3ReadModel(m); got.Vision {
		t.Error("vision annoncée sans ses poids")
	}
	_ = os.WriteFile(filepath.Join(m, "model.safetensors.index.json"), []byte(`{"weight_map":{"model.visual.blocks.0.attn.qkv.weight":"a"}}`), 0o644)
	got, ok := exl3ReadModel(m)
	if !ok || got.BPW != 4.0 || !got.Vision {
		t.Fatalf("modèle EXL3 mal lu : %+v %v", got, ok)
	}
	g := filepath.Join(dir, "gguf")
	_ = os.MkdirAll(g, 0o755)
	_ = os.WriteFile(filepath.Join(g, "config.json"), []byte(`{"architectures":["X"]}`), 0o644)
	if _, ok := exl3ReadModel(g); ok {
		t.Error("dossier non EXL3 reconnu comme EXL3")
	}
}

func TestExl3PatchVision(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "backends", "exllamav3")
	_ = os.MkdirAll(p, 0o755)
	src := "class X:\n    def load(self):\n" + exl3VisionOrig + "\n                pass\n"
	_ = os.WriteFile(filepath.Join(p, "model.py"), []byte(strings.ReplaceAll(src, "\n", "\r\n")), 0o644)
	if !exl3PatchVision(dir) {
		t.Fatal("patch non appliqué")
	}
	b, _ := os.ReadFile(filepath.Join(p, "model.py"))
	if !strings.Contains(string(b), "TABBY_VISION_DEVICE") || strings.Contains(string(b), "reserve_per_device=None if") && !strings.Contains(string(b), `"reserve_per_device": None`) {
		t.Errorf("patch incorrect :\n%s", b)
	}
	if !exl3PatchVision(dir) {
		t.Error("second passage : le patch déjà posé doit être reconnu")
	}
}
