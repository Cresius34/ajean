package ajean

// parallel_bridge_agent.go — le canal parle à l'AGENT, plus au modèle nu.
//
// CE QUI ÉTAIT FAUX AVANT (mesuré le 2026-10-27) : le pont proxifiait
// /v1/chat/completions vers le modèle brut du moteur (parallelBridgeHandler →
// oai.ServeHTTP). L'IA de l'autre côté ne recevait donc NI outils, NI mémoire, NI
// contexte de projet. Preuve : à qui demandait de recopier son propre journal
// strata-gyro-gyro-s.log, bekka répondait « INTROUVABLE — mon exécution est un
// service SaaS cloud, sans système de fichiers ». Elle ne pouvait pas diagnostiquer
// son propre moteur, et le crash Gyro-S restait impossible à examiner à distance.
//
// CE QUE FAIT CE FICHIER : quand la requête vient d'un AUTRE Ajean (en-tête du
// canal, posé par peerAsk), le pont ne prête plus son modèle. Il lance un VRAI tour
// d'agent local — le même que celui de l'onglet Projet — et rend la réponse de
// l'agent en forme de complétion OpenAI. En face, l'agent a ses outils, sa mémoire
// et son projet : il peut lire ses journaux, lancer une commande, vérifier un SHA.
//
// LE PARTAGE DE LA CONVERSATION EST ASSUMÉ, sur décision de l'utilisateur
// (2026-10-27) : « Pas spécialement problématique, si on inclus une balise en début
// de message dans vos échanges, comme ça vous pourrez faire la différence avec une
// interaction extérieure dans le chat ». Le tour du canal entre donc dans la
// conversation ACTIVE — celle que l'utilisateur voit — précédé de la balise, qui
// dit à l'agent, et dans la bulle, que ce message vient d'un Ajean voisin et non de
// l'utilisateur.
//
// Ce qui est refusé plutôt que subi :
//   - la conversation Jean : son fil est le journal de la mémoire, le trafic du
//     canal ne s'y mêle pas (409 — alignConvForMode y ferait NewSession, donc
//     jetterait la conversation de l'utilisateur) ;
//   - un tour déjà en cours : pas de mise en file. La file (issue #74) injecte le
//     message dans la réponse EN COURS de l'utilisateur ; un message venu du réseau
//     n'a rien à y faire ;
//   - le streaming : un tour d'agent appelle des outils, sa réponse se recompose
//     après chaque appel ;
//   - la récursion : pendant un tour déclenché par le canal, l'outil peer est coupé
//     (canalTurnActive). Deux IA autonomes qui se relancent la même question
//     tourneraient sans fin.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync/atomic"
	"time"
)

// canalFromHeader : la machine qui APPELLE, déclarée par peerAsk. L'appelé ne
// connaît pas le réseau de son voisin ; c'est ce nom qui entre dans la balise.
const canalFromHeader = "X-Ajean-Canal-From"

// canalHopHeader : profondeur de l'échange, posé à 1 par l'outil peer. Défense en
// profondeur contre une chaîne de postes qui se relaieraient la même demande : la
// coupure réelle de la récursion est canalTurnActive (l'outil peer est indisponible
// pendant un tour du canal), cette borne-là n'attrape que les clients qui fabriquent
// l'en-tête eux-mêmes.
const canalHopHeader = "X-Ajean-Canal-Hop"

const canalHopMax = 2

// canalBalise : la balise de début de message. Elle est dans le TEXTE et pas
// seulement dans un en-tête, parce que c'est ce que l'agent lit et ce que
// l'utilisateur voit dans sa bulle : « ce message-là vient du réseau ».
const canalBalise = "[Canal Ajean↔Ajean"

// canalTurnActive : 1 pendant un tour déclenché par le canal. Le gate de génération
// (c.Generating) est exclusif à l'échelle du process — un seul tour à la fois — donc
// ce drapeau ne peut pas mentir sur la période où il vaut 1.
var canalTurnActive atomic.Int32

func canalPeerCoupé() string {
	return "[erreur] outil peer indisponible : ce tour a lui-même été déclenché par le canal Ajean↔Ajean. " +
		"Le relancer d'ici renverrait la demande à celui qui vient de nous l'envoyer — boucle sans fin. " +
		"Traite la demande avec TES outils, chez toi."
}

