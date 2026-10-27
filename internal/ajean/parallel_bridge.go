package ajean

// parallel_bridge.go — le pont Ajean↔Ajean (niveau 0).
//
// But : deux machines, chacune sous Ajean, se parlent. Celle qui prête son
// cerveau expose une surface OpenAI sur le réseau local ; l'autre la déclare en
// preset « externe » et discute avec elle. L'amont sait déjà faire les deux
// morceaux — oaiHandler proxifie /v1/* vers le moteur (llm_oai.go:46), le preset
// externe consomme n'importe quelle API OpenAI-compatible (backend_external.go)
// — mais sur notre build parallèle le front n'était atteignable QUE par le
// tunnel ajean.link, lui-même coupé (parallel_profile.go:125 et
// parallelCommandAllowed). D'où ce listener dédié, indépendant de l'interface.
//
// Choix de sécurité, dans cet ordre :
//   - le pont est ÉTEINT par défaut : ouvrir un port sur le réseau local est un
//     exposé réel, cela se décide, jamais cela ne s'impose ;
//   - il ne sert QUE /v1/* : ni /api/* (pilotage, mémoire, fichiers, agent), ni
//     les pages. Toute l'interface reste en boucle locale ;
//   - la clé API de la machine est exigée sans exception. antKeyOK admet la
//     boucle locale sans clé (c'est le contrat de /v1/messages) ; ici une
//     requête venant d'ailleurs est refusée, et sans clé configurée le pont
//     refuse de démarrer — fail-closed ;
//   - derrière, le moteur valide la même clé (--api-key,
//     backend_moe_strata.go:225) : deux contrôles indépendants, pas un seul
//     qu'on oublie.
//
// Ce que le pont ne fait PAS : il ne partage ni la mémoire, ni les définitions
// d'outils, ni le fil de conversation de l'utilisateur.
//
// Ce qu'il fait depuis la révision 4 : quand l'appelant est un AUTRE Ajean
// (en-tête du canal) et que le MODE AGENT est allumé sur ce poste, il ne prête plus
// son modèle nu — il rend un vrai tour d'agent local, dont la réponse entre dans la
// conversation active précédée d'une balise. Voir parallel_bridge_agent.go : c'est
// ce qui permet au voisin de lire SES propres journaux et de diagnostiquer SON
// moteur, ce que le modèle nu lui interdisait (il répondait « INTROUVABLE, mon
// exécution est un SaaS cloud »). Agent éteint, on en reste au prêt du modèle nu
// décrit ci-dessus : l'interrupteur de la machine décide seul de prêter ou non son
// shell à un voisin.
//
// Niveau 0 de l'étude (mémoire r19-faisabilite-ajean-a-ajean) : le partage de la
// mémoire elle-même et le serveur MCP restent à faire.

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

const (
	parallelBridgeFlagKey    = "parallel_bridge"      // "1" = pont demandé
	parallelBridgePortKey    = "parallel_bridge_port" // écoute du pont, défaut 18190
	parallelBridgePortDef    = "18190"
	parallelBridgeHostEnvKey = "AJEAN_BRIDGE_HOST" // épingler une interface
	// parallelBridgeAgentKey : autoriser le canal à déclencher un tour d'AGENT
	// local (shell, disque, mémoire). ÉTEINT par défaut, volontairement.
	parallelBridgeAgentKey = "parallel_bridge_agent"
)

var (
	parallelBridgeMu sync.Mutex
	parallelBridgeLn net.Listener
)

func parallelBridgeRequested() bool { return getBool(bkState, parallelBridgeFlagKey) }

func setParallelBridgeRequested(on bool) error { return putBool(bkState, parallelBridgeFlagKey, on) }

// parallelBridgeAgentOn : le canal a-t-il droit à un tour d'agent, et pas seulement
// à une complétion du modèle prêté. ÉTEINT par défaut : ouvrir un port pour prêter
// un cerveau est une chose, y faire tourner le shell de la machine en est une autre,
// et cela se décide machine par machine. AJEAN_BRIDGE_AGENT=1 l'allume pour un
// lanceur ou un test, sans écrire dans la base.
func parallelBridgeAgentOn() bool {
	if v := strings.TrimSpace(os.Getenv("AJEAN_BRIDGE_AGENT")); v != "" {
		return v == "1" || strings.EqualFold(v, "true") || strings.EqualFold(v, "on")
	}
	return getBool(bkState, parallelBridgeAgentKey)
}

func setParallelBridgeAgentOn(on bool) error { return putBool(bkState, parallelBridgeAgentKey, on) }

// parallelBridgeEnvForced : un lanceur peut allumer le pont sans toucher à la
// base (AJEAN_BRIDGE=1). C'est ce qui permet de tester un pont sans repasser par
// l'interface, et surtout sans que l'installation d'à côté hérite du réglage.
func parallelBridgeEnvForced() bool {
	v := strings.TrimSpace(os.Getenv("AJEAN_BRIDGE"))
	return v == "1" || strings.EqualFold(v, "true") || strings.EqualFold(v, "on")
}

func parallelBridgeEnabled() bool { return parallelBridgeRequested() || parallelBridgeEnvForced() }

