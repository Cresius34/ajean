package ajean

import "testing"

// Le précompilé est conseillé partout, y compris Linux + CUDA depuis que
// llama.cpp publie des builds ubuntu-cuda.
func TestRecommendedMode(t *testing.T) {
	for _, b := range []string{"cuda", "rocm", "vulkan", "cpu", "metal"} {
		if got := recommendedMode(b); got["mode"] != "fast" {
			t.Errorf("backend %q : mode = %v, attendu fast", b, got["mode"])
		}
	}
}

// Noms d'assets réels de la release llama.cpp b11936.
func TestPickCudaLinuxAndWindows(t *testing.T) {
	names := []string{
		"cudart-llama-b11936-bin-ubuntu-cuda-12.8-x64.tar.gz",
		"cudart-llama-b11936-bin-ubuntu-cuda-13.4-x64.tar.gz",
		"cudart-llama-b11936-bin-ubuntu-cuda-13.4-arm64.tar.gz",
		"cudart-llama-bin-win-cuda-12.4-x64.zip",
		"cudart-llama-bin-win-cuda-13.4-x64.zip",
		"llama-b11936-bin-ubuntu-cuda-12.8-x64.tar.gz",
		"llama-b11936-bin-ubuntu-cuda-13.4-x64.tar.gz",
		"llama-b11936-bin-ubuntu-cuda-13.4-arm64.tar.gz",
		"llama-b11936-bin-win-cuda-12.4-x64.zip",
		"llama-b11936-bin-win-cuda-13.4-x64.zip",
		"llama-b11936-bin-win-cuda-13.4-arm64.zip",
	}
	var assets []ghAsset
	for _, n := range names {
		assets = append(assets, ghAsset{Name: n})
	}
	// Sans pilote détecté (machine de test) : la plus ancienne version.
	if driverCudaVersion() != 0 {
		t.Skip("pilote NVIDIA présent : choix dépendant de la machine")
	}
	cases := []struct{ os, arch, ext, main, cudart string }{
		{"ubuntu", "x64", ".tar.gz", "llama-b11936-bin-ubuntu-cuda-12.8-x64.tar.gz", "cudart-llama-b11936-bin-ubuntu-cuda-12.8-x64.tar.gz"},
		{"ubuntu", "arm64", ".tar.gz", "llama-b11936-bin-ubuntu-cuda-13.4-arm64.tar.gz", "cudart-llama-b11936-bin-ubuntu-cuda-13.4-arm64.tar.gz"},
		{"win", "x64", ".zip", "llama-b11936-bin-win-cuda-12.4-x64.zip", "cudart-llama-bin-win-cuda-12.4-x64.zip"},
	}
	for _, c := range cases {
		m, cr, _ := pickCuda(assets, c.os, c.arch, c.ext)
		if m == nil || m.Name != c.main || cr == nil || cr.Name != c.cudart {
			t.Errorf("%s/%s : main=%v cudart=%v", c.os, c.arch, m, cr)
		}
	}
}