// canalDemande : la requête mérite-t-elle un tour d'agent ? Quatre conditions.
// Uniquement le chemin des complétions de dialogue ; l'en-tête du canal (un
// logiciel tiers branché sur le pont garde son comportement habituel : le modèle
// prêté, rien de changé) ; le MODE AGENT ALLUMÉ sur ce poste (capsules du tour) ;
// et l'autorisation explicite du pont (parallel_bridge_agent, éteint par défaut).
// Ces deux derniers garde-fous sont de sécurité, pas des commodités : un tour
// d'agent met le shell et le disque de la machine à la portée de qui possède la clé
// API du pont. L'un ou l'autre éteint, le pont en reste au prêt du modèle nu avec sa
// carte d'identité (bridgeDeclareSelf, révision 3) — rien ne s'exécute.
func canalDemande(r *http.Request) bool {
	if r.Method != http.MethodPost || r.URL.Path != "/v1/chat/completions" {
		return false
	}
	if strings.TrimSpace(r.Header.Get(canalHeader)) == "" {
		return false
	}
	return parallelBridgeAgentOn() && agentEnabled()
}

// canalTexteContent : le contenu d'un message, en texte seul. Côté requête comme
// côté historique interne, Content est `any` : une chaîne d'ordinaire, ou un
// tableau de blocs {type:"text",text:…} quand une image est de la partie.
func canalTexteContent(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case []any:
		var b strings.Builder
		for _, blk := range t {
			m, ok := blk.(map[string]any)
			if !ok {
				continue
			}
			if s, ok := m["text"].(string); ok {
				b.WriteString(s)
			}
		}
		return b.String()
	}
	return ""
}

// canalMessageTexte : le message qui entre dans la conversation partagée. La balise
// est dans le TEXTE et non seulement dans un en-tête, sur la demande de l'utilisateur
// (2026-10-27) : c'est ce que l'agent lit, ce que l'utilisateur voit dans la bulle,
// et ce qui permet de faire la différence avec une interaction extérieure dans le chat.
func canalMessageTexte(qui, dernier string) string {
	if qui == "" {
		qui = "machine voisine"
	}
	return canalBalise + " · machine « " + qui + " »] Message d'un AUTRE Ajean, et non de l'utilisateur de ce " +
		"poste : il te parle d'égal à égal, sur le réseau local. Réponds-lui directement, avec tes outils habituels là " +
		"où la demande les concerne — il ne voit ni ton disque ni tes journaux : ce que tu lis chez toi ne lui parvient " +
		"que par ce que tu en écris.\n\n" + dernier
}