// parallelBridgePort : port d'écoute du pont. Volant, comme l'interface et le
// moteur, pour que deux instances de la même machine puissent chacune avoir le
// leur (c'était le blocage principal de l'étude : tout était en dur).
func parallelBridgePort() string {
	p := strings.TrimSpace(getStr(bkState, parallelBridgePortKey))
	if n, err := strconv.Atoi(p); err == nil && n >= 1025 && n <= 65534 {
		return strconv.Itoa(n)
	}
	if p = strings.TrimSpace(os.Getenv("AJEAN_BRIDGE_PORT")); p != "" {
		if n, err := strconv.Atoi(p); err == nil && n >= 1025 && n <= 65534 {
			return strconv.Itoa(n)
		}
	}
	return parallelBridgePortDef
}

// parallelBridgeHost : 0.0.0.0 par défaut (le pont sert le réseau local),
// réductible à une interface précise quand la machine en a plusieurs.
func parallelBridgeHost() string {
	h := strings.TrimSpace(os.Getenv(parallelBridgeHostEnvKey))
	if h == "" {
		return hostAllInterfaces
	}
	if net.ParseIP(h) == nil {
		return hostAllInterfaces
	}
	return h
}

// parallelBridgeRunning : le pont écoute-t-il à l'instant dans CE process.
func parallelBridgeRunning() bool {
	parallelBridgeMu.Lock()
	defer parallelBridgeMu.Unlock()
	return parallelBridgeLn != nil
}

// parallelBridgeListening : y a-t-il quelqu'un sur le port du pont, quel que
// soit le process ? C'est ce que voit une commande lancée à côté de l'instance
// en cours : la variable du listener ne vit que dans le process qui l'a ouverte.
func parallelBridgeListening() bool {
	if parallelBridgeRunning() {
		return true
	}
	c, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", parallelBridgePort()), 400*time.Millisecond)
	if err != nil {
		return false
	}
	_ = c.Close()
	return true
}

// parallelBridgeHandler : la surface OpenAI, bornée à /v1/* et verrouillée par
// la clé de la machine.
//
// Le point important : le pont ne doit PAS être collé à un llama-server local.
// Ce que la machine prête, c'est le preset ACTIF, exactement comme le chat local
// le voit — resolveChatEndpoint() est le seul arbitre. Un pont branché sur le
// seul port du moteur échouerait dès que la machine prête un modèle distant
// (preset « API Externe », GPU cloud) ou un moteur qui n'écoute pas là où on
// l'attend. Anthropic passe déjà par là (anthropicHandler → resolveChatEndpoint).
func parallelBridgeHandler() http.Handler {
	oai := oaiHandler()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/v1/") {
			sendJSON(w, http.StatusNotFound, map[string]any{
				"ok": false, "error": "pont Ajean : surface OpenAI uniquement (/v1/*)",
			})
			return
		}
		if readAPIKey() == "" {
			sendJSON(w, http.StatusServiceUnavailable, map[string]any{
				"ok":    false,
				"error": "aucune clé API sur cette machine : le pont refuserait n'importe quel client. Définir une clé (page Réglages, section clé API) puis réessayer.",
			})
			return
		}
		if !antKeyOK(r) {
			w.Header().Set("WWW-Authenticate", `Bearer realm="ajean-pont"`)
			sendJSON(w, http.StatusUnauthorized, map[string]any{"ok": false, "error": "clé API invalide"})
			return
		}
		// Le canal d'abord : quand l'appelant est un AUTRE Ajean, ce n'est pas notre
		// modèle qu'il cherche, c'est notre agent. Le pont lui rend donc un vrai tour
		// d'agent local (outils, mémoire, projet) au lieu de proxifier une complétion
		// vers le moteur nu — d'où ses réponses « INTROUVABLE » sur ses propres
		// journaux avant ce correctif. Voir parallel_bridge_agent.go.
		if canalDemande(r) {
			bridgeAgentTurn(w, r)
			return
		}
		// Le canal appelle, mais le tour d'agent n'est pas autorisé ici : on rend le
		// modèle nu, et on le DIT. Sans cet en-tête, l'appelant ne peut pas savoir si
		// c'est un choix ou un interrupteur oublié chez nous — mesuré sur le poste
		// voisin le 2026-10-27, où tout le monde cherchait « pourquoi modèle nu ? »
		// pendant que la réponse était un interrupteur éteint de ce côté-ci.
		if strings.TrimSpace(r.Header.Get(canalHeader)) != "" {
			if refus := canalAgentRefus(); refus != "" {
				w.Header().Set(canalAgentRefusHeader, refus)
			}
		}
		if ep := resolveChatEndpoint(); (ep.External || ep.Cloud) && !isAnthropicPath(r.URL.Path) {
			if bridgePointsToItself(ep) {
				sendJSON(w, http.StatusConflict, map[string]any{
					"ok": false,
					"error": fmt.Sprintf("le preset prêté (%s) pointe sur ce pont-ci : boucle sans fin. Deux Ajean sur le même PC doivent prêter un moteur à l'autre, pas se prêter eux-mêmes.", ep.URL),
				})
				return
			}
			bridgeDeclareSelf(r)
			bridgeRemoteHandler(ep).ServeHTTP(w, r)
			return
		}
		bridgeDeclareSelf(r)
		oai.ServeHTTP(w, r)
	})
}

// canalHeader : marqueur posé par l'outil peer (parallel_peer.go) sur une
// complétion qui vient d'UN AUTRE Ajean. Il distingue le canal d'un simple
// logiciel tiers branché sur le pont — et c'est ce qui autorise le pont à
// parler de lui-même dans la requête.
const canalHeader = "X-Ajean-Canal"

