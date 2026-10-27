package ajean

// parallel_peer.go — le CANAL, du côté qui parle.
//
// Le pont (parallel_bridge.go) est la porte : il prête le cerveau de cette
// machine à une autre. Ce fichier est l'inverse : il fait de CETTE machine une
// voisine qui FRAPPE à la porte d'une autre. Les deux bouts ensemble forment un
// canal entre deux IA — chacune garde son propre cerveau, sa mémoire, ses
// réglages, et elles s'échangent des messages.
//
// Trois règles, non négociables :
//
//  1. Rien n'est découvert tout seul. Un voisin, ça s'ajoute à la main
//     (`ajean peer add` ou la carte UI). Ajean ne balaie pas le réseau.
//  2. Rien ne s'exécute de l'autre côté, et rien ne s'exécute ici à cause de
//     l'autre. peerAsk n'envoie jamais de définitions d'outils : ce qui revient
//     est du texte simple. Le pont, de son côté, ne fait que proxifier vers un
//     modèle — un voisin ne peut donc pas faire tourner de commande ici.
//  3. Le canal a une mémoire. Les derniers échanges d'un voisin sont gardés et
//     renvoyés à chaque tour, sinon chaque message serait un premier message.
//
// Le canal fonctionne avec n'importe quel moteur du côté appelé : llama.cpp
// local, MoE Strata, API externe, GPU cloud. C'est le pont appelé qui prête son
// preset actif ; ici on ne suppose rien du tout, on parle à une URL OpenAI.

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	peersStateKey       = "parallel_peers"
	peerThreadKeyPrefix = "parallel_peer_thread:"
	peerThreadMax       = 12          // derniers échanges gardés par voisin
	// peerAskTimeout : 5 minutes allaient bien à une COMPLÉTION nue. Depuis la
	// révision 4, demander à un voisin peut déclencher chez lui un tour d'AGENT
	// complet : plusieurs passages par son modèle, plus l'exécution de ses outils
	// (lire un journal, lancer une commande, reposer un binaire). Sur un gros MoE
	// local, un tour qui diagnostique un moteur dépasse allègrement les 5 minutes —
	// et l'appelant perdrait le résultat d'un travail déjà payé. 20 minutes, avec
	// une deadline dans le contexte (peerHTTPClient n'a pas de timeout global).
	peerAskTimeout      = 20 * time.Minute
	peerProbeTimeout    = 12 * time.Second
)

// peerEndpoint : un voisin joignable. URL = la base des complétions, telle
// qu'on la collerait dans un preset « API Externe » (http://hote:port/v1).
type peerEndpoint struct {
	Name  string `json:"name"`
	URL   string `json:"url"`
	Key   string `json:"key"`
	Model string `json:"model,omitempty"` // vide = relevé sur le voisin
}

// peerThreadMessage : un message du canal, côté local comme côté voisin.
type peerThreadMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

func peerThreadKey(p peerEndpoint) string {
	h := sha256.Sum256([]byte(strings.ToLower(strings.TrimSpace(p.URL))))
	return peerThreadKeyPrefix + hex.EncodeToString(h[:8])
}

// loadPeers : les voisins déclarés. Une base abîmée ne doit pas empêcher
// l'application de démarrer : on renvoie vide et on le dit.
func loadPeers() []peerEndpoint {
	raw := getStr(bkState, peersStateKey)
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	var out []peerEndpoint
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil
	}
	return out
}

func savePeers(peers []peerEndpoint) error {
	if len(peers) == 0 {
		return putStr(bkState, peersStateKey, "")
	}
	b, err := json.Marshal(peers)
	if err != nil {
		return err
	}
	return putStr(bkState, peersStateKey, string(b))
}

// peerConfigured : y a-t-il au moins un voisin ? C'est ce qui décide si l'outil
// peer est annoncé au modèle — annoncer un outil sans voisin, c'est mentir.
func peerConfigured() bool { return len(loadPeers()) > 0 }