// bridgeAgentTurn : le tour d'agent déclenché par le voisin.
func bridgeAgentTurn(w http.ResponseWriter, r *http.Request) {
	// Ressasser l'autorisation ici, même si l'appelant (le pont) vient de la
	// vérifier : bridgeAgentTurn ne doit jamais devenir un exécuteur distant par
	// ce qu'un futur appelant aurait oublié.
	if !parallelBridgeAgentOn() || !agentEnabled() {
		sendJSON(w, http.StatusForbidden, map[string]any{
			"ok": false,
			"error": "le tour d'agent n'est pas autorisé sur ce poste : le pont ne prête que son modèle nu. " +
				"Sur la machine appelée : ajean bridge agent on, et le mode agent de l'interface allumé.",
		})
		return
	}
	if hop := strings.TrimSpace(r.Header.Get(canalHopHeader)); hop != "" {
		var n int
		if _, err := fmt.Sscanf(hop, "%d", &n); err == nil && n >= canalHopMax {
			sendJSON(w, http.StatusLoopDetected, map[string]any{
				"ok": false,
				"error": fmt.Sprintf("le canal en est déjà à son %dᵉ saut : personne de cette chaîne ne peut traiter "+
					"cette demande. C'est l'interlocuteur d'origine qui doit agir chez lui.", n),
			})
			return
		}
	}

	var corps struct {
		Stream   bool `json:"stream"`
		Messages []struct {
			Role    string `json:"role"`
			Content any    `json:"content"`
		} `json:"messages"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 8<<20)).Decode(&corps); err != nil {
		sendJSON(w, http.StatusBadRequest, map[string]any{
			"ok": false, "error": "corps de requête illisible : " + err.Error(),
		})
		return
	}
	dernier := ""
	for _, m := range corps.Messages {
		if strings.EqualFold(strings.TrimSpace(m.Role), "user") {
			dernier = strings.TrimSpace(canalTexteContent(m.Content))
		}
	}
	if dernier == "" {
		sendJSON(w, http.StatusBadRequest, map[string]any{
			"ok": false, "error": "aucun message utilisateur dans la requête : le canal n'a rien à traiter.",
		})
		return
	}
	if corps.Stream {
		sendJSON(w, http.StatusConflict, map[string]any{
			"ok": false,
			"error": "le tour d'agent du canal ne se streame pas : un tour d'agent appelle des outils et sa réponse se " +
				"recompose après chacun. Relancer sans \"stream\": true.",
		})
		return
	}

	// La conversation Jean est le journal de la mémoire personnelle de l'assistant :
	// le trafic du canal n'y entre pas.
	conv.mu.Lock()
	onJean := conv.ID == jeanConvID
	modeCourant := conv.Mode
	conv.mu.Unlock()
	if onJean {
		sendJSON(w, http.StatusConflict, map[string]any{
			"ok": false,
			"error": "la conversation « Jean » est active sur ce poste : le canal n'y entre pas (c'est le journal de la " +
				"mémoire). Basculer l'interface sur une conversation de projet pour parler au voisin.",
		})
		return
	}
	if conv.isGenerating() {
		sendJSON(w, http.StatusConflict, map[string]any{
			"ok": false,
			"error": "un tour est déjà en cours sur ce poste (l'utilisateur ou une tâche). Le message du canal n'est pas " +
				"mis en file : il s'intercalerait dans une réponse en cours. Réessayer une fois ce tour terminé.",
		})
		return
	}

	// Le mode de la conversation active, pas un mode imposé depuis le réseau : une
	// conversation garde le mode où elle a commencé (lockMode). Vide = projet.
	want := strings.TrimSpace(modeCourant)
	if want == "" {
		want = "project"
	}
	mode, fresh := conv.lockMode(want)
	var b chatReq
	applyChatMode(&b, mode)
	caps := capsFromBody(b)

	texte := canalMessageTexte(strings.TrimSpace(r.Header.Get(canalFromHeader)), dernier)

	// Repères AVANT le tour : tout ce que celui-ci ajoutera porte un Seq supérieur à
	// seq0, et les messages neufs commencent à l'index nbMsg0. Sans cette borne, la
	// recherche de la réponse finale remonterait jusqu'à une réponse d'un tour
	// antérieur quand l'agent n'a produit que des appels d'outils.
	conv.mu.Lock()
	seq0, nbMsg0, epoch := conv.Seq, len(conv.Messages), conv.epoch
	conv.mu.Unlock()

	// Le drapeau est posé AVANT StartTurn : celui-ci détache la génération
	// (go c.generate), donc l'agent peut appeler un outil avant la ligne suivante.
	// Posé après, la coupure de l'outil peer laissait une fenêtre de quelques
	// millisecondes où la boucle restait possible.
	canalTurnActive.Add(1)
	if err := conv.StartTurn(texte, nil, caps, 0); err != nil {
		canalTurnActive.Add(-1)
		if fresh {
			conv.unlockMode(mode)
		}
		code := http.StatusServiceUnavailable
		if errors.Is(err, ErrBusy) {
			code = http.StatusConflict
		}
		sendJSON(w, code, map[string]any{"ok": false, "error": "le tour du canal n'a pas pu démarrer : " + err.Error()})
		return
	}
	defer canalTurnActive.Add(-1)

	// Le contexte de la requête suit la connexion : si le voisin raccroche, on cesse
	// d'attendre. Le tour, lui, continue — c'est déjà le comportement d'un tour lancé
	// depuis le navigateur (la génération est détachée du navigateur).
	fausse, err := conv.waitCanalTurn(r.Context(), epoch, seq0)
	if err != nil {
		if r.Context().Err() != nil {
			return // appelant parti : rien à écrire sur une réponse refermée
		}
		sendJSON(w, http.StatusBadGateway, map[string]any{"ok": false, "error": err.Error()})
		return
	}

	reponse := conv.derniereReponseApres(nbMsg0)
	if strings.TrimSpace(reponse) == "" {
		msg := "l'agent de ce poste a terminé son tour sans répondre de texte : ses messages sont tous passés par des " +
			"outils. Demander une réponse écrite."
		if fausse != "" {
			msg = "le tour de l'agent a rencontré une erreur sur ce poste : " + fausse
		}
		sendJSON(w, http.StatusBadGateway, map[string]any{"ok": false, "error": msg})
		return
	}

	conv.mu.Lock()
	ctxUsed := conv.CtxUsed
	conv.mu.Unlock()
	envoiOpenAI(w, reponse, ctxUsed)
}

// waitCanalTurn : attend la fin du tour, signalée par turn_done dans le journal
// d'abonnement. Même mécanisme que SubscribeTail : verrou tenu par boucle, réveil
// par cond, et un goroutine qui Broadcast à la fermeture du contexte — sans quoi un
// abonné endormi ne se réveillerait jamais quand la connexion du voisin se ferme.
//
// Une delta « error » n'EST PAS une sortie : le tour continue après une erreur
// d'appel d'outil (l'agent peut rater puis réussir). On la retient seulement, pour
// l'expliquer si la réponse finale se trouve vide. turn_done, lui, arrive toujours :
// le defer de generate le pose sur toutes les branches, y compris l'erreur.
func (c *Conversation) waitCanalTurn(ctx context.Context, epoch, afterSeq int) (derniereErreur string, err error) {
	go func() {
		<-ctx.Done()
		c.mu.Lock()
		c.cond.Broadcast()
		c.mu.Unlock()
	}()

	c.mu.Lock()
	defer c.mu.Unlock()
	for {
		if ctx.Err() != nil {
			return derniereErreur, ctx.Err()
		}
		if c.epoch != epoch {
			return derniereErreur, errors.New("la conversation de ce poste a été réinitialisée pendant le tour du canal")
		}
		i := sort.Search(len(c.Log), func(i int) bool { return c.Log[i].Seq > afterSeq })
		for _, ev := range c.Log[i:] {
			if e, ok := ev.Delta["error"]; ok {
				derniereErreur = fmt.Sprint(e)
			}
			if _, ok := ev.Delta["turn_done"]; ok {
				return derniereErreur, nil
			}
		}
		c.cond.Wait()
	}
}

// derniereReponseApres : la réponse écrite du tour, à partir de l'index n.
// On ne collecte PAS les deltas du journal : entre deux appels d'outils l'agent
// raconte ce qu'il fait, et la compaction de fin de tour fusionne ces événements
// (Seq redécalé) — un lecteur resté en dessous ne les reverrait pas. Le message
// assistant ajouté à la vue modèle en fin de tour (generate) est, lui, l'unique
// réponse finale : c'est exactement ce que le canal doit rendre.
func (c *Conversation) derniereReponseApres(n int) string {
	c.mu.Lock()
	defer c.mu.Unlock()
	for i := len(c.Messages) - 1; i >= n && i >= 0; i-- {
		m := c.Messages[i]
		if m.Role != "assistant" {
			continue
		}
		// Les messages « assistant » porteurs d'un tool_call ont un contenu vide :
		// ils ne comptent pas, on continue vers l'arrière.
		if s := strings.TrimSpace(canalTexteContent(m.Content)); s != "" {
			return s
		}
	}
	return ""
}

// envoiOpenAI : la réponse au format d'une complétion, pour que l'outil peer de
// l'appelant n'ait rien à adapter.
func envoiOpenAI(w http.ResponseWriter, reponse string, promptTokens int) {
	nb := len([]rune(reponse))
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"id":      "chatcmpl-pont-" + fmt.Sprint(time.Now().UnixNano()),
		"object":  "chat.completion",
		"created": time.Now().Unix(),
		"model":   "ajean-agent",
		"choices": []any{map[string]any{
			"index":         0,
			"message":       map[string]any{"role": "assistant", "content": reponse},
			"finish_reason": "stop",
		}},
		// Le tour d'agent ne passe pas par le compteur de llama.cpp : le contexte
		// réel mesuré du tour (CtxUsed) tient lieu de prompt_tokens. C'est la
		// grandeur honnête, et c'est elle que le vérificateur du patch compare pour
		// prouver que le contexte injecté part bien.
		"usage": map[string]any{
			"prompt_tokens":     promptTokens,
			"completion_tokens": nb,
			"total_tokens":      promptTokens + nb,
		},
	})
}

// canalMachineNom : le nom de CETTE machine, pour l'en-tête d'appel. os.Hostname()
// et non machineID() : ce dernier est un hexadécimal taillé pour le relais
// ajean.link, illisible dans une balise que deux IA doivent se citer.
func canalMachineNom() string {
	if h, err := os.Hostname(); err == nil {
		if s := strings.TrimSpace(h); s != "" {
			return s
		}
	}
	return "cette machine"
}
