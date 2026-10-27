package ajean

// parallel_bridge_agent_test.go — ce que le canal rend quand il parle à l'AGENT.
//
// Le tour complet (modèle + outils) ne peut pas se jouer ici : il faut un moteur
// prêt. Ce fichier teste donc les briques qui décident du comportement, et qui sont
// précisément là où le contrat a changé : qui déclenche un tour d'agent, ce que
// waitCanalTurn considère comme « fini », et quelle réponse est rendue.

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Le garde-fou de sécurité : un tour d'agent met le shell et le disque de la
// machine à la portée de qui possède la clé du pont. Il faut DONC deux
// autorisations indépendantes, toutes deux éteintes par défaut.
func TestCanalNeDeclencheUnTourAgentQueSurDoubleAutorisation(t *testing.T) {
	parallelTestHome(t)

	requete := func(chemin, canal string) *http.Request {
		r := httptest.NewRequest(http.MethodPost, chemin, strings.NewReader(`{"messages":[{"role":"user","content":"x"}]}`))
		if canal != "" {
			r.Header.Set(canalHeader, canal)
		}
		return r
	}
	valide := requete("/v1/chat/completions", "1")

	// Rien n'est allumé : le pont n'a pas le droit d'exécuter quoi que ce soit.
	if canalDemande(valide) {
		t.Fatal("par défaut le canal ne déclenche aucun tour d'agent")
	}
	if parallelBridgeAgentOn() {
		t.Fatal("l'autorisation du pont doit naître éteinte")
	}
	if agentEnabled() {
		t.Fatal("le mode agent doit naître éteint dans cette installation de test")
	}

	// Une seule des deux autorisations ne suffit pas.
	if err := setParallelBridgeAgentOn(true); err != nil {
		t.Fatal(err)
	}
	if canalDemande(valide) {
		t.Fatal("autorisation du pont seule ne doit rien exécuter : le mode agent de la machine compte aussi")
	}
	if err := setParallelBridgeAgentOn(false); err != nil {
		t.Fatal(err)
	}
	if err := putStr(bkState, "agent", "1"); err != nil {
		t.Fatal(err)
	}
	if canalDemande(valide) {
		t.Fatal("mode agent seul ne doit rien exécuter pour un voisin : l'autorisation du pont compte aussi")
	}

	// Les deux : le tour d'agent est attendu.
	if err := setParallelBridgeAgentOn(true); err != nil {
		t.Fatal(err)
	}
	if !canalDemande(valide) {
		t.Fatal("double autorisation : un tour d'agent est attendu")
	}

	// Un logiciel tiers qui branche son logiciel sur le pont n'est pas un Ajean :
	// il garde le modèle prêté, sans rien exécuter ici.
	if canalDemande(requete("/v1/chat/completions", "")) {
		t.Fatal("sans l'en-tête du canal, la requête d'un client tiers ne déclenche rien")
	}
	// Ni un autre chemin, ni une méthode en lecture.
	if canalDemande(requete("/v1/models", "1")) {
		t.Fatal("/v1/models ne déclenche pas de tour")
	}
	rModelee := httptest.NewRequest(http.MethodGet, "/v1/chat/completions", nil)
	rModelee.Header.Set(canalHeader, "1")
	if canalDemande(rModelee) {
		t.Fatal("un GET ne déclenche pas de tour")
	}
}

// L'interrupteur doit être lisible depuis l'état du pont : sinon on cherche une
// heure pourquoi le voisin récite une réponse de modèle nu.
func TestEtatDuPontDeclareLAuthorisationAgent(t *testing.T) {
	parallelTestHome(t)
	st := parallelBridgeStatus()
	if st["agent_on"].(bool) {
		t.Fatal("agent_on doit être faux tant que rien n'est autorisé")
	}
	if err := setParallelBridgeAgentOn(true); err != nil {
		t.Fatal(err)
	}
	st = parallelBridgeStatus()
	if !st["agent_requested"].(bool) {
		t.Fatal("agent_requested ne suit pas l'autorisation du pont")
	}
	if st["agent_on"].(bool) {
		t.Fatal("le mode agent local est éteint : agent_on doit rester faux")
	}
	if err := putStr(bkState, "agent", "1"); err != nil {
		t.Fatal(err)
	}
	if !parallelBridgeStatus()["agent_on"].(bool) {
		t.Fatal("les deux autorisations prises : agent_on doit être vrai")
	}
}

