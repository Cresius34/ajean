package ajean

// Independent memory encoder. Global defaults live in state; a preset may
// override them without reconfiguring the conversation engine or its mmproj.
import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

const embeddingProtocol = "embeddinggemma2-search-v1/chunks-1800-rrf60"

type embeddingConfig struct {
	Enabled    bool   `json:"enabled"`
	Bin        string `json:"bin"`
	Model      string `json:"model"`
	Port       int    `json:"port"`
	Threads    int    `json:"threads"`
	Provider   string `json:"provider"`
	Device     string `json:"device"`
	GPUDevice  string `json:"gpu_device"`
	Modalities string `json:"modalities"`
	Dimensions int    `json:"dimensions"`
}

func readEmbeddingDefaults() embeddingConfig {
	c := embeddingConfig{Port: 18081, Threads: 8}
	getJSON(bkState, "memory_embeddings", &c)
	if parallelBuild() {
		root := filepath.Dir(AjeanHome())
		if filepath.IsAbs(c.Bin) && filepath.Base(filepath.Dir(c.Bin)) == "encoder" {
			c.Bin = filepath.Join("encoder", filepath.Base(c.Bin))
		}
		if filepath.IsAbs(c.Model) && filepath.Base(c.Model) == "embeddinggemma-2-Q8_0.gguf" {
			c.Model = filepath.Join("data", "models", filepath.Base(c.Model))
		}
		if c.Bin != "" && !filepath.IsAbs(c.Bin) {
			c.Bin = filepath.Join(root, c.Bin)
		}
		if c.Model != "" && !filepath.IsAbs(c.Model) {
			c.Model = filepath.Join(root, c.Model)
		}
	}
	return normalizeEmbeddingConfig(c)
}

func readEmbeddingConfig() embeddingConfig {
	return embeddingPresetConfig(readEmbeddingDefaults(), ReadConfig())
}

func saveEmbeddingConfig(c embeddingConfig) error {
	c = normalizeEmbeddingConfig(c)
	if parallelBuild() {
		root := filepath.Dir(AjeanHome())
		paths := []*string{&c.Bin, &c.Model}
		for _, p := range paths {
			if *p == "" {
				continue
			}
			if !filepath.IsAbs(*p) {
				*p = filepath.Join(root, *p)
			}
			if !pathWithin(root, *p) {
				return fmt.Errorf("copier l’encodeur et son modèle dans le dossier portable : %s", *p)
			}
			if err := parallelPackagePath(*p); err != nil {
				return err
			}
			rel, err := filepath.Rel(root, *p)
			if err != nil {
				return err
			}
			*p = rel
		}
	}
	return putJSON(bkState, "memory_embeddings", c)
}

func validateEmbeddingConfig(c embeddingConfig) error {
	c = normalizeEmbeddingConfig(c)
	if c.Port < 1024 || c.Port > 65535 || c.Port == 8080 || c.Port == 8090 || c.Port == 18080 || c.Port == 18090 {
		return errors.New("port encodeur invalide ou réservé (utiliser 18081)")
	}
	if c.Threads < 1 || c.Threads > 64 {
		return errors.New("threads CPU : 1 à 64")
	}
	if err := validateEmbeddingOptions(c); err != nil {
		return err
	}
	if !c.Enabled {
		return nil
	}
	for i, p := range []string{c.Bin, c.Model} {
		if !filepath.IsAbs(p) {
			return errors.New("choisir un chemin absolu pour le moteur et le modèle")
		}
		fi, err := os.Stat(p)
		wantDirectory := i == 1 && c.Provider == "transformers"
		if err != nil || (wantDirectory && !fi.IsDir()) || (!wantDirectory && !fi.Mode().IsRegular()) {
			return fmt.Errorf("fichier introuvable : %s", p)
		}
	}
	if c.Provider == "transformers" {
		if _, err := os.Stat(filepath.Join(c.Model, "config.json")); err != nil {
			return errors.New("modèle complet absent : lancer Installer-Multimodal.cmd")
		}
		return nil
	}
	if !strings.EqualFold(filepath.Ext(c.Model), ".gguf") {
		return errors.New("le modèle doit être un fichier GGUF")
	}
	return nil
}

