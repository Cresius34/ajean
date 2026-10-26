package ajean

// Le canal Ajean↔Ajean, prouvé des deux bouts : peerAsk d'un côté,
// parallelBridgeHandler de l'autre, un faux moteur derrière. Ce qu'on vérifie
// n'est pas « ça répond » mais les règles du canal :
//   - ce qui traverse est du texte ; aucune définition d'outil ne part d'ici ;
//   - l'échange a une mémoire, donc le deuxième message n'est pas un premier ;
//   - la clé du voisin n'est jamais exposée par l'API ;
//   - une instance ne se prête pas à elle-même.
//
// Les deux bouts vivent dans le MÊME processus sans se marcher dessus : le pont
// lit le preset (bkConfig) et le canal lit sa propre liste (bkState) — deux
// magasins distincts, ce que ce test vérifie au passage.

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

// peerTestNeighbour : la machine d'à côté — son moteur, sa clé, son pont ouvert
// sur un vrai port TCP.
func peerTestNeighbour(t *testing.T, key, reply string, vus *map[string]any) string {
	t.Helper()

	engine := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/models":
			json.NewEncoder(w).Encode(map[string]any{"data": []map[string]string{{"id": "cerveau-du-voisin"}}})
		case "/v1/chat/completions":
			var in map[string]any
			_ = json.NewDecoder(r.Body).Decode(&in)
			if vus != nil {
				*vus = in
			}
			json.NewEncoder(w).Encode(map[string]any{
				"choices": []map[string]any{{
					"message":       map[string]string{"role": "assistant", "content": reply},
					"finish_reason": "stop",
				}},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(engine.Close)

	// Le voisin prête un moteur LOCAL : c'est le cas normal (une machine qui prête
	// son MoE). Son preset à lui, sa clé à lui.
	if err := putStr(bkState, "parallel_engine_port", urlPort(engine.URL)); err != nil {
		t.Fatal(err)
	}
	if err := WriteConfig(map[string]string{"MODEL": "cerveau-du-voisin.gguf"}); err != nil {
		t.Fatal(err)
	}
	if err := writeAPIKey(key); err != nil {
		t.Fatal(err)
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skip("pas de port libre pour ce test")
	}
	srv := &http.Server{Handler: parallelBridgeHandler()}
	go func() { _ = srv.Serve(ln) }()
	t.Cleanup(func() { _ = srv.Close() })
	return "http://127.0.0.1:" + strconv.Itoa(ln.Addr().(*net.TCPAddr).Port) + "/v1"
}

// peerMustFind : un voisin que le test vient de déclarer ; s'il n'est pas là,
// c'est la déclaration qui a échoué et le test doit le dire.
func peerMustFind(t *testing.T, name string) peerEndpoint {
	t.Helper()
	p, ok := peerFind(name)
	if !ok {
		t.Fatalf("voisin %s introuvable", name)
	}
	return p
}

// peerTestCtx : la même échéance que l'outil, pour qu'un test qui pend échoue
// au lieu de faire pendre la suite entière.
func peerTestCtx(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), peerAskTimeout)
	t.Cleanup(cancel)
	return ctx
}

func urlPort(raw string) string {
	h := raw
	if i := strings.Index(h, "://"); i >= 0 {
		h = h[i+3:]
	}
	if i := strings.Index(h, "/"); i >= 0 {
		h = h[:i]
	}
	if i := strings.LastIndex(h, ":"); i >= 0 {
		return h[i+1:]
	}
	return "80"
}