// bridgeDeclareSelf : le modèle prêté reçoit le texte brut, sans prompt système.
// Sans rien pour le situer, il invente ce qu'il est. Mesuré le 2026-10-27, à qui
// demandait « as-tu un outil peer chez toi ? » : « je fonctionne sur une
// infrastructure cloud, pas sur un poste physique », puis « le voisin est ajouté
// sur le pont 18190 » — alors qu'aucun outil d'ajout n'existe. Deux réponses
// fausses, fabriquées faute de contexte.
//
// Le pont corrige donc ce qu'il sait être vrai, et RIEN D'AUTRE : quelle machine
// prête, que l'interlocuteur est une autre instance d'Ajean, et surtout que par
// le canal il n'a ni outil, ni disque, ni shell. Un modèle qui sait qu'il n'a pas
// d'outils cesse d'affirmer qu'il les a utilisés.
//
// Trois garde-fous : uniquement si l'en-tête du canal est présent (un client
// tiers générique garde sa requête telle quelle), uniquement si le chemin est
// bien celui des complétions de dialogue, et JAMAIS si l'appelant a déjà posé un
// prompt système — sa volonté passe avant la nôtre.
func bridgeDeclareSelf(r *http.Request) {
	if r.Method != http.MethodPost || r.URL.Path != "/v1/chat/completions" {
		return
	}
	if strings.TrimSpace(r.Header.Get(canalHeader)) == "" {
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 64<<20))
	if err != nil {
		return
	}
	var payload map[string]any
	if json.Unmarshal(body, &payload) != nil {
		r.Body = io.NopCloser(bytes.NewReader(body)) // corps illisible : on le rend tel quel
		return
	}
	msgs, _ := payload["messages"].([]any)
	for _, m := range msgs {
		mm, _ := m.(map[string]any)
		role := strings.ToLower(strings.TrimSpace(fmt.Sprint(mm["role"])))
		// system, developer, ou tout rôle non user/assistant déjà en tête :
		// l'appelant a une volonté explicite sur l'identité, on ne se substitue pas à lui.
		if role == "system" || role == "developer" || role == "instruction" {
			r.Body = io.NopCloser(bytes.NewReader(body))
			return
		}
	}
	msgs = append([]any{map[string]any{
		"role":    "system",
		"content": bridgeIdentityNote(),
	}}, msgs...)
	payload["messages"] = msgs
	out, err := json.Marshal(payload)
	if err != nil {
		r.Body = io.NopCloser(bytes.NewReader(body))
		return
	}
	r.Body = io.NopCloser(bytes.NewReader(out))
	r.ContentLength = int64(len(out))
	// Volontairement PAS d'en-tête Content-Length posé sur r.Header : le proxy et
	// le transport le déduisent de r.ContentLength, et l'écrire ici risque un
	// doublon à la sortie.
}

// bridgeIdentityNote : la carte d'identité du poste qui prête. Volontairement
// courte et négative : ce qui compte n'est pas ce que le modèle prêté est, c'est
// ce qu'il ne peut pas faire depuis le canal.
func bridgeIdentityNote() string {
	// os.Hostname() et non machineID() : ce dernier est un hexadécimal aléatoire
	// taillé pour le relais ajean.link, illisible pour une IA qui doit nommer la
	// machine où elle répond.
	host := ""
	if h, err := os.Hostname(); err == nil {
		host = strings.TrimSpace(h)
	}
	if host == "" {
		host = "cette machine"
	}
	return fmt.Sprintf(
		"Tu réponds par le pont Ajean↔Ajean de la machine « %s » (Ajean %s), sur le réseau local. "+
			"L'interlocuteur qui t'écrit est une AUTRE instance d'Ajean, avec son propre cerveau et sa propre mémoire : vous vous parlez d'égal à égal, tu n'es pas son sous-programme.\n"+
			"Dans ce contexte précis, tu n'as AUCUN outil : pas de shell, pas de disque, pas de navigateur, pas de mémoire persistante, pas de moteur de recherche. Seul du texte traverse le pont. "+
			"Ne prétends donc jamais avoir exécuté, lu ou écrit quoi que ce soit ici : si on te demande une action, dis en une phrase qu'elle est du ressort de l'interlocuteur, qui l'exécutera chez lui.\n"+
			"Ne devine pas non plus ton environnement (dossier d'installation, système d'exploitation, matériel) : tu ne le vois pas. Réponds dans la langue de l'interlocuteur.",
		host, displayVersion())
}

// bridgePointsToItself : le preset prêté est-ce ce pont ? Le piège est classique
// quand on fait dialoguer deux Ajean sur le même PC : le preset « externe » colle
// l'URL du pont local, et chaque appel se renvoie à lui-même jusqu'à saturer les
// sockets. Mieux vaut un refus lisible qu'un 502 incompréhensible.
func bridgePointsToItself(ep chatEndpoint) bool {
	u, err := url.Parse(completionsURL(ep.URL))
	if err != nil {
		return false
	}
	port := u.Port()
	if port == "" {
		port = "80"
	}
	if port != parallelBridgePort() {
		return false
	}
	host := u.Hostname()
	if host == "" || host == "localhost" || host == "::1" {
		return true
	}
	if ip := net.ParseIP(host); ip != nil && ip.IsUnspecified() {
		return true
	}
	// Une des adresses de cette machine (celles que le pont annonce lui-même).
	for _, raw := range parallelBridgeLanURLs() {
		if lu, err := url.Parse(raw); err == nil && lu.Hostname() == host {
			return true
		}
	}
	return false
}