// La balise est ce qui distingue, dans le fil partagé, un message venu du réseau
// d'un message de l'utilisateur. Sans elle, l'arbitrage du 2026-10-27 tombe.
func TestCanalBaliseDansLeTexteEtPasSeulementDansUnEnTete(t *testing.T) {
	msg := canalMessageTexte("DESKTOP-QUO1ID7", "recopie ton journal strata-gyro-gyro-s.log")
	if !strings.HasPrefix(msg, "[Canal Ajean↔Ajean") {
		t.Fatalf("le message du canal ne commence pas par la balise : %q", msg[:min(60, len(msg))])
	}
	if !strings.Contains(msg, "DESKTOP-QUO1ID7") {
		t.Fatalf("la machine qui parle n'est pas nommée : %q", msg)
	}
	if !strings.Contains(msg, "non de l'utilisateur") {
		t.Fatalf("le message doit dire à l'agent que son interlocuteur n'est pas l'utilisateur : %q", msg)
	}
	// Le texte du voisin arrive tel quel après la balise : le pont n'a pas à le
	// reformuler, c'est lui que l'utilisateur lit dans la bulle.
	if !strings.HasSuffix(msg, "recopie ton journal strata-gyro-gyro-s.log") {
		t.Fatalf("la demande du voisin a été modifiée : %q", msg)
	}
	// Voisin sans nom déclaré : la balise reste lisible.
	if msg2 := canalMessageTexte("", "bonjour"); !strings.Contains(msg2, "machine voisine") {
		t.Fatalf("nom de machine absent : %q", msg2)
	}
}

// Ce que l'appelant lit doit être balisé aussi : depuis que le pont rend un tour
// d'agent, la réponse du voisin ressemble trait pour trait à la mienne.
func TestReponseDuVoisinBaliseChezLAppelant(t *testing.T) {
	parallelTestHome(t)
	bridge := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			// model:"ajean-agent" est ce qu'annonce envoiOpenAI : c'est ainsi que
			// l'appelant sait, sans protocole de plus, si le voisin a répondu par son
			// agent (outils, disque) ou par son modèle nu.
			"model":   "ajean-agent",
			"choices": []map[string]any{{"message": map[string]string{"content": "voici mes journaux"}, "finish_reason": "stop"}},
		})
	}))
	defer bridge.Close()

	p, err := peerAddDeclared("bekka", bridge.URL+"/v1", "sk-cle", "")
	if err != nil {
		t.Fatal(err)
	}
	reply, err := peerAsk(peerTestCtx(t), p, "lit ton journal du moteur")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(reply, "[Réponse du canal Ajean↔Ajean · voisin « bekka » · il a répondu par son agent] ") {
		t.Fatalf("réponse de l'appelant non balisée comme attendu : %q", reply)
	}
	if !strings.HasSuffix(reply, "voici mes journaux") {
		t.Fatalf("texte du voisin abîmé : %q", reply)
	}
	// Le souvenir repart au voisin à chaque tour : il doit rester nu.
	if brut := getStr(bkState, peerThreadKey(p)); strings.Contains(brut, "Réponse du canal") {
		t.Fatalf("la balise s'est glissée dans le souvenir rejoué : %s", brut)
	}
}

// autoriserCanalAgent allume les deux interrupteurs (pont + mode agent de la
// machine) pour qu'un tour d'agent soit effectivement engagé dans le test.
func autoriserCanalAgent(t *testing.T) {
	t.Helper()
	if err := setParallelBridgeAgentOn(true); err != nil {
		t.Fatal(err)
	}
	if err := putStr(bkState, "agent", "1"); err != nil {
		t.Fatal(err)
	}
}

// bridgeAgentTurn refuse de se laisser appeler comme un exécuteur distant, même si
// un appelant futur en avait oublié la vérification en amont.
func TestBridgeAgentTurnRefuseSansAutorisation(t *testing.T) {
	parallelTestHome(t)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/v1/chat/completions",
		strings.NewReader(`{"messages":[{"role":"user","content":"supprime tout"}]}`))
	r.Header.Set(canalHeader, "1")
	bridgeAgentTurn(w, r)
	if w.Code != http.StatusForbidden {
		t.Fatalf("sans autorisation, attendu 403, obtenu %d : %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "bridge agent on") {
		t.Fatalf("le refus doit dire comment autoriser : %s", w.Body.String())
	}
}