func TestPeerChannelCarriesTextAndMemory(t *testing.T) {
	parallelTestHome(t)
	t.Setenv("AJEAN_BRIDGE", "")

	var reçu map[string]any
	bridgeURL := peerTestNeighbour(t, "sk-cle-du-voisin", "Bonjour, moi c'est l'autre IA", &reçu)

	// Déclarer un voisin ne touche PAS au preset : les deux magasins sont séparés,
	// sinon prêter et parler se marcheraient dessus.
	if _, err := peerAddDeclared("poste-salon", bridgeURL, "sk-cle-du-voisin", ""); err != nil {
		t.Fatal(err)
	}
	if ReadConfig()["MODEL"] != "cerveau-du-voisin.gguf" {
		t.Fatalf("déclarer un voisin a abîmé le preset : %+v", ReadConfig())
	}
	if !peerConfigured() {
		t.Fatal("peerConfigure should être vrai après déclaration")
	}

	// Premier message.
	reply, err := peerAsk(peerTestCtx(t), peerMustFind(t, "poste-salon"), "qui es-tu ?")
	if err != nil {
		t.Fatal(err)
	}
	if reply != "Bonjour, moi c'est l'autre IA" {
		t.Fatalf("réponse du canal : %q", reply)
	}
	// Ce qui est réellement parti du côté appelé.
	msgs, _ := reçu["messages"].([]any)
	if len(msgs) != 1 {
		t.Fatalf("messages partis : %+v", reçu["messages"])
	}
	if _, outils := reçu["tools"]; outils {
		t.Fatal("des définitions d'outils ont traversé le canal : un voisin pourrait faire exécuter ici")
	}

	// Deuxième message : le souvenir doit suivre, sinon chaque tour est un premier
	// message et le « canal » n'est qu'une série de questions isolées.
	if _, err := peerAsk(peerTestCtx(t), peerMustFind(t, "poste-salon"), "et maintenant ?"); err != nil {
		t.Fatal(err)
	}
	msgs, _ = reçu["messages"].([]any)
	if len(msgs) != 3 {
		t.Fatalf("mémoire du canal absente : %d messages vus par le voisin (%+v)", len(msgs), reçu["messages"])
	}

	// Le voisin est joignable par l'outil, pas seulement par la fonction.
	if out := runPeerTool(map[string]any{"action": "status", "peer": "poste-salon"}); !strings.Contains(out, "cerveau-du-voisin") {
		t.Fatalf("peer status : %s", out)
	}
	if out := runPeerTool(map[string]any{"action": "ask", "peer": "poste-salon", "message": "troisième"}); !strings.Contains(out, "l'autre IA") {
		t.Fatalf("peer ask : %s", out)
	}
	if out := runPeerTool(map[string]any{"action": "ask", "peer": "fantôme", "message": "x"}); !strings.Contains(out, "[erreur]") {
		t.Fatalf("voisin inconnu doit refuser : %s", out)
	}

	// Oublier efface le souvenir sans effacer le voisin.
	if out := runPeerTool(map[string]any{"action": "forget", "peer": "poste-salon"}); !strings.Contains(out, "[ok]") {
		t.Fatalf("peer forget : %s", out)
	}
	if len(peerThread(peerMustFind(t, "poste-salon"))) != 0 {
		t.Fatal("le souvenir est resté")
	}
	if _, ok := peerFind("poste-salon"); !ok {
		t.Fatal("forget a effacé le voisin, ce n'était pas demandé")
	}
}

func TestPeerChannelRejectsWrongKey(t *testing.T) {
	parallelTestHome(t)
	t.Setenv("AJEAN_BRIDGE", "")

	bridgeURL := peerTestNeighbour(t, "sk-cle-du-voisin", "x", nil)
	if _, err := peerAddDeclared("voisin", bridgeURL, "sk-mauvaise-cle", ""); err != nil {
		t.Fatal(err)
	}
	_, err := peerAsk(peerTestCtx(t), peerMustFind(t, "voisin"), "bonjour")
	if err == nil {
		t.Fatal("une mauvaise clé a été acceptée par le pont")
	}
	if !strings.Contains(err.Error(), "clé API refusée") {
		t.Fatalf("diagnostic de clé trompeur : %v", err)
	}
}