// bridgeRemoteHandler : le preset prêté est un endpoint distant. Le pont rejoue
// alors ce que fait le chat local : il forwarding vers CETTE URL avec CETTE clé,
// en imposant le modèle du preset. Le client du réseau local n'a donc pas à
// connaître l'identifiant exact du modèle prêté — il envoie ce qu'il veut, la
// machine qui prête décide qui répond.
func bridgeRemoteHandler(ep chatEndpoint) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/models":
			bridgeRemoteModels(w, r, ep)
			return
		case "/v1/chat/completions", "/v1/completions", "/v1/embeddings", "/v1/rerank":
			bridgeRemoteForward(w, r, ep)
			return
		default:
			sendJSON(w, http.StatusNotFound, map[string]any{
				"ok": false,
				"error": "pont Ajean : le modèle prêté est une API distante, qui n'expose que /v1/models et /v1/chat/completions",
			})
			return
		}
	})
}

// bridgeRemoteBase : la racine OpenAI de l'endpoint prêté, depuis l'URL des
// complétions (…/v1/chat/completions → …/v1).
func bridgeRemoteBase(ep chatEndpoint) string {
	u, err := url.Parse(completionsURL(ep.URL))
	if err != nil {
		return ""
	}
	return strings.TrimSuffix(u.Path, "/chat/completions")
}

func bridgeRemoteModels(w http.ResponseWriter, r *http.Request, ep chatEndpoint) {
	base := bridgeRemoteBase(ep)
	if base != "" {
		ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
		defer cancel()
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/models", nil)
		if err == nil {
			if ep.Key != "" {
				req.Header.Set("Authorization", "Bearer "+ep.Key)
			}
			client := &http.Client{Timeout: 8 * time.Second}
			if resp, err := client.Do(req); err == nil {
				defer resp.Body.Close()
				if resp.StatusCode == http.StatusOK {
					w.Header().Set("Content-Type", "application/json")
					_, _ = io.Copy(w, io.LimitReader(resp.Body, 1<<20))
					return
				}
			}
		}
	}
	// L'endpoint ne expose pas /v1/models (ou n'y répond pas) : on annonce quand
	// même le modèle prêté, sinon l'autre Ajean croirait à un pont vide.
	sendJSON(w, http.StatusOK, map[string]any{
		"object": "list",
		"data":   []map[string]string{{"id": ep.Model, "object": "model"}},
	})
}

