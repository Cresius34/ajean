package ajean

import (
	"errors"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

func normalizeEmbeddingConfig(c embeddingConfig) embeddingConfig {
	if c.Provider == "" {
		c.Provider = "llama_cpp"
	}
	if c.Device == "" {
		c.Device = "cpu"
	}
	if c.Dimensions == 0 {
		c.Dimensions = 768
	}
	if c.Modalities == "" {
		c.Modalities = "text"
	}
	return c
}

func embeddingPresetConfig(c embeddingConfig, cfg map[string]string) embeddingConfig {
	switch cfg["MEMORY_SYSTEM"] {
	case "classic":
		c.Enabled = false
	case "hybrid":
		c.Enabled = true
	}
	for key, dest := range map[string]*string{
		"PROVIDER": &c.Provider, "BIN": &c.Bin, "MODEL": &c.Model, "DEVICE": &c.Device, "GPU_DEVICE": &c.GPUDevice, "MODALITIES": &c.Modalities,
	} {
		if v := cfg["MEMORY_EMBED_"+key]; v != "" {
			*dest = v
		}
	}
	for key, dest := range map[string]*string{"BIN": &c.Bin, "MODEL": &c.Model} {
		if cfg["MEMORY_EMBED_"+key] != "" && !filepath.IsAbs(*dest) {
			if key == "MODEL" && !strings.ContainsAny(*dest, `/\`) {
				*dest = filepath.Join(modelsDir(), *dest)
			} else {
				*dest = filepath.Join(AjeanHome(), *dest)
			}
		}
	}
	if v, err := strconv.Atoi(cfg["MEMORY_EMBED_THREADS"]); err == nil {
		c.Threads = v
	}
	if v, err := strconv.Atoi(cfg["MEMORY_EMBED_DIMENSIONS"]); err == nil {
		c.Dimensions = v
	}
	return normalizeEmbeddingConfig(c)
}

func validateEmbeddingPreset(cfg map[string]string) error {
	switch cfg["MEMORY_SYSTEM"] {
	case "", "classic", "hybrid":
	default:
		return errors.New("système mémoire inconnu")
	}
	return validateEmbeddingOptions(embeddingPresetConfig(readEmbeddingDefaults(), cfg))
}

func validateEmbeddingOptions(c embeddingConfig) error {
	if c.Provider != "llama_cpp" && c.Provider != "transformers" {
		return errors.New("moteur mémoire inconnu")
	}
	if c.Device != "cpu" && c.Device != "gpu" {
		return errors.New("choisir CPU ou GPU pour la mémoire")
	}
	switch c.Dimensions {
	case 128, 256, 512, 768:
	default:
		return errors.New("dimensions mémoire : 128, 256, 512 ou 768")
	}
	seen := map[string]bool{}
	for _, m := range strings.Split(c.Modalities, ",") {
		if m != "text" && m != "image" && m != "audio" && m != "video" {
			return errors.New("modalité mémoire inconnue")
		}
		if seen[m] {
			return errors.New("modalité dupliquée")
		}
		seen[m] = true
	}
	if !seen["text"] {
		return errors.New("le texte est nécessaire aux requêtes mémoire")
	}
	if c.Provider == "llama_cpp" && c.Modalities != "text" {
		return errors.New("les modalités image/audio/vidéo nécessitent le moteur complet Transformers")
	}
	if c.Device == "gpu" && c.GPUDevice == "" {
		return errors.New("renseigner le périphérique GPU (ROCm0, Vulkan0 ou cuda:0 selon le moteur)")
	}
	if c.GPUDevice != "" && (!regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_.:-]*$`).MatchString(c.GPUDevice) || (c.Device == "gpu" && (c.GPUDevice == "none" || c.GPUDevice == "cpu"))) {
		return errors.New("identifiant de périphérique GPU invalide")
	}
	if c.Provider == "transformers" && c.Device == "gpu" && !regexp.MustCompile(`^cuda:[0-9]+$`).MatchString(c.GPUDevice) {
		return errors.New("PyTorch : utiliser cuda:N, aussi sous ROCm")
	}
	return nil
}
