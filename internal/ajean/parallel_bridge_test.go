package ajean

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

// Le pont Ajean↔Ajean prête un cerveau, pas la machine. Ces tests verrouillent
// ce qui ne doit jamais glisser : éteint par défaut, clé exigée, surface réduite
// à /v1/*, et des ports qui se suivent au lieu d'être gravés.

func freeTestPort(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skip("aucun port libre pour ce test")
	}
	port := strconv.Itoa(ln.Addr().(*net.TCPAddr).Port)
	ln.Close()
	return port
}

func TestParallelBridgeOffByDefaultAndRefusesWithoutKey(t *testing.T) {
	parallelTestHome(t)
	t.Setenv("AJEAN_BRIDGE", "")
	t.Cleanup(func() { _ = stopParallelBridge() })
	_ = writeAPIKey("")
	_ = setParallelBridgeRequested(false)
	if parallelBridgeEnabled() || parallelBridgeRequested() || parallelBridgeEnvForced() {
		t.Fatal("le pont doit être éteint par défaut")
	}
	if err := startParallelBridge(); err == nil || !strings.Contains(err.Error(), "désactivé") {
		t.Fatalf("pont ouvert sans demande : %v", err)
	}
	// Demandé mais sans clé machine : refuse aussi, fail-closed.
	if err := setParallelBridgeRequested(true); err != nil {
		t.Fatal(err)
	}
	if err := startParallelBridge(); err == nil || !strings.Contains(err.Error(), "clé API") {
		t.Fatalf("pont ouvert sans clé : %v", err)
	}
	if parallelBridgeRunning() {
		t.Fatal("un pont refusé ne doit rien écouter")
	}
}

func TestParallelBridgeSurfaceAndAuth(t *testing.T) {
	parallelTestHome(t)
	if err := writeAPIKey("sk-ajean-pont-test"); err != nil {
		t.Fatal(err)
	}
	h := parallelBridgeHandler()

	// Hors /v1/* : rien. Ni l'API de pilotage, ni les pages.
	for _, p := range []string{"/", "/api/memory", "/api/chat/send", "/semantic-memory", "/api/bridge"} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", "http://192.168.1.9"+p, nil))
		if w.Code != http.StatusNotFound {
			t.Fatalf("%s : %d, attendu 404", p, w.Code)
		}
	}

	// /v1/* sans clé, depuis le réseau : refusé.
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "http://192.168.1.9/v1/chat/completions", strings.NewReader(`{"model":"x"}`))
	req.RemoteAddr = "192.168.1.42:51515"
	h.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("sans clé depuis le réseau : %d, attendu 401\n%s", w.Code, w.Body.String())
	}

	// /v1/* avec la mauvaise clé : refusé.
	w = httptest.NewRecorder()
	req = httptest.NewRequest("POST", "http://192.168.1.9/v1/chat/completions", strings.NewReader(`{"model":"x"}`))
	req.RemoteAddr = "192.168.1.42:51515"
	req.Header.Set("Authorization", "Bearer pas-la-bonne")
	h.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("mauvaise clé : %d, attendu 401", w.Code)
	}

	// /v1/* avec la bonne clé : notre contrôle d'authentification est franchi.
	// Le moteur derrière est absent dans ce test (502) — ce qui prouve le
	// passage, c'est que ce n'est plus NOTRE refus.
	w = httptest.NewRecorder()
	req = httptest.NewRequest("GET", "http://192.168.1.9/v1/models", nil)
	req.RemoteAddr = "192.168.1.42:51515"
	req.Header.Set("Authorization", "Bearer sk-ajean-pont-test")
	h.ServeHTTP(w, req)
	if strings.Contains(w.Body.String(), "clé API invalide") || w.Code == http.StatusNotFound {
		t.Fatalf("clé valide refusée par le pont : %d\n%s", w.Code, w.Body.String())
	}
}