// The fingerprint changes when either executable or weights change. It also
// versions the task prefixes and chunker. Vectors are only cached in RAM.
func embeddingFingerprint(c embeddingConfig) (string, error) {
	c = normalizeEmbeddingConfig(c)
	if err := validateEmbeddingConfig(c); err != nil {
		return "", err
	}
	h := sha256.New()
	fmt.Fprintln(h, embeddingProtocol, c.Port, c.Threads, c.Provider, c.Device, c.GPUDevice, c.Modalities, c.Dimensions)
	for _, p := range []string{c.Bin, c.Model} {
		fi, err := os.Stat(p)
		if err != nil {
			return "", err
		}
		fmt.Fprintln(h, filepath.Clean(p), fi.Size(), fi.ModTime().UnixNano())
		if fi.IsDir() {
			err := filepath.WalkDir(p, func(path string, entry os.DirEntry, walkErr error) error {
				if walkErr != nil {
					return walkErr
				}
				if entry.IsDir() {
					if strings.HasPrefix(entry.Name(), ".") {
						return filepath.SkipDir
					}
					return nil
				}
				info, err := entry.Info()
				if err != nil {
					return err
				}
				fmt.Fprintln(h, path, info.Size(), info.ModTime().UnixNano())
				return nil
			})
			if err != nil {
				return "", err
			}
		}
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

type embeddingHTTPClient struct {
	URL, Key, Fingerprint string
	HTTP                  *http.Client
	Dimensions            int
	Provider              string
}

func (c *embeddingHTTPClient) embed(ctx context.Context, input []string) ([][]float32, error) {
	if len(input) == 0 {
		return nil, nil
	}
	return c.embedRequest(ctx, input, len(input))
}

func (c *embeddingHTTPClient) embedRequest(ctx context.Context, input any, count int) ([][]float32, error) {
	body, err := json.Marshal(map[string]any{"input": input, "model": "ajean-memory", "encoding_format": "float"})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, "POST", c.URL+"/v1/embeddings", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.Key)
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		// Do not echo the request, response or source text into logs.
		return nil, fmt.Errorf("encodeur HTTP %d (consulter memory-encoder.log)", resp.StatusCode)
	}
	var result struct {
		Data []struct {
			Index     int       `json:"index"`
			Embedding []float32 `json:"embedding"`
		} `json:"data"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 8<<20)).Decode(&result); err != nil {
		return nil, err
	}
	if len(result.Data) != count {
		return nil, errors.New("nombre de vecteurs inattendu")
	}
	out := make([][]float32, count)
	dim := 0
	for _, item := range result.Data {
		if item.Index < 0 || item.Index >= len(out) || out[item.Index] != nil {
			return nil, errors.New("index de vecteur invalide")
		}
		v := item.Embedding
		// EmbeddingGemma 2 text encoder: 768 dimensions. Refuse another model
		// silently selected by mistake, NaNs and truncated responses.
		if len(v) != 768 || (dim != 0 && len(v) != dim) {
			return nil, errors.New("dimension inattendue (768 attendues pour EmbeddingGemma 2)")
		}
		if c.Dimensions > 0 && c.Dimensions < 768 {
			v = v[:c.Dimensions]
		}
		dim = 768
		norm := 0.0
		for _, x := range v {
			if math.IsNaN(float64(x)) || math.IsInf(float64(x), 0) {
				return nil, errors.New("vecteur non fini")
			}
			norm += float64(x) * float64(x)
		}
		if norm <= 0 {
			return nil, errors.New("vecteur nul")
		}
		norm = math.Sqrt(norm)
		for i := range v {
			v[i] = float32(float64(v[i]) / norm)
		}
		out[item.Index] = v
	}
	return out, nil
}

type embeddingProcess struct {
	cmd    *exec.Cmd
	done   chan struct{}
	err    error // published by closing done
	client *embeddingHTTPClient
	ready  bool // protected by embeddingServer.Mutex
}

var embeddingServer struct {
	sync.Mutex
	p         *embeddingProcess
	lastError string
}

func embeddingArgs(c embeddingConfig, key string) []string {
	c = normalizeEmbeddingConfig(c)
	device, layers := "none", "0"
	if c.Device == "gpu" {
		device, layers = c.GPUDevice, "999"
	}
	return []string{"--model", c.Model, "--alias", "ajean-memory", "--embeddings", "--pooling", "mean",
		"--device", device, "--n-gpu-layers", layers, "--host", "127.0.0.1", "--port", strconv.Itoa(c.Port),
		"--threads", strconv.Itoa(c.Threads), "--threads-batch", strconv.Itoa(c.Threads),
		"--ctx-size", "8192", "--batch-size", "8192", "--ubatch-size", "8192", "--parallel", "1", "--api-key", key}
}

func startEmbeddingServer(ctx context.Context, c embeddingConfig) (*embeddingHTTPClient, error) {
	c = normalizeEmbeddingConfig(c)
	if !c.Enabled {
		return nil, errors.New("recherche sémantique désactivée")
	}
	fp, err := embeddingFingerprint(c)
	if err != nil {
		return nil, err
	}
	embeddingServer.Lock()
	p := embeddingServer.p
	if p != nil {
		select {
		case <-p.done:
			embeddingServer.p = nil
			p = nil
		default:
		}
	}
	if p != nil && p.client.Fingerprint != fp {
		killProcessTree(p.cmd)
		embeddingServer.p = nil
		embeddingServer.Unlock()
		select {
		case <-p.done:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		return startEmbeddingServer(ctx, c)
	}
	if p == nil {
		if readEmbeddingConfig() != c {
			embeddingServer.Unlock()
			return nil, errors.New("configuration encodeur modifiée")
		}
		// Never adopt, terminate or send memory to an unrelated process on a
		// busy port. Every child also gets its own ephemeral API key.
		// A previous owned child may still be releasing its socket during an
		// immediate app restart. Wait briefly without adopting/killing it.
		if e := waitPortFree("127.0.0.1", strconv.Itoa(c.Port), 5*time.Second); e != nil {
			embeddingServer.Unlock()
			return nil, fmt.Errorf("port encodeur %d occupé ; aucun processus arrêté", c.Port)
		}
		ln, e := net.Listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(c.Port)))
		if e != nil {
			embeddingServer.Unlock()
			return nil, fmt.Errorf("port encodeur %d occupé ; aucun processus arrêté", c.Port)
		}
		ln.Close()
		secret := make([]byte, 32)
		if _, e = rand.Read(secret); e != nil {
			embeddingServer.Unlock()
			return nil, e
		}
		key := hex.EncodeToString(secret)
		log, e := os.OpenFile(filepath.Join(AjeanHome(), "memory-encoder.log"), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
		if e != nil {
			embeddingServer.Unlock()
			return nil, e
		}
		args := embeddingArgs(c, key)
		if c.Provider == "transformers" {
			args, e = embeddingTransformerArgs(c, key)
			if e != nil {
				log.Close()
				embeddingServer.Unlock()
				return nil, e
			}
		}
		cmd := hideCmd(exec.Command(c.Bin, args...))
		cmd.Dir = filepath.Dir(c.Bin)
		cmd.Env = libraryPathEnv(cmd.Dir)
		cmd.Stdout, cmd.Stderr = log, log
		prepareBoundChild(cmd)
		if e = cmd.Start(); e != nil {
			log.Close()
			embeddingServer.Unlock()
			return nil, e
		}
		bindChild(cmd)
		p = &embeddingProcess{cmd: cmd, done: make(chan struct{}), client: &embeddingHTTPClient{
			URL: fmt.Sprintf("http://127.0.0.1:%d", c.Port), Key: key, Fingerprint: fp, HTTP: &http.Client{Timeout: 30 * time.Second}, Dimensions: c.Dimensions, Provider: c.Provider,
		}}
		embeddingServer.p = p
		go func(p *embeddingProcess) { p.err = p.cmd.Wait(); log.Close(); close(p.done) }(p)
	}
	if p.ready {
		embeddingServer.Unlock()
		return p.client, nil
	}
	embeddingServer.Unlock()
	probe := &http.Client{Timeout: 700 * time.Millisecond}
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	for {
		req, _ := http.NewRequestWithContext(ctx, "GET", p.client.URL+"/health", nil)
		req.Header.Set("Authorization", "Bearer "+p.client.Key)
		if resp, e := probe.Do(req); e == nil {
			resp.Body.Close()
			if resp.StatusCode == 200 {
				embeddingServer.Lock()
				if embeddingServer.p != p {
					embeddingServer.Unlock()
					return nil, errors.New("encodeur remplacé")
				}
				p.ready = true
				embeddingServer.lastError = ""
				embeddingServer.Unlock()
				return p.client, nil
			}
		}
		select {
		case <-p.done:
			return nil, fmt.Errorf("encodeur arrêté (%v) ; moteur compatible gemma-embedding2 requis, voir memory-encoder.log", p.err)
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
		}
	}
}

func stopEmbeddingServer() {
	embeddingServer.Lock()
	p := embeddingServer.p
	embeddingServer.p = nil
	embeddingServer.Unlock()
	if p != nil {
		select {
		case <-p.done:
		default:
			killProcessTree(p.cmd)
		}
		select {
		case <-p.done:
		case <-time.After(5 * time.Second):
		}
	}
	clearEmbeddingCache()
}

func readyEmbeddingClient(c embeddingConfig) (*embeddingHTTPClient, error) {
	fp, err := embeddingFingerprint(c)
	if err != nil {
		return nil, err
	}
	embeddingServer.Lock()
	defer embeddingServer.Unlock()
	p := embeddingServer.p
	if p == nil || !p.ready || p.client.Fingerprint != fp {
		return nil, errors.New("encodeur non démarré : ouvrir Mémoire sémantique")
	}
	select {
	case <-p.done:
		return nil, errors.New("encodeur arrêté : redémarrer dans Mémoire sémantique")
	default:
	}
	return p.client, nil
}