// stream:true, requête sans message user, chaîne de postes : refusés avant de
// consommer un tour.
func TestCanalRefuseLeStreamSansMessageEtLeSecondSaut(t *testing.T) {
	parallelTestHome(t)
	autoriserCanalAgent(t)
	if err := writeAPIKey("sk-ici"); err != nil {
		t.Fatal(err)
	}
	if err := putStr(bkState, "parallel_engine_port", "1"); err != nil {
		t.Fatal(err)
	}
	if err := WriteConfig(map[string]string{"MODEL": "cerveau-ici.gguf"}); err != nil {
		t.Fatal(err)
	}

	// stream:true : refusé, avec la raison. Un tour d'agent recompose sa réponse
	// après chaque appel d'outil ; le streamer produirait un flux incohérent.
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/v1/chat/completions",
		strings.NewReader(`{"stream":true,"messages":[{"role":"user","content":"recopie ton journal"}]}`))
	r.Header.Set(canalHeader, "1")
	r.Header.Set(canalFromHeader, "DESKTOP-VOISIN")
	bridgeAgentTurn(w, r)
	if w.Code != http.StatusConflict {
		t.Fatalf("stream:true devait être refusé en 409, obtenu %d : %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "stream") {
		t.Fatalf("le refus doit nommer le streaming : %s", w.Body.String())
	}

	// Aucun message user : rien à traiter.
	w = httptest.NewRecorder()
	r = httptest.NewRequest(http.MethodPost, "/v1/chat/completions",
		strings.NewReader(`{"messages":[{"role":"assistant","content":"déjà dit"}]}`))
	r.Header.Set(canalHeader, "1")
	bridgeAgentTurn(w, r)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("requête sans message user devait être refusée en 400, obtenu %d", w.Code)
	}

	// Chaîne de postes qui se relaie la demande : refusée avant de consommer un tour.
	w = httptest.NewRecorder()
	r = httptest.NewRequest(http.MethodPost, "/v1/chat/completions",
		strings.NewReader(`{"messages":[{"role":"user","content":"encore toi"}]}`))
	r.Header.Set(canalHeader, "1")
	r.Header.Set(canalHopHeader, "2")
	bridgeAgentTurn(w, r)
	if w.Code != http.StatusLoopDetected {
		t.Fatalf("le 2e saut devait être refusé en 421, obtenu %d : %s", w.Code, w.Body.String())
	}
}

// waitCanalTurn : turn_done est la seule sortie « fini ». Une delta « error » ne
// l'est PAS — le tour continue après un appel d'outil raté.
func TestWaitCanalTurnAttendTurnDoneEtGardeLErreur(t *testing.T) {
	c := newTestConv()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		c.appendDelta(c.epoch, map[string]any{"user": "recopie ton journal"})
		c.appendDelta(c.epoch, map[string]any{"error": "outil bash : timeout"})
		c.appendDelta(c.epoch, map[string]any{"content": "voici le journal"})
		c.appendDelta(c.epoch, map[string]any{"turn_done": true, "elapsed_ms": 1200})
	}()

	derniereErreur, err := c.waitCanalTurn(ctx, c.epoch, 0)
	if err != nil {
		t.Fatalf("le tour s'est terminé proprement : %v", err)
	}
	if derniereErreur == "" {
		t.Fatal("l'erreur d'outil doit être retenue pour expliquer une réponse vide, rien n'a été gardé")
	}
	if !strings.Contains(derniereErreur, "timeout") {
		t.Fatalf("erreur retenée inattendue : %q", derniereErreur)
	}
}

// Un abonné à un epoch périmé (l'utilisateur a fait « clear chat » pendant le tour
// du voisin) sort au lieu d'attendre une fin qui n'arrivera jamais sur ce fil.
func TestWaitCanalTurnSortSurEpochPerime(t *testing.T) {
	c := newTestConv()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		// Reset en cours de tour : epoch bumpé, aucun turn_done sur le nouveau fil.
		c.mu.Lock()
		c.epoch++
		c.cond.Broadcast()
		c.mu.Unlock()
	}()
	if _, err := c.waitCanalTurn(ctx, c.epoch-1, 0); err == nil {
		t.Fatal("un epoch périmé doit interrompre l'attente")
	}
}