func bridgeRemoteForward(w http.ResponseWriter, r *http.Request, ep chatEndpoint) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 64<<20))
	if err != nil {
		sendJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "corps illisible"})
		return
	}
	// Le modèle du preset gagne toujours : c'est la machine qui prête qui choisit
	// qui répond, et ça évite un « model not found » côté distant.
	if ep.Model != "" {
		var payload map[string]any
		if json.Unmarshal(body, &payload) == nil {
			payload["model"] = ep.Model
			if b, err := json.Marshal(payload); err == nil {
				body = b
			}
		}
	}
	target := completionsURL(ep.URL)
	// /v1/completions et /v1/embeddings gardent leur bout de chemin.
	if r.URL.Path != "/v1/chat/completions" {
		if base := bridgeRemoteBase(ep); base != "" {
			target = strings.TrimSuffix(base, "/v1") + r.URL.Path
		}
	}
	req, err := http.NewRequestWithContext(r.Context(), http.MethodPost, target, bytes.NewReader(body))
	if err != nil {
		sendJSON(w, http.StatusInternalServerError, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	req.Header.Set("Content-Type", "application/json")
	if accept := r.Header.Get("Accept"); accept != "" {
		req.Header.Set("Accept", accept)
	}
	if ep.Key != "" {
		req.Header.Set("Authorization", "Bearer "+ep.Key)
	}
	resp, err := bridgeHTTPClient.Do(req)
	if err != nil {
		sendJSON(w, http.StatusBadGateway, map[string]any{
			"ok": false, "error": "modèle prêté injoignable : " + err.Error(),
		})
		return
	}
	defer resp.Body.Close()
	for k, vs := range resp.Header {
		for _, v := range vs {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(w, resp.Body) // streaming : io.Copy recopie au fil de l'eau
}

// bridgeHTTPClient : pas de timeout global — une complétion longue ou un flux
// SSE doit vivre aussi longtemps que la réponse. Le contexte de la requête est
// la seule échéance.
var bridgeHTTPClient = &http.Client{}

// startParallelBridge ouvre le listener et sert le front. Non bloquant : le
// serveur HTTP tourne dans sa goroutine. Un port déjà pris échoue sans rien
// arrêter d'autre — règle du build parallèle : on ne tue pas un port.
func startParallelBridge() error {
	parallelBridgeMu.Lock()
	defer parallelBridgeMu.Unlock()
	if parallelBridgeLn != nil {
		return nil
	}
	if !parallelBridgeEnabled() {
		return errors.New("pont désactivé")
	}
	if readAPIKey() == "" {
		return errors.New("le pont exige une clé API machine (page Réglages, section clé API)")
	}
	port := parallelBridgePort()
	if err := checkParallelPortFree(port); err != nil {
		return err
	}
	ln, err := net.Listen("tcp", net.JoinHostPort(parallelBridgeHost(), port))
	if err != nil {
		return fmt.Errorf("port du pont %s occupé ; aucun processus arrêté : %w", port, err)
	}
	srv := &http.Server{
		Handler:           parallelBridgeHandler(),
		ReadHeaderTimeout: 10 * time.Second,
		// Complétions longues et streaming SSE : aucun WriteTimeout.
	}
	parallelBridgeLn = ln
	go func() {
		if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			fmt.Fprintf(os.Stderr, "[pont ajean] %v\n", err)
		}
	}()
	n, _ := strconv.Atoi(port)
	fw := "inconnu"
	if !firewallInert {
		// Le pont écoute sur le réseau local : sans règle, Windows laisse rien
		// passer. Échec attendu sans droits administrateur — on le dit, on ne
		// fait pas semblant d'avoir ouvert.
		if err := firewallOpen(n); err != nil {
			fw = "ferme"
			fmt.Printf("[pont ajean] %s\n", firewallManualHint(n))
		} else {
			fw = "ouvert"
		}
	}
	fmt.Printf("[pont ajean] surface OpenAI sur http://%s/v1 (pare-feu : %s)\n", net.JoinHostPort(parallelBridgeHost(), port), fw)
	return nil
}

func stopParallelBridge() error {
	parallelBridgeMu.Lock()
	defer parallelBridgeMu.Unlock()
	if parallelBridgeLn == nil {
		return nil
	}
	err := parallelBridgeLn.Close()
	parallelBridgeLn = nil
	if !firewallInert {
		if n, conv := strconv.Atoi(parallelBridgePort()); conv == nil {
			_ = firewallClose(n)
		}
	}
	return err
}

// parallelBridgeReconcile aligne l'écoute réelle sur le réglage enregistré. Un
// seul endroit pour cette règle, appelée par la veille et par rien d'autre : ce
// qui demande doit écouter, ce qui n'est plus demandé doit se taire.
func parallelBridgeReconcile() {
	if parallelBridgeEnabled() {
		if !parallelBridgeRunning() {
			if err := startParallelBridge(); err != nil {
				fmt.Fprintf(os.Stderr, "[pont ajean] réveil refusé : %v\n", err)
			}
		}
		return
	}
	if parallelBridgeRunning() {
		_ = stopParallelBridge()
	}
}

// parallelBridgeWatch rattache l'état écouté au réglage enregistré. Sans lui,
// `ajean bridge on` lancé à côté de l'instance en cours écrirait une demande
// que personne n'exécuterait avant le prochain redémarrage : bbolt est ouvert à
// chaque opération, donc la commande voisine écrit bien, mais seule l'instance
// qui tient l'interface peut ouvrir le port. Cette boucle la réveille.
//
// Le rythme est volontairement lent et sans E/S quand l'état colle : un
// booléen en cache et, si ça bouge, un appel mémoire.
func parallelBridgeWatch(stop <-chan struct{}) {
	t := time.NewTicker(4 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-stop:
			return
		case <-t.C:
			parallelBridgeReconcile()
		}
	}
}

// checkParallelPortFree refuse un port déjà tenu, par cette instance ou par un
// tiers, sans jamais arrêter le tiers (parallèleBuild = on ne tue rien).
func checkParallelPortFree(port string) error {
	used := map[string]string{
		parallelUIPort():     "l'interface de cette instance",
		parallelEnginePort(): "le réglage moteur de cette instance",
		// Le port réellement servi par le moteur peut différer du réglage (un
		// preset importé impose PORT dans la config, et c'est lui que le proxy du
		// pont va frapper). Les deux comptent comme pris.
		parallelLocalEnginePort(): "le moteur de cette instance",
		strconv.Itoa(readEmbeddingDefaults().Port): "l'encodeur de mémoire de cette instance",
	}
	if who, dup := used[port]; dup {
		return fmt.Errorf("port %s déjà pris par %s", port, who)
	}
	if err := waitPortFree("127.0.0.1", port, 0); err != nil {
		return fmt.Errorf("port %s occupé ; aucun processus arrêté : %w", port, err)
	}
	return nil
}

// parallelBridgeLanURLs : ce qu'il faut coller chez l'autre machine. On
// n'invente aucune adresse : on reprend l'IP détectée par le panneau réseau.
func parallelBridgeLanURLs() []string {
	port := parallelBridgePort()
	out := []string{"http://127.0.0.1:" + port + "/v1"}
	if ip := strings.TrimSpace(localIP()); ip != "" && ip != "localhost" && ip != "127.0.0.1" {
		out = append(out, "http://"+ip+":"+port+"/v1")
	}
	return out
}

// discoverEngineModel demande /v1/models au moteur de CETTE instance. Le nom du
// modèle vient de lui : un preset externe qui l'invente reçoit une erreur du
// moteur, pas une réponse vide.
// discoverEngineModel : le modèle que ce pont prêtera. On interroge d'abord le
// moteur local (c'est le cas le plus fréquent), mais le vrai arbitre est le
// preset actif : un preset « API Externe » ou un GPU cloud n'a rien sur le port
// du moteur, et un pont qui prétendrait n'avoir aucun modèle serait faux.
func discoverEngineModel() string {
	if m := probeLocalEngineModel(); m != "" {
		return m
	}
	if ep := resolveChatEndpoint(); ep.Model != "" {
		return ep.Model
	}
	return ""
}

// probeLocalEngineModel : le /v1/models du moteur local, s'il répond.
func probeLocalEngineModel() string {
	base := "http://127.0.0.1:" + parallelLocalEnginePort()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/v1/models", nil)
	if err != nil {
		return ""
	}
	if k := readAPIKey(); k != "" {
		req.Header.Set("Authorization", "Bearer "+k)
	}
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return ""
	}
	var out struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&out); err != nil {
		return ""
	}
	for _, m := range out.Data {
		if id := strings.TrimSpace(m.ID); id != "" {
			return id
		}
	}
	return ""
}

