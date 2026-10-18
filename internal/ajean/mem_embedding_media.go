package ajean

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type embeddingInput struct {
	Text  string   `json:"text"`
	Image []string `json:"image,omitempty"`
	Audio string   `json:"audio,omitempty"`
	Video string   `json:"video,omitempty"`
}

var memoryMediaLink = regexp.MustCompile(`!?\[[^\]\r\n]*\]\(([^)\r\n]+)\)`)

// Only media referenced by a canonical Markdown note is indexed. Results still
// point to its note/lines for mem_read. No network fetch, recursive inventory,
// external path or symlink escape; vault encryption never indexes clear media.
func embeddingMedia(root, text string, cfg embeddingConfig) (embeddingInput, string, error) {
	input := embeddingInput{Text: text}
	if cfg.Provider != "transformers" || cfg.Modalities == "text" {
		return input, "", nil
	}
	active := "," + cfg.Modalities + ","
	hash := sha256.New()
	seen := map[string]bool{}
	for _, match := range memoryMediaLink.FindAllStringSubmatch(text, -1) {
		path := strings.TrimSpace(match[1])
		if strings.HasPrefix(path, "<") {
			if end := strings.Index(path, ">"); end > 0 {
				path = path[1:end]
			}
		} else {
			path = strings.SplitN(path, ` "`, 2)[0]
		}
		path, err := url.PathUnescape(path)
		if err != nil {
			return input, "", err
		}
		u, err := url.Parse(path)
		if err != nil || u.Scheme != "" || u.Host != "" {
			continue
		}
		modality := ""
		switch strings.ToLower(filepath.Ext(path)) {
		case ".png", ".jpg", ".jpeg", ".webp", ".bmp":
			modality = "image"
		case ".wav", ".flac", ".mp3", ".ogg":
			modality = "audio"
		case ".mp4", ".webm", ".mov":
			modality = "video"
		}
		if modality == "" || !strings.Contains(active, ","+modality+",") {
			continue
		}
		if memEncActive() {
			return input, "", errors.New("les médias sont désactivés dans une mémoire chiffrée ; utiliser Texte")
		}
		full := filepath.Join(root, filepath.FromSlash(path))
		if filepath.IsAbs(path) || !pathWithin(root, full) {
			return input, "", errors.New("média hors du projet mémoire")
		}
		real, err := filepath.EvalSymlinks(full)
		if err != nil {
			return input, "", fmt.Errorf("média local introuvable : %s", filepath.Base(path))
		}
		if !pathWithin(root, real) {
			return input, "", errors.New("lien média hors du projet mémoire")
		}
		if seen[real] {
			continue
		}
		seen[real] = true
		file, err := os.Open(real)
		if err != nil {
			return input, "", err
		}
		fi, err := file.Stat()
		if err != nil || !fi.Mode().IsRegular() || fi.Size() > 64<<20 {
			file.Close()
			return input, "", errors.New("média non régulier ou supérieur à 64 Mio")
		}
		fmt.Fprintln(hash, path)
		_, err = io.Copy(hash, file)
		file.Close()
		if err != nil {
			return input, "", err
		}
		switch modality {
		case "image":
			if len(input.Image) >= 4 {
				return input, "", errors.New("quatre images au maximum par bloc")
			}
			input.Image = append(input.Image, real)
		case "audio":
			if input.Audio != "" {
				return input, "", errors.New("un audio au maximum par bloc")
			}
			input.Audio = real
		case "video":
			if input.Video != "" {
				return input, "", errors.New("une vidéo au maximum par bloc")
			}
			input.Video = real
		}
		input.Text += " <|" + modality + "|>"
	}
	if len(seen) == 0 {
		return input, "", nil
	}
	return input, hex.EncodeToString(hash.Sum(nil)), nil
}

func (c *embeddingHTTPClient) embedInput(ctx context.Context, input embeddingInput) ([][]float32, error) {
	if len(input.Image) == 0 && input.Audio == "" && input.Video == "" {
		return c.embed(ctx, []string{input.Text})
	}
	if c.Provider != "transformers" {
		return nil, errors.New("moteur mémoire multimodal requis")
	}
	return c.embedRequest(ctx, []embeddingInput{input}, 1)
}
