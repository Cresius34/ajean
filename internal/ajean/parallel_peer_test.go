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
	"fmt"
	"io"
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
	reply, err := peerAsk(peerTestCtx(t), peerMustFind(t, "poste-salon"), "qui es-tu ?", canalOrigineIA)
	if err != nil {
		t.Fatal(err)
	}
	// La réponse rendue à l'agent porte la balise du canal (décision du
	// 2026-10-27 : distinguer ce qui vient du voisin d'une interaction extérieure),
	// et dit SURTOUT ce qui a répondu là-bas : ici le moteur nu, donc rien ne
	// s'exécute chez le voisin.
	//
	// Le préfixe s'arrête à « modèle nu » et non au crochet : depuis la révision 6,
	// le pont voisin annonce aussi POURQUOI il n'a pas pris son tour d'agent, et ce
	// motif vit dans la balise (l'emplacement des métadonnées), pas collé à la suite
	// des mots du voisin. Ce test n'a pas à figer la phrase du refus — il vérifie la
	// balise, et que les mots du voisin arrivent intacts et en dernier.
	if !strings.HasPrefix(reply, "[Réponse du canal Ajean↔Ajean · voisin « poste-salon » · il a répondu par son modèle nu") {
		t.Fatalf("la réponse de l'appelant n'est pas balisée comme attendu : %q", reply)
	}
	if !strings.HasSuffix(reply, "Bonjour, moi c'est l'autre IA") {
		t.Fatalf("le texte du voisin a été abîmé : %q", reply)
	}
	// Le souvenir, lui, reste NU : il est rejoué au voisin à chaque tour.
	if brut := getStr(bkState, peerThreadKey(peerMustFind(t, "poste-salon"))); strings.Contains(brut, "Réponse du canal") {
		t.Fatalf("la balise s'est glissée dans le souvenir rejoué au voisin : %s", brut)
	}
	// Ce qui est réellement parti du côté appelé. Le pont ajoute une carte
	// d'identité (bridgeDeclareSelf) : elle doit être là, UNIQUE, et ne jamais
	// s'empiler tour après tour — le souvenir, lui, ne garde que user/assistant.
	msgs, _ := reçu["messages"].([]any)
	système, fil := decompterMessages(msgs)
	if système != 1 {
		t.Fatalf("le pont doit injecter exactement une carte d'identité, %d trouvées (%+v)", système, reçu["messages"])
	}
	if fil != 1 {
		t.Fatalf("messages partis : %+v", reçu["messages"])
	}
	if _, outils := reçu["tools"]; outils {
		t.Fatal("des définitions d'outils ont traversé le canal : un voisin pourrait faire exécuter ici")
	}

	// Deuxième message : le souvenir doit suivre, sinon chaque tour est un premier
	// message et le « canal » n'est qu'une série de questions isolées.
	if _, err := peerAsk(peerTestCtx(t), peerMustFind(t, "poste-salon"), "et maintenant ?", canalOrigineIA); err != nil {
		t.Fatal(err)
	}
	msgs, _ = reçu["messages"].([]any)
	système, fil = decompterMessages(msgs)
	if système != 1 {
		t.Fatalf("la carte d'identité s'empile : %d prompts système (%+v)", système, reçu["messages"])
	}
	if fil != 3 {
		t.Fatalf("mémoire du canal absente : %d messages hors identité vus par le voisin (%+v)", fil, reçu["messages"])
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
	_, err := peerAsk(peerTestCtx(t), peerMustFind(t, "voisin"), "bonjour", canalOrigineIA)
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

// decompterMessages : sépare la carte d'identité injectée par le pont du fil
// réel. Un compteur unique ne voyait pas la différence entre « le souvenir suit »
// et « l'identité s'empile tour après tour », deux bugs opposés.
func decompterMessages(msgs []any) (systeme, fil int) {
	for _, m := range msgs {
		mm, ok := m.(map[string]any)
		if !ok {
			continue
		}
		if strings.ToLower(strings.TrimSpace(fmt.Sprint(mm["role"]))) == "system" {
			systeme++
			continue
		}
		fil++
	}
	return systeme, fil
}

// Le pont prête un modèle NU : sans carte d'identité, ce modèle invente ce qu'il
// est. Mesuré le 2026-10-27 sur le poste voisin, à qui demandait s'il avait un
// outil peer : « je fonctionne sur une infrastructure cloud, pas sur un poste
// physique », puis « le voisin est ajouté sur le pont 18190 » — alors qu'aucune
// commande d'ajout n'existe côté modèle. Les deux réponses étaient fausses.
func TestBridgeDeclaresItselfOnlyAcrossTheCanal(t *testing.T) {
	parallelTestHome(t)
	t.Setenv("AJEAN_BRIDGE", "")

	var vus []map[string]any
	engine := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var in map[string]any
		_ = json.NewDecoder(r.Body).Decode(&in)
		vus = append(vus, in)
		json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{"message": map[string]string{"content": "ok"}, "finish_reason": "stop"}},
		})
	}))
	defer engine.Close()

	if err := writeAPIKey("sk-ici"); err != nil {
		t.Fatal(err)
	}
	// Le port du moteur passe par le RÉGLAGE d'instance, jamais par la config :
	// normalizeParallelConfig écrase PORT par parallelEnginePort()
	// (parallel_config.go:116). Écrire PORT dans WriteConfig ne trompe personne —
	// et sans cette ligne le pont va frapper le moteur réel du poste de test, qui
	// répond 401 avec SA clé. Mesuré, pas supposé.
	if err := putStr(bkState, "parallel_engine_port", urlPort(engine.URL)); err != nil {
		t.Fatal(err)
	}
	if err := WriteConfig(map[string]string{"MODEL": "cerveau-ici.gguf"}); err != nil {
		t.Fatal(err)
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skip("pas de port libre")
	}
	srv := &http.Server{Handler: parallelBridgeHandler()}
	go func() { _ = srv.Serve(ln) }()
	t.Cleanup(func() { _ = srv.Close() })

	envoyer := func(avecCanal bool) {
		t.Helper()
		req, _ := http.NewRequest(http.MethodPost,
			"http://127.0.0.1:"+strconv.Itoa(ln.Addr().(*net.TCPAddr).Port)+"/v1/chat/completions",
			strings.NewReader(`{"model":"x","messages":[{"role":"user","content":"as-tu un terminal ?"}]}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer sk-ici")
		if avecCanal {
			req.Header.Set(canalHeader, "1")
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			b, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<14))
			t.Fatalf("statut %d : %s", resp.StatusCode, b)
		}
	}

	envoyer(true)
	msgs, _ := vus[0]["messages"].([]any)
	if s, _ := decompterMessages(msgs); s != 1 {
		t.Fatalf("le modèle prêté ne reçoit aucune carte d'identité (%+v)", vus[0]["messages"])
	}
	if content := fmt.Sprint(msgs[0].(map[string]any)["content"]); !strings.Contains(content, "AUCUN outil") {
		t.Fatalf("la carte doit dire au modèle qu'il n'a pas d'outils : %s", content)
	}

	// Un logiciel tiers branché sur le pont n'est pas un Ajean : sa requête
	// traverse telle quelle. Le pont ne réécrit pas le dialogue d'un client.
	envoyer(false)
	if s, fil := decompterMessages(vus[1]["messages"].([]any)); s != 0 || fil != 1 {
		t.Fatalf("requête d'un client tiers réécrite : %+v", vus[1]["messages"])
	}
}

// La volonté de l'appelant passe avant celle du pont : si le voisin a déjà posé
// un prompt système, le pont ne se substitue pas à lui.
func TestBridgeIdentityYieldsToCallerSystemPrompt(t *testing.T) {
	parallelTestHome(t)
	t.Setenv("AJEAN_BRIDGE", "")

	var reçu map[string]any
	engine := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&reçu)
		json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{"message": map[string]string{"content": "ok"}, "finish_reason": "stop"}},
		})
	}))
	defer engine.Close()
	if err := writeAPIKey("sk-ici"); err != nil {
		t.Fatal(err)
	}
	if err := putStr(bkState, "parallel_engine_port", urlPort(engine.URL)); err != nil {
		t.Fatal(err)
	}
	if err := WriteConfig(map[string]string{"MODEL": "cerveau-ici.gguf"}); err != nil {
		t.Fatal(err)
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skip("pas de port libre")
	}
	srv := &http.Server{Handler: parallelBridgeHandler()}
	go func() { _ = srv.Serve(ln) }()
	t.Cleanup(func() { _ = srv.Close() })

	req, _ := http.NewRequest(http.MethodPost,
		"http://127.0.0.1:"+strconv.Itoa(ln.Addr().(*net.TCPAddr).Port)+"/v1/chat/completions",
		strings.NewReader(`{"model":"x","messages":[{"role":"system","content":"Tu es un traducteur."},{"role":"user","content":"bonjour"}]}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer sk-ici")
	req.Header.Set(canalHeader, "1")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	msgs, _ := reçu["messages"].([]any)
	if len(msgs) != 2 {
		t.Fatalf("le pont a ajouté son identité malgré un prompt système de l'appelant : %+v", reçu["messages"])
	}
	if fmt.Sprint(msgs[0].(map[string]any)["content"]) != "Tu es un traducteur." {
		t.Fatalf("le prompt système de l'appelant a été déplacé : %+v", reçu["messages"])
	}
}

// Le refus doit dire POURQUOI il refuse. Six préfixes partageaient un seul
// message, qui renvoyait vers la « page Mémoire sémantique » — sans rapport avec
// le réseau. Capturé par l'utilisateur le 2026-10-27 en cliquant sur
// « joignable depuis le réseau local ».
func TestParallelRefusentAvecLeurPropreRaison(t *testing.T) {
	parallelTestHome(t)
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Error("une route bannie a atteint son gestionnaire")
	})
	h := parallelHTTPHandler(next)

	vus := map[string]string{}
	for _, b := range parallelBlockedRoutes {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, b.prefix+"test", nil))
		if w.Code != http.StatusForbidden {
			t.Fatalf("%s : statut %d, attendu 403", b.prefix, w.Code)
		}
		var out struct {
			Error string `json:"error"`
		}
		_ = json.Unmarshal(w.Body.Bytes(), &out)
		if out.Error == "" {
			t.Fatalf("%s : refus sans raison", b.prefix)
		}
		vus[out.Error] = b.prefix
	}
	if len(vus) != len(parallelBlockedRoutes) {
		t.Fatalf("des refus partagent la même raison : %d raisons pour %d routes", len(vus), len(parallelBlockedRoutes))
	}

	// Le cas mesuré : la route réseau doit parler de réseau, pas de mémoire.
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/network", nil))
	msg := w.Body.String()
	if strings.Contains(msg, "Mémoire sémantique") {
		t.Fatalf("le refus réseau renvoie encore à la Mémoire sémantique : %s", msg)
	}
	if !strings.Contains(msg, "pont") {
		t.Fatalf("le refus réseau doit désigner le seul port ouvert, le pont : %s", msg)
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