// parallelBridgeStatus : ce que voient la carte UI, `ajean bridge status` et
// l'API. `serving` dit d'OÙ vient le cerveau prêté — c'est ce qui permet de
// annoncer honnêtement « modèle distant » au lieu d'un faux « aucun modèle ».
func parallelBridgeStatus() map[string]any {
	running := parallelBridgeRunning()
	model := ""
	if running {
		model = discoverEngineModel()
	} else if ep := resolveChatEndpoint(); ep.External || ep.Cloud {
		// Pont éteint mais preset distant : le modèle prêté se lit dans la
		// config, inutile d'aller sonder un moteur local qui n'existera jamais.
		model = ep.Model
	}
	ep := resolveChatEndpoint()
	serving := "moteur local 127.0.0.1:" + parallelLocalEnginePort()
	switch {
	case ep.External:
		serving = "API distante " + ep.URL
	case ep.Cloud:
		serving = "GPU cloud"
	}
	return map[string]any{
		"ok":          true,
		"requested":   parallelBridgeRequested(),
		"env_forced":  parallelBridgeEnvForced(),
		"enabled":     parallelBridgeEnabled(),
		"running":     running,
		"listening":   parallelBridgeListening(),
		"port":        parallelBridgePort(),
		"host":        parallelBridgeHost(),
		"has_key":     readAPIKey() != "",
		// agent_on : le canal déclenche-t-il un tour d'agent ici (shell, disque),
		// ou seulement des complétions du modèle prêté. Les deux sources comptent :
		// l'interrupteur du pont ET le mode agent de la machine.
		"agent_on":         parallelBridgeAgentOn() && agentEnabled(),
		"agent_requested":  parallelBridgeAgentOn(),
		"agent_mode_local": agentEnabled(),
		"model":       model,
		"urls":        parallelBridgeLanURLs(),
		// engine_port : le port du RÉGLAGE d'instance ; engine_serving : le port
		// que la config impose au moteur et que le pont proxifie. Les deux peuvent
		// différer (preset importé) — les montrer évite de chercher midi à quatorze
		// heures quand le pont répond mais ne trouve personne.
		"engine_port":    parallelEnginePort(),
		"engine_serving": parallelLocalEnginePort(),
		"serving":        serving,
		"preset":         strings.TrimSpace(ReadConfig()["NAME"]),
	}
}

// handleParallelBridge : /api/bridge. GET = état ; POST {enabled, port} =
// allumer/couper et/ou déplacer le port, à chaud.
func handleParallelBridge(w http.ResponseWriter, r *http.Request) {
	if !parallelBuild() {
		sendJSON(w, http.StatusForbidden, map[string]any{
			"ok": false, "error": "pont réservé à la version portable de Cresius",
		})
		return
	}
	switch r.Method {
	case http.MethodGet:
		w.Header().Set("Cache-Control", "no-store, max-age=0")
		sendJSON(w, http.StatusOK, parallelBridgeStatus())
		return
	case http.MethodPost:
		var req struct {
			Enabled *bool  `json:"enabled"`
			Agent   *bool  `json:"agent"`
			Port    string `json:"port"`
			Apply   bool   `json:"apply"`
		}
		if r.Body == nil {
			sendJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "JSON attendu"})
			return
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 32<<10)).Decode(&req); err != nil {
			sendJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "JSON invalide"})
			return
		}
		if req.Port != "" {
			n, err := strconv.Atoi(strings.TrimSpace(req.Port))
			if err != nil || n < 1025 || n > 65534 {
				sendJSON(w, http.StatusBadRequest, map[string]any{
					"ok": false, "error": "port du pont invalide : " + req.Port + " (choisir entre 1025 et 65534)",
				})
				return
			}
			p := strconv.Itoa(n)
			if err := stopParallelBridge(); err != nil {
				sendJSON(w, http.StatusInternalServerError, map[string]any{"ok": false, "error": err.Error()})
				return
			}
			if err := checkParallelPortFree(p); err != nil {
				sendJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": err.Error()})
				return
			}
			if err := putStr(bkState, parallelBridgePortKey, p); err != nil {
				sendJSON(w, http.StatusInternalServerError, map[string]any{"ok": false, "error": err.Error()})
				return
			}
		}
		if req.Agent != nil {
			if err := setParallelBridgeAgentOn(*req.Agent); err != nil {
				sendJSON(w, http.StatusInternalServerError, map[string]any{"ok": false, "error": err.Error()})
				return
			}
		}
		if req.Enabled != nil {
			if err := setParallelBridgeRequested(*req.Enabled); err != nil {
				sendJSON(w, http.StatusInternalServerError, map[string]any{"ok": false, "error": err.Error()})
				return
			}
			if err := stopParallelBridge(); err != nil {
				sendJSON(w, http.StatusInternalServerError, map[string]any{"ok": false, "error": err.Error()})
				return
			}
		}
		// Rien n'est écouté « à la demande » : soit le pont tourne, soit il est
		// éteint. On ne le rallume que si on le peut, et on le dit.
		if parallelBridgeEnabled() && !parallelBridgeRunning() {
			if err := startParallelBridge(); err != nil {
				st := parallelBridgeStatus()
				st["ok"] = false
				st["error"] = err.Error()
				sendJSON(w, http.StatusOK, st)
				return
			}
		}
		sendJSON(w, http.StatusOK, parallelBridgeStatus())
		return
	default:
		sendJSON(w, http.StatusMethodNotAllowed, map[string]any{"ok": false, "error": "GET ou POST"})
	}
}