// peerFind : par nom, ou par index si le nom est un chiffre. Un nom unique se
// passe d'être saisi en entier : la correspondance accepte un début de nom.
func peerFind(name string) (peerEndpoint, bool) {
	peers := loadPeers()
	if len(peers) == 0 {
		return peerEndpoint{}, false
	}
	name = strings.TrimSpace(name)
	if name == "" {
		if len(peers) == 1 {
			return peers[0], true
		}
		return peerEndpoint{}, false
	}
	for _, p := range peers {
		if strings.EqualFold(p.Name, name) {
			return p, true
		}
	}
	for _, p := range peers {
		if strings.HasPrefix(strings.ToLower(p.Name), strings.ToLower(name)) {
			return p, true
		}
	}
	if n, err := strconv.Atoi(name); err == nil && n >= 1 && n <= len(peers) {
		return peers[n-1], true
	}
	return peerEndpoint{}, false
}

// peerAddDeclared : déclare un voisin. L'URL est validée avant d'être écrite —
// une URL sans schéma part en demande impossible et le diagnostic suivant est
// toujours obscur.
func peerAddDeclared(name, rawURL, key, model string) (peerEndpoint, error) {
	u := strings.TrimSpace(rawURL)
	if u == "" {
		return peerEndpoint{}, errors.New("url du voisin requise (http://192.168.1.71:18190/v1)")
	}
	if !strings.Contains(u, "://") {
		// On accepte « 192.168.1.71:18190/v1 » : c'est ce que l'on colle depuis
		// `ajean bridge status`, et le http:// manque est l'erreur la plus fréquente.
		u = "http://" + u
	}
	if !strings.HasPrefix(u, "http://") && !strings.HasPrefix(u, "https://") {
		return peerEndpoint{}, fmt.Errorf("url invalide : %s (http:// ou https:// attendu)", rawURL)
	}
	if !strings.Contains(strings.TrimPrefix(strings.TrimPrefix(u, "http://"), "https://"), "/") {
		u += "/v1" // le pont parle la surface OpenAI, sa racine est /v1
	}
	if _, err := http.NewRequest(http.MethodGet, completionsURL(u), nil); err != nil {
		return peerEndpoint{}, fmt.Errorf("url invalide : %s", err)
	}
	name = strings.TrimSpace(name)
	if name == "" {
		name = strings.TrimPrefix(strings.TrimPrefix(u, "http://"), "https://")
	}
	peers := loadPeers()
	p := peerEndpoint{Name: name, URL: strings.TrimRight(u, "/"), Key: strings.TrimSpace(key), Model: strings.TrimSpace(model)}
	for i, q := range peers {
		if strings.EqualFold(q.Name, p.Name) {
			peers[i] = p // même nom = on remplace, pas de doublon
			return p, savePeers(peers)
		}
	}
	peers = append(peers, p)
	return p, savePeers(peers)
}

func peerRemove(name string) (string, error) {
	peers := loadPeers()
	p, ok := peerFind(name)
	if !ok {
		return "", errors.New("voisin inconnu")
	}
	out := peers[:0]
	for _, q := range peers {
		if !strings.EqualFold(q.Name, p.Name) {
			out = append(out, q)
		}
	}
	if err := savePeers(out); err != nil {
		return "", err
	}
	_ = putStr(bkState, peerThreadKey(p), "") // le canal et son souvenir partent ensemble
	return p.Name, nil
}

// peerThread : le souvenir de ce canal (les derniers échanges, dans l'ordre).
func peerThread(p peerEndpoint) []peerThreadMessage {
	raw := getStr(bkState, peerThreadKey(p))
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	var out []peerThreadMessage
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil
	}
	return out
}

func savePeerThread(p peerEndpoint, thread []peerThreadMessage) error {
	if len(thread) > peerThreadMax {
		thread = thread[len(thread)-peerThreadMax:]
	}
	if len(thread) == 0 {
		return putStr(bkState, peerThreadKey(p), "")
	}
	b, err := json.Marshal(thread)
	if err != nil {
		return err
	}
	return putStr(bkState, peerThreadKey(p), string(b))
}

// peerForget : efface le souvenir sans effacer le voisin.
func peerForget(p peerEndpoint) error { return putStr(bkState, peerThreadKey(p), "") }

