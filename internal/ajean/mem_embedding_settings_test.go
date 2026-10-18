package ajean

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMemoryGlobalModeAndPresetPriority(t *testing.T) {
	parallelTestHome(t)
	c := embeddingConfig{Enabled: false, Port: 18081, Threads: 8, Dimensions: 256}
	if err := saveEmbeddingConfig(c); err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	handleEmbeddingMode(w, httptest.NewRequest("POST", "/api/embeddings/mode", strings.NewReader(`{"mode":"hybrid"}`)))
	if w.Code != 200 || !readEmbeddingDefaults().Enabled || !readEmbeddingConfig().Enabled {
		t.Fatal(w.Body.String())
	}
	if err := WriteConfig(map[string]string{"MEMORY_SYSTEM": "classic", "MEMORY_EMBED_DIMENSIONS": "128"}); err != nil {
		t.Fatal(err)
	}
	if readEmbeddingConfig().Enabled || !readEmbeddingDefaults().Enabled || readEmbeddingConfig().Dimensions != 128 || readEmbeddingDefaults().Dimensions != 256 {
		t.Fatal("preset/global state mixed")
	}
	if err := WriteConfig(map[string]string{}); err != nil {
		t.Fatal(err)
	}
	if !readEmbeddingConfig().Enabled || readEmbeddingConfig().Dimensions != 256 {
		t.Fatal("defaults lost")
	}
	w = httptest.NewRecorder()
	handleEmbeddingMode(w, httptest.NewRequest("POST", "/api/embeddings/mode", strings.NewReader(`{"mode":"classic"}`)))
	if w.Code != 200 || readEmbeddingConfig().Enabled {
		t.Fatal(w.Body.String())
	}
	if readEmbeddingDefaults().Dimensions != 256 {
		t.Fatal("mode toggle erased encoder defaults")
	}
}

func TestEmbeddingTruncationRenormalizesEveryVector(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		v := make([]float32, 768)
		v[0] = 3
		v[1] = 4
		v[500] = 12
		json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{{"index": 0, "embedding": v}, {"index": 1, "embedding": v}}})
	}))
	defer server.Close()
	client := embeddingHTTPClient{URL: server.URL, HTTP: server.Client(), Dimensions: 128}
	rows, err := client.embed(context.Background(), []string{"a", "b"})
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range rows {
		if len(v) != 128 || v[0] < .599 || v[0] > .601 || v[1] < .799 || v[1] > .801 {
			t.Fatal("truncation/norm", v)
		}
	}
}

func TestEmbeddingDeviceAndModalitiesValidation(t *testing.T) {
	parallelTestHome(t)
	base := embeddingConfig{Device: "gpu", GPUDevice: "ROCm0", Threads: 8, Port: 18081}
	if err := validateEmbeddingConfig(base); err != nil {
		t.Fatal(err)
	}
	args := strings.Join(embeddingArgs(base, "secret"), " ")
	if !strings.Contains(args, "--device ROCm0 --n-gpu-layers 999") {
		t.Fatal(args)
	}
	base.Modalities = "text,image"
	if validateEmbeddingConfig(base) == nil {
		t.Fatal("GGUF accepted unsupported media")
	}
	base.Provider = "transformers"
	base.GPUDevice = "cuda:0"
	if err := validateEmbeddingConfig(base); err != nil {
		t.Fatal(err)
	}
	if _, err := SavePreset("", "bad", "MEMORY_EMBED_MODALITIES=image\n"); err == nil {
		t.Fatal("text-less preset accepted")
	}
}

func TestEmbeddingMediaContainmentAndContentInvalidation(t *testing.T) {
	home := testHome(t)
	root := filepath.Join(home, "memory")
	os.MkdirAll(root, 0700)
	cfg := normalizeEmbeddingConfig(embeddingConfig{Provider: "transformers", Modalities: "text,image"})
	media := filepath.Join(root, "capture.png")
	os.WriteFile(media, []byte("old"), 0600)
	note := "# Capture\n![vue](capture.png)\n"
	os.WriteFile(filepath.Join(root, "note.md"), []byte(note), 0600)
	before, err := loadMemoryDocumentsFor(root, cfg)
	if err != nil {
		t.Fatal(err)
	}
	input, _, err := embeddingMedia(root, note, cfg)
	if err != nil || len(input.Image) != 1 || !strings.Contains(input.Text, "<|image|>") {
		t.Fatal(input, err)
	}
	os.WriteFile(media, []byte("new"), 0600)
	after, err := loadMemoryDocumentsFor(root, cfg)
	if err != nil || before[0].Hash == after[0].Hash || chunkMemory(before)[0].Key == chunkMemory(after)[0].Key {
		t.Fatal("media edit not invalidated", err)
	}
	if _, _, err := embeddingMedia(root, "![escape](../outside.png)", cfg); err == nil {
		t.Fatal("path escape accepted")
	}
	ext := filepath.Join(t.TempDir(), "outside.png")
	os.WriteFile(ext, []byte("private"), 0600)
	if err := os.Symlink(ext, filepath.Join(root, "link.png")); err == nil {
		if _, _, err := embeddingMedia(root, "![link](link.png)", cfg); err == nil {
			t.Fatal("symlink escape accepted")
		}
	}
	_, hash, err := embeddingMedia(root, "![remote](https://example.org/a.png)", cfg)
	if err != nil || hash != "" {
		t.Fatal("network media fetched")
	}
	// Multimodal inputs travel only to the explicit full-model worker.
	if _, err := (&embeddingHTTPClient{}).embedInput(context.Background(), input); err == nil {
		t.Fatal("GGUF accepted multimodal input")
	}
}
