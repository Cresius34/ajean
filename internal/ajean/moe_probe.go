package ajean

// An explicit smoke test against the running engine. HTTP health alone does
// not demonstrate generation or OpenAI tool-call support. No tool is executed.
import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"time"
)

type moeProbeResult struct {
	OK       bool   `json:"ok"`
	Date     string `json:"date"`
	Version  string `json:"version"`
	Backend  string `json:"backend"`
	Context  string `json:"context"`
	Health   bool   `json:"health"`
	Response bool   `json:"response"`
	ToolCall bool   `json:"tool_call"`
	Error    string `json:"error,omitempty"`
	Report   string `json:"report,omitempty"`
}

func probeMoe(ctx context.Context, client *http.Client, baseURL, key string) moeProbeResult {
	result := moeProbeResult{Date: time.Now().UTC().Format(time.RFC3339), Version: strataVersion}
	request := func(method, path string, payload any, out any) error {
		var b []byte
		if payload != nil {
			b, _ = json.Marshal(payload)
		}
		req, err := http.NewRequestWithContext(ctx, method, baseURL+path, bytes.NewReader(b))
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", "application/json")
		if key != "" {
			req.Header.Set("Authorization", "Bearer "+key)
		}
		resp, err := client.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			return fmt.Errorf("%s : HTTP %d", path, resp.StatusCode)
		}
		if out == nil {
			return nil
		}
		return json.NewDecoder(io.LimitReader(resp.Body, 2<<20)).Decode(out)
	}
	if err := request("GET", "/health", nil, nil); err != nil {
		result.Error = err.Error()
		return result
	}
	result.Health = true
	var models struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := request("GET", "/v1/models", nil, &models); err != nil {
		result.Error = err.Error()
		return result
	}
	if len(models.Data) == 0 {
		result.Error = "aucun modèle chargé"
		return result
	}
	nonce := make([]byte, 8)
	if _, err := rand.Read(nonce); err != nil {
		result.Error = err.Error()
		return result
	}
	word := "AJEAN_" + hex.EncodeToString(nonce)
	var output struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
				Tools   []struct {
					Function struct {
						Name      string `json:"name"`
						Arguments string `json:"arguments"`
					} `json:"function"`
				} `json:"tool_calls"`
			} `json:"message"`
		} `json:"choices"`
	}
	payload := map[string]any{"model": models.Data[0].ID, "messages": []map[string]string{{"role": "user", "content": "Reply with exactly this string, without explanation: " + word}}, "stream": false, "temperature": 0, "max_tokens": 256, "chat_template_kwargs": map[string]bool{"enable_thinking": false}}
	if err := request("POST", "/v1/chat/completions", payload, &output); err != nil {
		result.Error = err.Error()
		return result
	}
	if len(output.Choices) == 0 || strings.TrimSpace(output.Choices[0].Message.Content) != word {
		result.Error = "réponse générée différente de la chaîne demandée"
		return result
	}
	result.Response = true
	payload["messages"] = []map[string]string{{"role": "user", "content": "Call ajean_probe with token set to " + word + ". This is a test; no real action is executed."}}
	payload["tools"] = []any{map[string]any{"type": "function", "function": map[string]any{"name": "ajean_probe", "description": "Return a diagnostic token. No side effects.", "parameters": map[string]any{"type": "object", "properties": map[string]any{"token": map[string]string{"type": "string"}}, "required": []string{"token"}}}}}
	payload["tool_choice"] = map[string]any{"type": "function", "function": map[string]string{"name": "ajean_probe"}}
	output.Choices = nil
	if err := request("POST", "/v1/chat/completions", payload, &output); err != nil {
		result.Error = err.Error()
		return result
	}
	if len(output.Choices) == 1 && len(output.Choices[0].Message.Tools) == 1 {
		call := output.Choices[0].Message.Tools[0].Function
		var args struct {
			Token string `json:"token"`
		}
		result.ToolCall = call.Name == "ajean_probe" && json.Unmarshal([]byte(call.Arguments), &args) == nil && args.Token == word
	}
	result.OK = result.Health && result.Response && result.ToolCall
	if !result.OK {
		result.Error = "appel d’outil ou arguments JSON incorrects"
	}
	return result
}

func handleMoeProbe(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		sendJSON(w, 405, map[string]any{"ok": false, "error": "POST attendu"})
		return
	}
	c := ReadConfig()
	if !isStrataMoe(c) || !parallelBuild() || !parallelEngineActive() {
		sendJSON(w, 409, map[string]any{"ok": false, "error": "activer et charger un preset MoE Windows dans ce TEST"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Minute)
	defer cancel()
	result := probeMoe(ctx, &http.Client{Timeout: 90 * time.Second}, "http://127.0.0.1:"+parallelLocalEnginePort(), readAPIKey())
	result.Backend, result.Context = c["MOE_BACKEND"], c["CTX"]
	path := filepath.Join(moeHome(), "proofs", "test-"+time.Now().UTC().Format("20060102-150405.000000000")+".json")
	if err := parallelWritablePath(path); err == nil {
		result.Report, _ = filepath.Rel(AjeanHome(), path)
		if err = strataWriteJSON(path, result); err != nil {
			result.OK = false
			result.Error = "rapport local non enregistré : " + err.Error()
		}
	} else {
		result.OK = false
		result.Error = err.Error()
	}
	sendJSON(w, 200, result)
}