// peerProbe : ce que le voisin prête (modèles). Sert au status et à choisir le
// modèle quand le voisin ne l'a pas déclaré.
func peerProbe(ctx context.Context, p peerEndpoint) ([]string, error) {
	u, err := http.NewRequestWithContext(ctx, http.MethodGet, completionsBase(p.URL)+"/models", nil)
	if err != nil {
		return nil, err
	}
	if p.Key != "" {
		u.Header.Set("Authorization", "Bearer "+p.Key)
	}
	resp, err := peerHTTPClient.Do(u)
	if err != nil {
		return nil, peerDiagnose(p, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK {
		return nil, peerStatusError(p, resp.StatusCode, body)
	}
	var out struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("réponse inattendue du voisin %s : %s", p.Name, strings.TrimSpace(string(body)))
	}
	ids := make([]string, 0, len(out.Data))
	for _, d := range out.Data {
		if d.ID != "" {
			ids = append(ids, d.ID)
		}
	}
	return ids, nil
}

// completionsBase : la racine OpenAI d'une URL de voisin (…/v1).
func completionsBase(raw string) string {
	u := strings.TrimRight(strings.TrimSpace(raw), "/")
	if strings.HasSuffix(u, "/chat/completions") {
		return strings.TrimSuffix(u, "/chat/completions")
	}
	if strings.HasSuffix(u, "/v1") {
		return u
	}
	return u + "/v1"
}

// peerAsk : un tour du canal. Le message part avec la mémoire de l'échange,
// la réponse revient et entre dans cette mémoire.
func peerAsk(ctx context.Context, p peerEndpoint, message string) (string, error) {
	message = strings.TrimSpace(message)
	if message == "" {
		return "", errors.New("message vide")
	}
	model := p.Model
	if model == "" {
		if ids, err := peerProbe(ctx, p); err == nil && len(ids) > 0 {
			model = ids[0]
		} else {
			model = "ajean" // le pont impose de toute façon le modèle de son preset
		}
	}
	thread := append(peerThread(p), peerThreadMessage{Role: "user", Content: message})
	payload, _ := json.Marshal(map[string]any{"model": model, "messages": thread, "stream": false})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, completionsURL(p.URL), bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	// Le pont d'en face doit savoir que ça vient d'un Ajean, pas d'un simple
	// logiciel tiers : c'est ce qui lui autorise à déclarer au modèle prêté qui
	// il est et ce qu'il ne peut pas faire (parallel_bridge.go,
	// bridgeDeclareSelf). Sans cet en-tête, la complétion traverse telle quelle.
	req.Header.Set(canalHeader, "1")
	// Qui parle, et à quel saut de l'échange on en est. Depuis que le pont rend un
	// tour d'AGENT (parallel_bridge_agent.go), l'appelé a besoin de savoir qui il a
	// en face : c'est ce que porte la balise de son message. La profondeur, elle,
	// borne une chaîne de postes qui se relaieraient la demande.
	req.Header.Set(canalFromHeader, canalMachineNom())
	req.Header.Set(canalHopHeader, "1")
	if p.Key != "" {
		req.Header.Set("Authorization", "Bearer "+p.Key)
	}
	resp, err := peerHTTPClient.Do(req)
	if err != nil {
		return "", peerDiagnose(p, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return "", fmt.Errorf("lecture de la réponse du voisin %s : %w", p.Name, err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", peerStatusError(p, resp.StatusCode, body)
	}
	var out struct {
		Model string `json:"model"`
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return "", fmt.Errorf("réponse illisible du voisin %s : %s", p.Name, strings.TrimSpace(string(body)))
	}
	if len(out.Choices) == 0 {
		return "", fmt.Errorf("le voisin %s n'a rien répondu", p.Name)
	}
	reply := strings.TrimSpace(out.Choices[0].Message.Content)
	if reply == "" {
		return "", fmt.Errorf("le voisin %s a répondu vide", p.Name)
	}
	// Le souvenir s'écrit APRÈS la réponse : si l'appel échoue, le canal n'est
	// pas pollué d'un message orphelin.
	if err := savePeerThread(p, append(thread, peerThreadMessage{Role: "assistant", Content: reply})); err != nil {
		return reply, fmt.Errorf("réponse reçue mais le souvenir du canal n'a pas pu être écrit : %w", err)
	}
	// Balise aussi de ce côté-ci, mais sur la réponse RENVOYÉE à l'agent et non sur
	// celle gardée dans le souvenir. La décision du 2026-10-27 demande de pouvoir
	// distinguer, dans le chat, ce qui vient du canal d'une interaction extérieure :
	// chez l'appelé c'est la bulle qui porte la balise, chez l'appelant c'est le texte
	// que l'agent lit. Sans elle, ce que le voisin écrit et ce que j'écris se
	// ressemblent trait pour trait dans mon contexte — et depuis que le pont rend un
	// tour d'agent, la réponse du voisin ressemble à s'y méprendre à la mienne.
	// Le souvenir, lui, reste NU : il est rejoué au voisin à chaque tour, qui n'a pas
	// à lire dix fois qu'il se parle à lui-même.
	//
	// La balise dit AUSSI ce qui a répondu de l'autre côté. Un tour d'agent s'annonce
	// model:"ajean-agent" (envoiOpenAI), le modèle nu répond sous le nom de son gguf :
	// c'est suffisant pour étiqueter, sans inventer de protocole. Cette différence est
	// décisive pour celui qui lit — de l'agent, ou du texte de modèle nu ?
	facon := "modèle nu"
	if strings.TrimSpace(out.Model) == "ajean-agent" {
		facon = "agent"
	}
	return fmt.Sprintf("[Réponse du canal Ajean↔Ajean · voisin « %s » · il a répondu par son %s] %s",
		p.Name, facon, reply), nil
}

// peerHTTPClient : pas de timeout global — la deadline est dans le contexte,
// seule capable de couvrir une génération longue sans la couper au hasard.
var peerHTTPClient = &http.Client{}

// peerDiagnose : une erreur de transport traduite en cause actionnable. La
// cause n°1 d'un pont injoignable est le pare-feu de la machine qui prête.
func peerDiagnose(p peerEndpoint, err error) error {
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "no such host"):
		return fmt.Errorf("voisin %s : nom %s introuvable sur le réseau local", p.Name, p.URL)
	case strings.Contains(msg, "refused"):
		return fmt.Errorf("voisin %s : %s refuse la connexion — le pont est-il ouvert là-bas (ajean bridge on) et le bon port ?", p.Name, p.URL)
	case strings.Contains(msg, "timeout"), strings.Contains(msg, "deadline"):
		return fmt.Errorf("voisin %s : pas de réponse — pare-feu ou machine éteinte ? (%s)", p.Name, p.URL)
	default:
		return fmt.Errorf("voisin %s injoignable : %w", p.Name, err)
	}
}

// peerStatusError : un code HTTP du pont traduit. Ces messages-là viennent du
// pont lui-même, ils sont donc déjà en français et explicites.
func peerStatusError(p peerEndpoint, code int, body []byte) error {
	trimmed := strings.TrimSpace(string(body))
	switch code {
	case http.StatusUnauthorized:
		return fmt.Errorf("voisin %s : clé API refusée. La clé du canal doit être celle de la machine qui prête (ajean set-api-key là-bas, puis ajean peer add %s <url> <clé>)", p.Name, p.Name)
	case http.StatusServiceUnavailable:
		return fmt.Errorf("voisin %s : son pont refuse de servir — %s", p.Name, bridgeErrorOf(trimmed))
	case http.StatusConflict:
		return fmt.Errorf("voisin %s : %s", p.Name, bridgeErrorOf(trimmed))
	case http.StatusNotFound:
		return fmt.Errorf("voisin %s : %s répond mais ce n'est pas un pont Ajean (surface /v1/* attendue)", p.Name, p.URL)
	case http.StatusForbidden:
		// 403 chez un voisin qui a le pont agent (révision 4) : son agent est éteint
		// ou son pont n'autorise pas le tour. Le message du pont est déjà exact, on y
		// ajoute seulement ce que l'appelant peut faire, lui.
		return fmt.Errorf("voisin %s : %s — chez l'appelant : reformuler une demande que ton interlocuteur peut traiter avec le seul texte, ou lui demander d'ouvrir son agent (ajean bridge agent on)", p.Name, bridgeErrorOf(trimmed))
	case http.StatusLoopDetected:
		// 421 : le voisin a vu l'échange rebondir de poste en poste. La cause est
		// presque toujours du côté de l'appelant — il a posé chez un voisin une
		// demande que lui-même aurait pu traiter.
		return fmt.Errorf("voisin %s : la demande a déjà fait %s et personne au bout de la chaîne ne peut la traiter. C'est probablement CHEZ TOI qu'elle doit être traitée : la traiter avec tes outils plutôt que la relayer", p.Name, "deux sauts")
	default:
		return fmt.Errorf("voisin %s : statut %d — %s", p.Name, code, bridgeErrorOf(trimmed))
	}
}

// bridgeErrorOf : le champ error d'une réponse JSON du pont, ou le texte brut.
func bridgeErrorOf(raw string) string {
	var v struct {
		Error string `json:"error"`
	}
	if json.Unmarshal([]byte(raw), &v) == nil && strings.TrimSpace(v.Error) != "" {
		return v.Error
	}
	if raw == "" {
		return "aucun détail"
	}
	if len(raw) > 300 {
		return raw[:300] + "…"
	}
	return raw
}

// peerTool : l'outil vu par le modèle. Un seul outil, comme tracker : le modèle
// choisit l'action.
func peerTool() Tool {
	return Tool{
		Type: "function",
		Function: ToolFunction{
			Name: "peer",
			Description: "Talk to ANOTHER Ajean on this local network, through the Ajean↔Ajean bridge. Each side keeps its own brain: this is a channel between two AIs, not a borrowed model. What the peer can do depends on ITS OWN setting, and you cannot tell from here — so read its answer, do not assume. If its agent mode is ON, your message starts a real agent turn on that machine: it answers with its own tools, its own memory and its own project, so it CAN read its logs, run its commands and check its own files — and it never sees your disk, only what it writes back. If its agent mode is OFF, the bridge only lends its raw model: no tools, no disk, no shell there, and a claim of having run something would be confabulation. Either way nothing of yours is executed remotely and nothing of theirs is shared automatically. Every message crossing the channel is tagged in its text — yours arrives on their side as [Canal Ajean↔Ajean …], their answer reaches you as [Réponse du canal Ajean↔Ajean …] — so you can always tell a peer apart from a human speaking to you. Their agent may also refuse: a turn already running on their machine, their conversation « Jean » open, or a request relayed too many times. The last exchanges with that peer are kept and resent, so the conversation continues. Never call this tool to answer a message that itself came from the channel — it bounces the question back and the two of you loop. list: which peers exist. status: what a peer lends (models, whether it answers). ask: send a message and get the answer. forget: drop the shared history without removing the peer.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"action":  map[string]any{"type": "string", "enum": []string{"list", "status", "ask", "forget"}, "description": "Default list."},
					"peer":    map[string]any{"type": "string", "description": "Peer name (or its number from list). Omit when there is only one peer."},
					"message": map[string]any{"type": "string", "description": "What to say to that peer (ask)."},
				},
			},
		},
	}
}