// Deux Ajean sur le même PC : le piège classique est de prêter le pont à
// lui-même. Chaque appel se renverrait à lui-même jusqu'à saturer les sockets ;
// le pont doit le dire au lieu de mourir lentement.
func TestBridgeRefusesToLendToItself(t *testing.T) {
	parallelTestHome(t)
	t.Setenv("AJEAN_BRIDGE", "")

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skip("pas de port libre")
	}
	port := strconv.Itoa(ln.Addr().(*net.TCPAddr).Port)
	if err := putStr(bkState, parallelBridgePortKey, port); err != nil {
		t.Fatal(err)
	}
	if err := writeAPIKey("sk-ici"); err != nil {
		t.Fatal(err)
	}
	// Le preset « prêté » pointe sur ce pont-ci.
	if err := WriteConfig(map[string]string{
		extKeyFlag: "1", extKeyURL: "http://127.0.0.1:" + port + "/v1", extKeyModel: "moi",
	}); err != nil {
		t.Fatal(err)
	}
	ep := resolveChatEndpoint()
	if !bridgePointsToItself(ep) {
		t.Fatalf("boucle non détectée : %+v", ep)
	}

	srv := &http.Server{Handler: parallelBridgeHandler()}
	go func() { _ = srv.Serve(ln) }()
	t.Cleanup(func() { _ = srv.Close() })

	req, _ := http.NewRequest(http.MethodPost, "http://127.0.0.1:"+port+"/v1/chat/completions", strings.NewReader(`{"model":"moi"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer sk-ici")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("statut %d, attendu 409", resp.StatusCode)
	}
	// Un voisin sur une AUTRE machine du même réseau n'est pas soi.
	if bridgePointsToItself(chatEndpoint{URL: "http://192.168.1.132:" + port + "/v1/chat/completions"}) {
		t.Fatal("un voisin distant pris pour soi")
	}
}

// La clé du voisin ne sort jamais de la machine qui la garde.
func TestPeersAPINeverLeaksKeys(t *testing.T) {
	parallelTestHome(t)
	if _, err := peerAddDeclared("voisin", "http://192.168.1.132:18190/v1", "sk-top-secrete", ""); err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	handleParallelPeers(w, httptest.NewRequest(http.MethodGet, "/api/peers", nil))
	body := w.Body.String()
	if strings.Contains(body, "sk-top-secrete") {
		t.Fatalf("la clé fuit par /api/peers : %s", body)
	}
	if !strings.Contains(body, "has_key") {
		t.Fatalf("l'état de la clé devrait être visible : %s", body)
	}

	// Retirer le voisin emporte son souvenir : sinon un futur voisin portant le
	// même nom hériterait d'une conversation qui ne lui appartient pas.
	w = httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/peers", strings.NewReader(`{"remove":"voisin"}`))
	handleParallelPeers(w, req)
	if w.Code != 200 {
		t.Fatalf("retirer : %d %s", w.Code, w.Body.String())
	}
	if _, ok := peerFind("voisin"); ok {
		t.Fatal("le voisin est resté")
	}
	if raw := getStr(bkState, peerThreadKey(peerEndpoint{URL: "http://192.168.1.132:18190/v1"})); raw != "" {
		t.Fatal("le souvenir a survécu au voisin")
	}
}

// URL mal saisie = refus net, pas un appel impossible dont le diagnostic serait
// ensuite introuvable.
func TestPeerAddNormalizesAndRejects(t *testing.T) {
	parallelTestHome(t)
	p, err := peerAddDeclared("", "192.168.1.132:18190/v1", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(p.URL, "http://") || !strings.HasSuffix(p.URL, "/v1") {
		t.Fatalf("url recollée incorrectement : %q", p.URL)
	}
	if p.Name == "" {
		t.Fatal("un voisin sans nom ne peut pas être choisi")
	}
	if _, err = peerAddDeclared("x", "ftp://192.168.1.132/v1", "", ""); err == nil {
		t.Fatal("un schéma impossible accepté")
	}
	if _, err = peerAddDeclared("y", "", "", ""); err == nil {
		t.Fatal("une url vide acceptée")
	}
}
