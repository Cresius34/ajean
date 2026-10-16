package ajean

import (
	_ "embed"
	"os"
	"path/filepath"
	"strconv"
)

//go:embed embedding_server.py
var embeddingTransformerServer []byte

func embeddingTransformerArgs(c embeddingConfig, key string) ([]string, error) {
	dir := filepath.Join(AjeanHome(), "runtime", "memory-"+memoryDigest(string(embeddingTransformerServer))[:12])
	if parallelBuild() {
		if err := parallelWritablePath(dir); err != nil {
			return nil, err
		}
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	script := filepath.Join(dir, "embedding_server.py")
	if err := os.WriteFile(script, embeddingTransformerServer, 0600); err != nil {
		return nil, err
	}
	return []string{script, "--model", c.Model, "--root", AjeanHome(), "--device", c.Device, "--gpu-device", c.GPUDevice, "--modalities", c.Modalities, "--port", strconv.Itoa(c.Port), "--threads", strconv.Itoa(c.Threads), "--api-key", key}, nil
}

func embeddingDefaultPaths(provider string) (string, string) {
	if provider == "transformers" {
		return filepath.Join(AjeanHome(), "embedding-python", "python.exe"), filepath.Join(modelsDir(), "embeddinggemma-2-full")
	}
	suffix := ""
	if filepath.Ext(os.Args[0]) == ".exe" {
		suffix = ".exe"
	}
	return filepath.Join(filepath.Dir(AjeanHome()), "encoder", "llama-server"+suffix), filepath.Join(modelsDir(), "embeddinggemma-2-Q8_0.gguf")
}