// runPeerTool : l'exécution de l'outil peer.
func runPeerTool(args map[string]any) string {
	// Garde-fou anti-boucle. Depuis que le pont d'en face rend un tour d'AGENT et
	// non plus une complétion nue (parallel_bridge_agent.go), un tour déclenché par
	// le canal a le droit d'appeler ses outils — mais PAS l'outil peer : il
	// renverrait la demande à celui qui vient de la poser, et deux agents autonomes
	// se relaieraient la même question sans fin.
	if canalTurnActive.Load() > 0 {
		return canalPeerCoupé()
	}
	act, _ := args["action"].(string)
	who, _ := args["peer"].(string)
	act = strings.ToLower(strings.TrimSpace(act))
	if act == "" {
		act = "list"
	}
	peers := loadPeers()
	if act == "list" {
		if len(peers) == 0 {
			return "[ok] aucun voisin déclaré. Un canal se déclare à la main : ajean peer add <nom> <http://hote:18190/v1> <clé>, ou la carte « Pont Ajean ↔ Ajean »."
		}
		var b strings.Builder
		fmt.Fprintf(&b, "voisins déclarés (%d) — chacun a son propre cerveau, le canal ne fait que transporter le texte :\n", len(peers))
		for i, p := range peers {
			fmt.Fprintf(&b, "  %d. %s → %s", i+1, p.Name, p.URL)
			if p.Model != "" {
				fmt.Fprintf(&b, " (modèle déclaré : %s)", p.Model)
			}
			if n := len(peerThread(p)); n > 0 {
				fmt.Fprintf(&b, " · canal en cours, %d messages en souvenir", n)
			}
			b.WriteString("\n")
		}
		return strings.TrimRight(b.String(), "\n")
	}

	p, ok := peerFind(who)
	if !ok {
		if len(peers) == 0 {
			return "[erreur] aucun voisin déclaré : rien à interroger."
		}
		return fmt.Sprintf("[erreur] voisin introuvable (%s). Choisir parmi : %s", who, peerNames(peers))
	}
	switch act {
	case "status", "etat":
		ctx, cancel := context.WithTimeout(context.Background(), peerProbeTimeout)
		defer cancel()
		ids, err := peerProbe(ctx, p)
		if err != nil {
			return "[erreur] " + err.Error()
		}
		out := fmt.Sprintf("[ok] %s répond · modèles prêtés : %s", p.Name, strings.Join(ids, ", "))
		if n := len(peerThread(p)); n > 0 {
			out += fmt.Sprintf(" · %d messages en souvenir commun", n)
		}
		return out
	case "ask", "demander":
		msg, _ := args["message"].(string)
		if strings.TrimSpace(msg) == "" {
			return "[erreur] message vide — l'action ask exige message."
		}
		ctx, cancel := context.WithTimeout(context.Background(), peerAskTimeout)
		defer cancel()
		reply, err := peerAsk(ctx, p, msg)
		if err != nil {
			return "[erreur] " + err.Error()
		}
		return fmt.Sprintf("%s répond :\n%s", p.Name, reply)
	case "forget", "oublier":
		if err := peerForget(p); err != nil {
			return "[erreur] " + err.Error()
		}
		return fmt.Sprintf("[ok] souvenir du canal avec %s effacé (le voisin reste déclaré)", p.Name)
	default:
		return "[erreur] action inconnue : list, status, ask, forget"
	}
}