// cmdParallelBridge : `ajean bridge status|on|off [port]|serve [port]`. Le
// drapeau est écrit dans la base : l'instance en cours le prend à chaud par
// /api/bridge, ou au prochain démarrage. Une seconde instance, elle, ne peut pas
// hériter du réglage de l'autre — chaque dossier a sa base.
//
// `serve` tient le pont au premier plan, sans interface ni tray : c'est la
// manière de prêter un cerveau depuis un terminal, ou un autre binaire.
func cmdParallelBridge(args []string) error {
	sub := "status"
	if len(args) > 0 {
		sub = strings.ToLower(strings.TrimSpace(args[0]))
		args = args[1:]
	}
	// `bridge agent on|off` : l'autorisation de déclencher un tour d'agent local
	// depuis le canal. Séparée de `bridge on` parce que ce n'est pas la même chose
	// prêter un modèle et prêter le shell de sa machine.
	if sub == "agent" {
		qui := "on"
		if len(args) > 0 {
			qui = strings.ToLower(strings.TrimSpace(args[0]))
		}
		switch qui {
		case "on", "activer", "oui", "1":
			if err := setParallelBridgeAgentOn(true); err != nil {
				return err
			}
			fmt.Printf("%s le canal peut déclencher un tour d'agent sur ce poste (outils, shell, disque, mémoire).\n", green("[ok]"))
			if !agentEnabled() {
				fmt.Printf("%s attention : le MODE AGENT de cette machine est éteint, donc rien ne se déclenchera\n", yellow("[!]"))
				fmt.Printf("             tant que l'interface n'est pas en mode Projet/Agent.\n")
			}
			return nil
		case "off", "couper", "non", "0":
			if err := setParallelBridgeAgentOn(false); err != nil {
				return err
			}
			fmt.Printf("%s le canal ne prête plus que le modèle nu (complétions), rien ne s'exécute ici\n", green("[ok]"))
			return nil
		default:
			return fmt.Errorf("usage : ajean bridge agent [on|off]")
		}
	}
	port := ""
	if len(args) > 0 {
		port = strings.TrimSpace(args[0])
	}
	switch sub {
	case "status", "etat":
		st := parallelBridgeStatus()
		fmt.Printf("pont Ajean   : demandé=%v  actif=%v  à l'écoute=%v (ici=%v)\n", st["requested"], st["enabled"], st["listening"], st["running"])
		fmt.Printf("écoute       : http://%s/v1\n", net.JoinHostPort(parallelBridgeHost(), parallelBridgePort()))
		// Ce que le voisin a le DROIT de faire ici. La distinction s'affiche parce
		// qu'elle change tout au diagnostic : avec l'agent, le voisin peut lire ses
		// propres journaux ; sans lui, il ne reçoit que du texte de modèle nu.
		if st["agent_on"].(bool) {
			fmt.Printf("canal        : %s un tour d'agent local est autorisé (outils, shell, disque, mémoire)\n", green("agent"))
		} else {
			fmt.Printf("canal        : %s — le voisin ne reçoit que le modèle nu. Ouvrir l'agent : %s\n",
				yellow("prêt du modèle seul"), bold("ajean bridge agent on"))
			if parallelBridgeAgentOn() && !agentEnabled() {
				fmt.Printf("             (l'autorisation du pont est prise, mais le mode agent de cette machine est éteint)\n")
			}
		}
		if ep, ok := st["engine_serving"].(string); ok {
			fmt.Printf("moteur prêté : 127.0.0.1:%s\n", ep)
		}
		if !st["has_key"].(bool) {
			fmt.Printf("%s aucune clé API machine : le pont refuserait tout client, il ne démarrera pas\n", yellow("[!]"))
			fmt.Printf("             → ajean set-api-key <une-longue-cle>\n")
		}
		// Le pare-feu est la cause n°1 d'un pont « ouvert » que la machine
		// d'à côté n'atteint pas. Le dire ici évite une heure de diagnostics.
		if n, conv := strconv.Atoi(parallelBridgePort()); conv == nil {
			switch fw := firewallState(n); fw {
			case "ouvert":
				fmt.Printf("pare-feu     : %s\n", green("port autorisé"))
			case "ferme":
				fmt.Printf("pare-feu     : %s\n", yellow("aucune règle pour ce port — invisible depuis une autre machine"))
				fmt.Printf("             %s\n", firewallManualHint(n))
			}
		}
		for _, u := range parallelBridgeLanURLs() {
			fmt.Printf("coller chez l'autre Ajean : %s\n", u)
		}
		if m := discoverEngineModel(); m != "" {
			fmt.Printf("modèle       : %s\n", m)
		} else {
			fmt.Printf("%s aucun moteur ne répond sur ce port : le pont n'a rien à prêter\n", yellow("[!]"))
		}
		return nil
	case "on", "activer":
		if port != "" {
			n, err := strconv.Atoi(port)
			if err != nil || n < 1025 || n > 65534 {
				return fmt.Errorf("port du pont invalide : %s (choisir entre 1025 et 65534)", port)
			}
			if err := putStr(bkState, parallelBridgePortKey, strconv.Itoa(n)); err != nil {
				return err
			}
		}
		if err := setParallelBridgeRequested(true); err != nil {
			return err
		}
		if readAPIKey() == "" {
			fmt.Printf("%s sans clé API machine il ne démarrera pas : ajean set-api-key\n", yellow("[!]"))
		}
		// Cette commande écrit la demande ; elle ne tient pas l'écoute. Un
		// process CLI qui ouvrirait le port mourrait avec lui, et la machine
		// croirait à un pont ouvert. C'est donc l'instance en cours qui ouvre
		// (la veille le fait en quelques secondes), ou `bridge serve` si aucune
		// instance ne tourne.
		fmt.Printf("%s demande enregistrée : port %s\n", green("[ok]"), parallelBridgePort())
		if parallelBridgeListening() {
			fmt.Printf("       l'instance en cours l'appliquera d'elle-même en quelques secondes.\n")
		} else {
			fmt.Printf("       aucune instance de ce dossier n'est en cours : tiens le pont toi-même avec\n")
			fmt.Printf("       %s\n", bold("ajean bridge serve"))
		}
		return nil
	case "off", "couper":
		if err := setParallelBridgeRequested(false); err != nil {
			return err
		}
		if err := stopParallelBridge(); err != nil {
			return err
		}
		if parallelBridgeListening() {
			fmt.Printf("%s demande coupée ; un pont écoute encore sur %s : l'instance en cours doit le couper, ou un autre programme tient ce port.\n", yellow("[!]"), parallelBridgePort())
			return nil
		}
		fmt.Printf("%s pont coupé (plus aucune écoute sur le réseau local)\n", green("[ok]"))
		return nil
	case "serve", "ecouter", "écoute":
		return parallelBridgeServe()
	default:
		return fmt.Errorf("usage : ajean bridge [status|on [port]|off|serve [port]]")
	}
}