func TestParallelBridgeAPIReportsAndToggles(t *testing.T) {
	parallelTestHome(t)
	t.Setenv("AJEAN_BRIDGE", "")
	if err := writeAPIKey("sk-ajean-pont-api"); err != nil {
		t.Fatal(err)
	}
	_ = setParallelBridgeRequested(false)
	t.Cleanup(func() { _ = stopParallelBridge() })

	w := httptest.NewRecorder()
	handleParallelBridge(w, httptest.NewRequest("GET", "http://127.0.0.1/api/bridge", nil))
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	var etat struct {
		Enabled    bool   `json:"enabled"`
		Running    bool   `json:"running"`
		Port       string `json:"port"`
		EnginePort string `json:"engine_port"`
		HasKey     bool   `json:"has_key"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &etat); err != nil {
		t.Fatal(err)
	}
	if etat.Enabled || etat.Running {
		t.Fatalf("le pont doit démarrer éteint : %+v", etat)
	}
	if etat.Port != parallelBridgePortDef || etat.EnginePort != parallelEnginePortDef {
		t.Fatalf("ports par défaut déplacés : %+v", etat)
	}
	if !etat.HasKey {
		t.Fatal("la clé de la machine n'est pas vue")
	}

	// Un port qui chevauche l'interface ou le moteur est refusé : trois ports
	// volants ne doivent pas pouvoir se marcher dessus.
	for _, p := range []string{parallelUIPort(), parallelEnginePort(), "80", "70000"} {
		body, _ := json.Marshal(map[string]any{"port": p})
		w = httptest.NewRecorder()
		handleParallelBridge(w, httptest.NewRequest("POST", "http://127.0.0.1/api/bridge", strings.NewReader(string(body))))
		if w.Code == 200 {
			t.Fatalf("port %s accepté : %s", p, w.Body.String())
		}
	}
	if getStr(bkState, parallelBridgePortKey) != "" {
		t.Fatal("un port refusé ne doit pas être enregistré")
	}

	// Décaler le pont sur un port libre, l'allumer, l'appeler, le couper.
	port := freeTestPort(t)
	body, _ := json.Marshal(map[string]any{"port": port})
	w = httptest.NewRecorder()
	handleParallelBridge(w, httptest.NewRequest("POST", "http://127.0.0.1/api/bridge", strings.NewReader(string(body))))
	if w.Code != 200 || parallelBridgePort() != port {
		t.Fatalf("port du pont non enregistré : %d %s", w.Code, w.Body.String())
	}

	body, _ = json.Marshal(map[string]any{"enabled": true})
	w = httptest.NewRecorder()
	handleParallelBridge(w, httptest.NewRequest("POST", "http://127.0.0.1/api/bridge", strings.NewReader(string(body))))
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	if !parallelBridgeRequested() {
		t.Fatalf("pont non allumé : %s", w.Body.String())
	}
	if !parallelBridgeRunning() {
		t.Fatalf("pont demandé mais pas à l'écoute : %s", w.Body.String())
	}
	base := "http://127.0.0.1:" + port
	resp, err := http.Get(base + "/api/memory")
	if err != nil {
		t.Fatalf("pont injoignable : %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("/api/memory sur le pont : %d, attendu 404", resp.StatusCode)
	}
	resp, err = http.Get(base + "/v1/models")
	if err != nil {
		t.Fatalf("/v1/models injoignable sur le pont : %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		t.Fatal("/v1/models borné à tort par le pont")
	}

	body, _ = json.Marshal(map[string]any{"enabled": false})
	w = httptest.NewRecorder()
	handleParallelBridge(w, httptest.NewRequest("POST", "http://127.0.0.1/api/bridge", strings.NewReader(string(body))))
	if parallelBridgeRequested() || parallelBridgeRunning() {
		t.Fatalf("pont toujours actif : %s", w.Body.String())
	}
	if _, err := http.Get(base + "/v1/models"); err == nil {
		t.Fatal("le pont écoute encore après coupure")
	}
}

// La preuve de bout en bout : Ajean A, en preset externe, parle au pont de
// l'instance B, qui proxifie vers SON moteur. Deux processus n'ont pas besoin
// d'être simulés ici — ce qui compte est la chaîne complète : preset externe →
// authentification du pont → proxy /v1/* → moteur → réponse, streaming compris.
func TestParallelBridgeEndToEndTwoInstances(t *testing.T) {
	parallelTestHome(t)
	t.Setenv("AJEAN_BRIDGE", "")
	t.Cleanup(func() { _ = stopParallelBridge() })

	// --- B : son moteur (faux mais réel sur TCP, sur le port que B déclare) ---
	engineLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skip("aucun port libre pour ce test")
	}
	enginePort := strconv.Itoa(engineLn.Addr().(*net.TCPAddr).Port)
	moteurB := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/models":
			json.NewEncoder(w).Encode(map[string]any{"data": []map[string]string{{"id": "cerveau-de-B"}}})
		case "/v1/chat/completions":
			if r.Header.Get("Authorization") != "Bearer sk-cle-de-B" {
				http.Error(w, `{"error":{"message":"invalid api key"}}`, http.StatusUnauthorized)
				return
			}
			if strings.Contains(r.Header.Get("Accept"), "text/event-stream") {
				w.Header().Set("Content-Type", "text/event-stream")
				flusher := w.(http.Flusher)
				fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"Bon\"}}]}\n\n")
				flusher.Flush()
				fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"jour\"}}]}\n\n")
				flusher.Flush()
				fmt.Fprint(w, "data: [DONE]\n\n")
				return
			}
			json.NewEncoder(w).Encode(map[string]any{
				"model": "cerveau-de-B",
				"choices": []map[string]any{{
					"message":      map[string]string{"role": "assistant", "content": "Bonjour depuis B"},
					"finish_reason": "stop",
				}},
				"usage": map[string]int{"prompt_tokens": 4, "completion_tokens": 6},
			})
		default:
			http.NotFound(w, r)
		}
	})}
	go func() { _ = moteurB.Serve(engineLn) }()
	defer moteurB.Close()

	// B enregistre son port moteur et sa clé : c'est l'instance qui prête.
	if err = putStr(bkState, "parallel_engine_port", enginePort); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = putStr(bkState, "parallel_engine_port", "") })
	if err = WriteConfig(map[string]string{"MODEL": "cerveau-B.gguf"}); err != nil {
		t.Fatal(err)
	}
	if ReadConfig()["PORT"] != enginePort {
		t.Fatalf("port moteur non appliqué à la config : %+v", ReadConfig())
	}
	if err = writeAPIKey("sk-cle-de-B"); err != nil {
		t.Fatal(err)
	}

	// --- B : son pont, sur TCP (le front qu'on prête au réseau local) ---
	bridgeLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skip("aucun port libre pour ce test")
	}
	bridgePort := strconv.Itoa(bridgeLn.Addr().(*net.TCPAddr).Port)
	pontB := &http.Server{Handler: parallelBridgeHandler()}
	go func() { _ = pontB.Serve(bridgeLn) }()
	defer pontB.Close()

	// --- A : il ne connaît que l'URL du pont et la clé ---
	if err = WriteConfig(map[string]string{
		extKeyFlag: "1", extKeyURL: "http://127.0.0.1:" + bridgePort + "/v1",
		extKeyModel: "cerveau-de-B", extKeyToken: "sk-cle-de-B",
	}); err != nil {
		t.Fatal(err)
	}
	ep := resolveChatEndpoint()
	if !ep.External || ep.URL != "http://127.0.0.1:"+bridgePort+"/v1/chat/completions" {
		t.Fatalf("endpoint A faux : %+v", ep)
	}
	body, _ := json.Marshal(map[string]any{"model": ep.Model, "messages": []map[string]string{{"role": "user", "content": "bonjour"}}})
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, ep.URL, strings.NewReader(string(body)))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	// Comme le vrai chemin d'appel (llm_client.go:1130) : la clé de l'endpoint
	// part en Bearer.
	if ep.Key != "" {
		req.Header.Set("Authorization", "Bearer "+ep.Key)
	}
	resp, err := doLLM(context.Background(), req, body, ep)
	if err != nil {
		t.Fatalf("A → pont de B : %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 500))
		t.Fatalf("statut %d : %s", resp.StatusCode, b)
	}
	var out struct {
		Choices []struct {
			Message      map[string]string `json:"message"`
			FinishReason string            `json:"finish_reason"`
		} `json:"choices"`
	}
	if err = json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if len(out.Choices) == 0 || out.Choices[0].Message["content"] != "Bonjour depuis B" {
		t.Fatalf("réponse de B absente : %+v", out)
	}

	// Streaming : les morceaux arrivent au fil de l'eau, pas en une seule fois.
	req2, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, ep.URL, strings.NewReader(`{"stream":true}`))
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("Accept", "text/event-stream")
	req2.Header.Set("Authorization", "Bearer "+ep.Key)
	resp2, err := doLLM(context.Background(), req2, []byte(`{"stream":true}`), ep)
	if err != nil {
		t.Fatalf("stream A → pont de B : %v", err)
	}
	defer resp2.Body.Close()
	sc := bufio.NewScanner(resp2.Body)
	var morceaux []string
	for sc.Scan() {
		ligne := sc.Text()
		if strings.Contains(ligne, "Bon") || strings.Contains(ligne, "jour") {
			morceaux = append(morceaux, ligne)
		}
	}
	if len(morceaux) < 2 {
		t.Fatalf("streaming non rendu morceau par morceau : %v", morceaux)
	}
}

// Une copie décalée ne doit plus être punie pour des chiffres obsolètes : les
// garde-fous de port suivent l'instance, pas une constante gravée.
func TestParallelPortsMoveTogether(t *testing.T) {
	parallelTestHome(t)
	t.Setenv("AJEAN_ENGINE_PORT", "")
	if parallelUIPort() != parallelUIPortDef || parallelEnginePort() != parallelEnginePortDef {
		t.Fatalf("départs déplacés : %s / %s", parallelUIPort(), parallelEnginePort())
	}
	t.Setenv("AJEAN_UI_PORT", "18094")
	if parallelUIPort() != "18094" {
		t.Fatalf("AJEAN_UI_PORT ignoré : %s", parallelUIPort())
	}
	// Un preset importé de la production ne ramène jamais le moteur à 8080.
	next, err := normalizeParallelConfig(map[string]string{"MODEL": "Swift.gguf", "PORT": "8080", "HOST": "0.0.0.0"})
	if err != nil {
		t.Fatal(err)
	}
	if next["PORT"] != parallelEnginePortDef || next["HOST"] != "127.0.0.1" {
		t.Fatalf("config d'import non ramenée : %+v", next)
	}
	// L'encodeur : 18081 reste bon, le port de l'interface décalée est refusé.
	enc := embeddingConfig{Device: "cpu", Threads: 8, Port: 18081}
	if err := validateEmbeddingConfig(enc); err != nil {
		t.Fatalf("18081 refusé : %v", err)
	}
	enc.Port = 18094
	if err := validateEmbeddingConfig(enc); err == nil {
		t.Fatal("l'encodeur accepte le port de l'interface")
	}
	enc.Port = 18090 // l'ancien chiffre, devenu libre : plus de faux refus
	if err := validateEmbeddingConfig(enc); err != nil {
		t.Fatalf("18090 libre refusé : %v", err)
	}
}

// Se connecter à soi-même reste interdit, quel que soit le port choisi. Le port
// moteur est décalé sur une plage libre pour que le faux serveur /v1/models
// réponde vraiment et que la branche « succès » ne soit pas confondue avec un
// port voisin.
func TestParallelConnectRefusesOwnBridge(t *testing.T) {
	parallelTestHome(t)
	t.Setenv("AJEAN_UI_PORT", "18094")
	if err := putStr(bkState, "parallel_engine_port", "18099"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = putStr(bkState, "parallel_engine_port", "") })

	for _, u := range []string{
		"http://127.0.0.1:" + parallelUIPort() + "/v1",
		"http://127.0.0.1:" + parallelBridgePort() + "/v1",
		"http://127.0.0.1:" + parallelEnginePort() + "/v1",
		"http://127.0.0.1:18081/v1",
	} {
		if err := connectParallelChat(context.Background(), u, ""); err == nil || !strings.Contains(err.Error(), "autre Ajean") {
			t.Fatalf("%s : %v", u, err)
		}
	}

	// Une URL qui n'est aucun de nos ports passe le contrôle et va interroger
	// le serveur : c'est bien la comparaison d'instance qui filtre, pas un refus
	// systématique.
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"data": []map[string]string{{"id": "cerveau-d-ailleurs"}}})
	}))
	defer fake.Close()
	if err := connectParallelChat(context.Background(), fake.URL+"/v1", ""); err != nil {
		t.Fatalf("autre Ajean refusé : %v", err)
	}
	if ReadConfig()[extKeyModel] != "cerveau-d-ailleurs" {
		t.Fatalf("preset non enregistré : %+v", ReadConfig())
	}
}