func peerNames(peers []peerEndpoint) string {
	names := make([]string, 0, len(peers))
	for _, p := range peers {
		names = append(names, p.Name)
	}
	return strings.Join(names, ", ")
}

// cmdParallelPeer : `ajean peer list | add <nom> <url> [clé] [modèle] | rm <nom> |
// status [nom] | ask <nom> <message> | forget [nom]`.
func cmdParallelPeer(args []string) error {
	sub := "list"
	if len(args) > 0 {
		sub = strings.ToLower(strings.TrimSpace(args[0]))
		args = args[1:]
	}
	switch sub {
	case "list", "etat":
		peers := loadPeers()
		if len(peers) == 0 {
			fmt.Printf("aucun voisin déclaré\n")
			fmt.Printf("  ajean peer add <nom> http://192.168.1.71:18190/v1 <cle-api>\n")
			return nil
		}
		for i, p := range peers {
			fmt.Printf("%d. %-14s %s", i+1, p.Name, p.URL)
			if p.Key != "" {
				fmt.Printf(" · clé posée")
			} else {
				fmt.Printf(" · SANS CLÉ")
			}
			if p.Model != "" {
				fmt.Printf(" · modèle %s", p.Model)
			}
			if n := len(peerThread(p)); n > 0 {
				fmt.Printf(" · canal %d messages", n)
			}
			fmt.Printf("\n")
		}
		return nil
	case "add", "ajouter":
		if len(args) < 2 {
			return errors.New("usage : ajean peer add <nom> <http://hote:port/v1> [cle] [modele]")
		}
		p, err := peerAddDeclared(args[0], args[1], argAt(args, 2), argAt(args, 3))
		if err != nil {
			return err
		}
		fmt.Printf("%s voisin %s : %s\n", green("[ok]"), p.Name, p.URL)
		if p.Key == "" {
			fmt.Printf("%s aucune clé : le pont de l'autre machine refusera la demande. ajean peer add %s %s <cle>\n", yellow("[!]"), p.Name, p.URL)
			return nil
		}
		ctx, cancel := context.WithTimeout(context.Background(), peerProbeTimeout)
		defer cancel()
		if ids, err := peerProbe(ctx, p); err != nil {
			fmt.Printf("%s %v\n", yellow("[!]"), err)
			fmt.Printf("             le voisin est déclaré malgré tout ; vérifier son pare-feu et l'état de son pont (ajean bridge status là-bas).\n")
		} else {
			fmt.Printf("             il prête : %s\n", strings.Join(ids, ", "))
		}
		return nil
	case "rm", "remove", "retirer":
		if len(args) < 1 {
			return errors.New("usage : ajean peer rm <nom>")
		}
		name, err := peerRemove(args[0])
		if err != nil {
			return err
		}
		fmt.Printf("%s voisin %s retiré (son souvenir de canal part avec lui)\n", green("[ok]"), name)
		return nil
	case "status", "joinable":
		p, ok := peerFind(argAt(args, 0))
		if !ok {
			return errors.New("voisin introuvable — ajean peer list")
		}
		ctx, cancel := context.WithTimeout(context.Background(), peerProbeTimeout)
		defer cancel()
		ids, err := peerProbe(ctx, p)
		if err != nil {
			return err
		}
		fmt.Printf("%s : %s\n", green("[ok]"), p.Name)
		fmt.Printf("  prête   : %s\n", strings.Join(ids, ", "))
		fmt.Printf("  canal   : %d messages en souvenir commun\n", len(peerThread(p)))
		return nil
	case "ask", "demander":
		if len(args) < 2 {
			return errors.New("usage : ajean peer ask <nom> <message>")
		}
		p, ok := peerFind(args[0])
		if !ok {
			return errors.New("voisin introuvable — ajean peer list")
		}
		ctx, cancel := context.WithTimeout(context.Background(), peerAskTimeout)
		defer cancel()
		reply, err := peerAsk(ctx, p, strings.Join(args[1:], " "))
		if err != nil {
			return err
		}
		fmt.Printf("%s\n", reply)
		return nil
	case "forget", "oublier":
		p, ok := peerFind(argAt(args, 0))
		if !ok {
			return errors.New("voisin introuvable — ajean peer list")
		}
		if err := peerForget(p); err != nil {
			return err
		}
		fmt.Printf("%s souvenir du canal avec %s effacé\n", green("[ok]"), p.Name)
		return nil
	default:
		return errors.New("usage : ajean peer [list|add <nom> <url> [cle] [modele]|rm <nom>|status [nom]|ask <nom> <message>|forget [nom]]")
	}
}