// La réponse rendue au voisin est l'unique message assistant final du tour. Les
// messages assistant porteurs d'un tool_call (contenu vide) ne comptent pas, et on
// ne remonte JAMAIS avant la borne du tour.
func TestDerniereReponseApresNeDepassePasLaBorneDuTour(t *testing.T) {
	c := newTestConv()
	c.Messages = []Message{
		{Role: "user", Content: "tour d'avant"},
		{Role: "assistant", Content: "réponse d'un tour ANTERIEUR"},
	}
	n := len(c.Messages)
	if got := c.derniereReponseApres(n); got != "" {
		t.Fatalf("un tour qui n'a rien produit ne doit pas emprunter la réponse d'avant, obtenu %q", got)
	}

	c.Messages = append(c.Messages,
		Message{Role: "assistant", ToolCalls: []ToolCall{{ID: "1", Function: ToolCallFunc{Name: "bash"}}}},
		Message{Role: "user", Content: "résultat de l'outil"},
		Message{Role: "assistant", Content: "voilà le journal demandé"},
	)
	if got := c.derniereReponseApres(n); got != "voilà le journal demandé" {
		t.Fatalf("la réponse finale attendue était le dernier message texte, obtenu %q", got)
	}
}

// Le contenu d'un message est `any` des deux côtés (requête comme historique) :
// chaîne d'ordinaire, blocs quand une image est de la partie.
func TestCanalTexteContentAccepteChaineEtBlocs(t *testing.T) {
	if got := canalTexteContent("bonjour"); got != "bonjour" {
		t.Fatalf("chaîne : %q", got)
	}
	blocs := []any{
		map[string]any{"type": "text", "text": "regarde "},
		map[string]any{"type": "image_url", "image_url": map[string]any{"url": "data:image/png;base64,…"}},
		map[string]any{"type": "text", "text": "cette image"},
	}
	if got := canalTexteContent(blocs); got != "regarde cette image" {
		t.Fatalf("blocs : %q", got)
	}
	if got := canalTexteContent(nil); got != "" {
		t.Fatalf("nil devait donner vide, obtenu %q", got)
	}
}

// La réponse arrive au format d'une complétion : l'outil peer de l'appelant n'a
// rien à adapter, il lit choices[0].message.content comme avant.
func TestEnvoiOpenAIRendLaFormeAttendueParLoutilPeer(t *testing.T) {
	w := httptest.NewRecorder()
	envoiOpenAI(w, "voici le journal", 4210)
	var out struct {
		Object string `json:"object"`
		Model  string `json:"model"`
		Choices []struct {
			Message struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			TotalTokens      int `json:"total_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("réponse illisible : %v (%s)", err, w.Body.String())
	}
	if out.Object != "chat.completion" || len(out.Choices) != 1 {
		t.Fatalf("forme inattendue : %s", w.Body.String())
	}
	if out.Choices[0].Message.Content != "voici le journal" {
		t.Fatalf("contenu rendu : %q", out.Choices[0].Message.Content)
	}
	// Le modèle annoncé n'est pas un gguf : c'est un agent, et l'appelant doit
	// pouvoir le lire (le vérificateur du patch s'appuie sur ce delta de contexte).
	if out.Model != "ajean-agent" {
		t.Fatalf("modèle annoncé : %q", out.Model)
	}
	if out.Usage.PromptTokens != 4210 || out.Usage.CompletionTokens != len("voici le journal") {
		t.Fatalf("usage incohérent : %+v", out.Usage)
	}
}

// L'outil peer coupé pendant un tour du canal : deux IA autonomes qui se relancent
// la même question tourneraient sans fin.
func TestOutilPeerCoupePendantUnTourDuCanal(t *testing.T) {
	canalTurnActive.Add(1)
	defer canalTurnActive.Add(-1)
	out := runPeerTool(map[string]any{"action": "list"})
	if !strings.HasPrefix(out, "[erreur]") {
		t.Fatalf("l'outil peer aurait dû être coupé, obtenu : %s", out)
	}
	if !strings.Contains(out, "boucle") {
		t.Fatalf("la coupure doit dire pourquoi (boucle) : %s", out)
	}
}
