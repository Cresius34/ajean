package ajean

import (
	"context"
	"image"
	"image/color"
	"image/png"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

// Explicit opt-in: no network or Python/model installation during go test.
func TestEmbeddingRealMultimodalWorker(t *testing.T) {
	bin, model := os.Getenv("AJEAN_TEST_EMBED_PYTHON"), os.Getenv("AJEAN_TEST_EMBED_FULL_MODEL")
	if bin == "" || model == "" {
		t.Skip("set AJEAN_TEST_EMBED_PYTHON and AJEAN_TEST_EMBED_FULL_MODEL")
	}
	testHome(t)
	root := memoryDir()
	os.MkdirAll(root, 0700)
	img := image.NewRGBA(image.Rect(0, 0, 128, 128))
	for y := 0; y < 128; y++ {
		for x := 0; x < 128; x++ {
			img.Set(x, y, color.RGBA{220, 30, 30, 255})
		}
	}
	f, _ := os.Create(filepath.Join(root, "rouge.png"))
	png.Encode(f, img)
	f.Close()
	os.WriteFile(filepath.Join(root, "visuel.md"), []byte("# Échantillon rouge\n![surface rouge](rouge.png)\n"), 0600)
	os.WriteFile(filepath.Join(root, "physique.md"), []byte("# Ancrage Chaos\nLes ancrages maintiennent la base immobile pendant la fracture.\n"), 0600)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()
	cfg := normalizeEmbeddingConfig(embeddingConfig{Enabled: true, Bin: bin, Model: model, Provider: "transformers", Device: "cpu", Modalities: "text,image", Dimensions: 256, Port: port, Threads: 4})
	if err := saveEmbeddingConfig(cfg); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(stopEmbeddingServer)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	embeddingCache.Lock()
	epoch := embeddingCache.Epoch
	embeddingCache.Unlock()
	idx, err := rebuildEmbeddingIndex(ctx, root, cfg, epoch)
	if err != nil {
		t.Fatal(err)
	}
	if len(idx.Vectors) < 2 {
		t.Fatal("index absent")
	}
	for _, v := range idx.Vectors {
		if len(v) != 256 {
			t.Fatal("dimensions ignored")
		}
	}
	result := SearchMemory(ctx, "Comment fixer la base pendant une rupture ?", 3)
	if result.Mode != "hybrid" || len(result.Hits) == 0 {
		t.Fatalf("%+v", result)
	}
	if err := os.Remove(filepath.Join(root, "rouge.png")); err != nil {
		t.Fatal(err)
	}
	if result = SearchMemory(ctx, "surface rouge", 3); result.Mode != "lexical" {
		t.Fatal("deleted media not invalidated")
	}
	t.Log("Real multimodal HTTP worker, media index, 256d query and deletion invalidation passed on port " + strconv.Itoa(port))
}