func argAt(args []string, i int) string {
	if i < len(args) {
		return strings.TrimSpace(args[i])
	}
	return ""
}

// handleParallelPeers : /api/peers. GET = la liste (la clé n'est JAMAIS renvoyée
// — seulement « posée ou non »), POST = ajouter / retirer / oublier.
// Réservé au build portable : sans pont, il n'y a pas de canal.
func handleParallelPeers(w http.ResponseWriter, r *http.Request) {
	if !parallelBuild() {
		sendJSON(w, http.StatusForbidden, map[string]any{
			"ok": false, "error": "canal réservé à la version portable de Cresius",
		})
		return
	}
	switch r.Method {
	case http.MethodGet:
		peers := loadPeers()
		out := make([]map[string]any, 0, len(peers))
		for _, p := range peers {
			out = append(out, map[string]any{
				"name": p.Name, "url": p.URL, "model": p.Model,
				"has_key": p.Key != "", "thread": len(peerThread(p)),
			})
		}
		sendJSON(w, http.StatusOK, map[string]any{"ok": true, "peers": out})
		return
	case http.MethodPost:
		var in struct {
			Add    string `json:"add"`
			URL    string `json:"url"`
			Key    string `json:"key"`
			Model  string `json:"model"`
			Remove string `json:"remove"`
			Forget string `json:"forget"`
		}
		raw, _ := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err := json.Unmarshal(raw, &in); err != nil {
			sendJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "JSON invalide"})
			return
		}
		switch {
		case strings.TrimSpace(in.Add) != "":
			p, err := peerAddDeclared(in.Add, in.URL, in.Key, in.Model)
			if err != nil {
				sendJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": err.Error()})
				return
			}
			sendJSON(w, http.StatusOK, map[string]any{"ok": true, "name": p.Name, "url": p.URL})
			return
		case strings.TrimSpace(in.Remove) != "":
			name, err := peerRemove(in.Remove)
			if err != nil {
				sendJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": err.Error()})
				return
			}
			sendJSON(w, http.StatusOK, map[string]any{"ok": true, "removed": name})
			return
		case strings.TrimSpace(in.Forget) != "":
			p, ok := peerFind(in.Forget)
			if !ok {
				sendJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "voisin inconnu"})
				return
			}
			if err := peerForget(p); err != nil {
				sendJSON(w, http.StatusInternalServerError, map[string]any{"ok": false, "error": err.Error()})
				return
			}
			sendJSON(w, http.StatusOK, map[string]any{"ok": true, "forgot": p.Name})
			return
		default:
			sendJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "add, remove ou forget attendu"})
			return
		}
	default:
		sendJSON(w, http.StatusMethodNotAllowed, map[string]any{"ok": false, "error": "GET ou POST"})
		return
	}
}