// parallelBridgeServe tient le pont au premier plan : pas d'interface, pas de
// tray, rien d'exposé d'autre que /v1/*. Le moteur local est démarré s'il
// manque — un pont qui prête un cerveau sans cerveau ne sert à rien.
func parallelBridgeServe() error {
	fmt.Printf("données : %s\n", AjeanHome())
	if readAPIKey() == "" {
		return errors.New("le pont exige une clé API machine : ajean set-api-key <clé>")
	}
	if err := setParallelBridgeRequested(true); err != nil {
		return err
	}
	if err := ensureParallelEnginePort(); err != nil {
		return err
	}
	// D'où vient le cerveau ? Un preset « API Externe » ou un GPU cloud n'a rien
	// à lancer en local : vouloir démarrer un moteur là dedans, c'est échouer pour
	// rien (et parfois télécharger des dizaines de go).
	if usesRemoteEndpoint(ReadConfig()) {
		ep := resolveChatEndpoint()
		fmt.Printf("[pont ajean] modèle prêté : preset distant (%s), modèle %s\n", ep.URL, ep.Model)
	} else if m := probeLocalEngineModel(); m != "" {
		fmt.Printf("[pont ajean] moteur déjà en service sur 127.0.0.1:%s (%s)\n", parallelLocalEnginePort(), m)
	} else {
		fmt.Printf("[pont ajean] moteur absent : démarrage sur 127.0.0.1:%s …\n", parallelLocalEnginePort())
		if err := parallelServiceAction("start"); err != nil {
			return fmt.Errorf("moteur introuvable : %w", err)
		}
		if err := waitEngineReady(120 * time.Second); err != nil {
			return err
		}
	}
	if err := startParallelBridge(); err != nil {
		return err
	}
	fmt.Printf("[pont ajean] à l'écoute. Coller chez l'autre Ajean (preset externe) :\n")
	for _, u := range parallelBridgeLanURLs() {
		fmt.Printf("    %s\n", u)
	}
	if m := discoverEngineModel(); m != "" {
		fmt.Printf("  modèle : %s\n", m)
	}
	fmt.Printf("Ctrl-C pour couper.\n")
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	<-signals
	signal.Stop(signals)
	_ = stopParallelBridge()
	_ = setParallelBridgeRequested(false)
	_ = parallelServiceAction("stop")
	fmt.Printf("\n[pont ajean] coupé.\n")
	return nil
}

// waitEngineReady attend que le moteur réponde à /v1/models — la preuve qu'il
// peut servir, pas seulement qu'un process existe.
func waitEngineReady(max time.Duration) error {
	deadline := time.Now().Add(max)
	for time.Now().Before(deadline) {
		if discoverEngineModel() != "" {
			return nil
		}
		time.Sleep(2 * time.Second)
	}
	return fmt.Errorf("moteur prêt nulle part sur 127.0.0.1:%s après %s", parallelLocalEnginePort(), max)
}
