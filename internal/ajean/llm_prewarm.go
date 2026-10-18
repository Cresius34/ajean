package ajean

// llm_prewarm.go — pré-chauffage du PRÉFIXE de prompt auprès du moteur.
//
// Le premier message d'une conversation paie le préfill de tout ce qui précède
// le texte de l'utilisateur : préambule système, briefing machine, prompt du
// preset et surtout les schémas d'outils — plusieurs milliers de jetons. Sur un
// modèle local, c'est l'essentiel du délai avant le premier jeton, et ce préfixe
// est IDENTIQUE d'une conversation à l'autre (le contexte projet, lui, part dans
// le premier message utilisateur, voir normalizeSystemMessages).
//
// Dès que le moteur répond, on envoie donc une complétion minuscule
// (max_tokens=1) avec EXACTEMENT ce préfixe. llama-server garde le cache KV du
// créneau (--cache-ram / --cache-idle-slots sont actifs par défaut) : le premier
// vrai message ne traite plus que son propre texte, et le premier jeton arrive
// tout de suite. Rien n'est affiché : la requête est interne, sa réponse jetée.
//
// Le pré-chauffage est BÉNIN : jamais pendant une génération (le compteur
// llmInFlight le dit), annulé dès qu'un tour utilisateur démarre (le tour
// chauffera le cache lui-même), deux tentatives au plus, signature mémorisée
// pour ne pas rejouer le même préfixe à chaque sondage de /api/status.

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	// Texte du faux message utilisateur : il ne sert qu'à rendre le prompt
	// valide pour tous les gabarits de chat. Seul le préfixe qui le précède
	// nous intéresse.
	llmPrewarmUserText = "ok"
	// Borne : sur un gros modèle CPU le préfill peut être long, mais au-delà le
	// pré-chauffage retarderait le premier vrai message pour rien.
	llmPrewarmTimeout = 30 * time.Second
	// Deux essais suffisent (moteur ancien qui refuserait le corps, coupure…).
	// Ensuite on abandonne jusqu'au prochain redémarrage du moteur ou changement
	// de préfixe.
	llmPrewarmMaxTries = 2
	// Une fois le préfixe chauffé, on revérifie quand même de temps en temps
	// qu'il n'a pas changé (prompt du preset édité, outils activés…) — sans
	// reconstruire le préfixe à chaque sondage de /api/status (toutes les 5 s).
	llmPrewarmRecheck = 15 * time.Second
)

// llmInFlight compte les complétions réellement en cours vers le moteur (chat,
// tâches, Jean, apps…). Sert au pré-chauffage : inutile — et nuisible — de
// s'insérer devant une génération déjà lancée.
var llmInFlight atomic.Int32

// llmTurnsActive couvre les TROUS entre deux requêtes d'un même tour (exécution
// d'un outil, lecture de fichier…) : llmInFlight retombe à zéro alors que le tour
// n'est pas fini et qu'un pré-chauffage s'intercalerait devant l'étape suivante.
// Compteur (pas un booléen) : un Reset peut lancer le tour suivant avant que le
// tour abandonné ne se termine ; sa décrémentation ne doit pas libérer le tour
// vivant.
var llmTurnsActive atomic.Int32

// llmPrewarmState : un seul pré-chauffage à la fois, et un souvenir de ce qui a
// déjà été chauffé pour ne pas le refaire à chaque sondage.
var llmWarm struct {
	mu        sync.Mutex
	healthy   bool // dernier état de santé observé (détecte un redémarrage moteur)
	sig       string
	warmed    bool
	inflight  bool
	attempts  int
	nextTry   time.Time
	lastCheck time.Time
	cancel    context.CancelFunc
}

// prewarmPrefix construit le préfixe commun à toute conversation : le bloc
// système tel qu'InjectSkills + normalizeSystemMessages le produisent, et les
// outils tels que runChat les annonce. Le contexte projet est volontairement
// absent (il vit dans le premier message utilisateur).
//
// Renvoie ok=false quand il n'y a rien à chauffer (modèle brut, sans système ni
// outil) : envoyer une requête ne gagnerait rien.
func prewarmPrefix(caps Caps) (system string, tools []Tool, ok bool) {
	var parts []string
	if bp := baseSystemPrompt(caps); bp != "" {
		parts = append(parts, bp)
	}
	if mp := machineSystemPrompt(caps); mp != "" {
		parts = append(parts, mp)
	}
	// Même ordre que generate : le prompt du preset est le premier message
	// système, InjectSkills le fusionne APRÈS le préambule.
	if sp := readSysPrompt(); sp != "" && caps.Agent && !caps.Terminal {
		parts = append(parts, sp)
	}
	system = strings.Join(parts, "\n\n")
	// Même liste que runChat : withoutRecallTools retire recall/recall_search
	// tant que le fil n'a pas été compacté (c'est le cas d'une conversation
	// neuve, celle qu'on pré-chauffe).
	tools = withRecallTools(EnabledTools(caps), nil)
	return system, tools, system != "" || len(tools) > 0
}

// prewarmSignature identifie le préfixe chauffé. Un changement de preset, de
// modèle, d'outil ou de prompt système doit relancer le pré-chauffage.
func prewarmSignature(ep chatEndpoint, system string, tools []Tool) string {
	h := sha256.New()
	h.Write([]byte(ep.URL))
	h.Write([]byte{0})
	h.Write([]byte(ep.Model))
	h.Write([]byte{0})
	h.Write([]byte(system))
	for _, t := range tools {
		b, _ := json.Marshal(t) // ordre des champs fixe, clés de map triées
		h.Write([]byte{0})
		h.Write(b)
	}
	return hex.EncodeToString(h.Sum(nil)[:12])
}

// prewarmPayload est le corps envoyé au moteur. Volontairement minimal : on ne
// veut qu'un préfill, pas une génération.
func prewarmPayload(ep chatEndpoint, system string, tools []Tool) map[string]any {
	msgs := make([]Message, 0, 2)
	if system != "" {
		msgs = append(msgs, Message{Role: "system", Content: system})
	}
	msgs = append(msgs, Message{Role: "user", Content: llmPrewarmUserText})
	p := map[string]any{
		"model":        ep.Model,
		"messages":     msgs,
		"max_tokens":   1,
		"temperature":  0,
		"stream":       false,
		"cache_prompt": true,
	}
	if len(tools) > 0 {
		p["tools"] = tools
		p["parallel_tool_calls"] = false
	}
	return p
}

// LLMPrewarm doit être appelé à chaque fois que l'état du moteur est connu au
// service d'interface (démarrage de l'UI/link, sondage /api/status). Il est non
// bloquant : le pré-chauffage part dans sa propre goroutine.
//
// healthy=false signale un moteur qui ne répond pas encore : au prochain
// passage à true, le cache KV est forcément froid, on ré-arme.
func LLMPrewarm(healthy bool) {
	// Phase 1 : état rapide, sans travail lourd sous le verrou.
	llmWarm.mu.Lock()
	if !healthy {
		llmWarm.healthy = false
		llmWarm.mu.Unlock()
		return
	}
	if !llmWarm.healthy {
		llmWarm.healthy = true
		llmWarm.sig, llmWarm.warmed, llmWarm.attempts = "", false, 0
		llmWarm.lastCheck = time.Time{}
	}
	// Une génération réelle occupe le moteur : on ne se met pas devant elle.
	// (Si le pré-chauffage est déjà parti, le tour le coupera — voir StartTurn.)
	if llmInFlight.Load() > 0 || llmTurnsActive.Load() > 0 || llmWarm.inflight {
		llmWarm.mu.Unlock()
		return
	}
	if llmWarm.warmed && time.Since(llmWarm.lastCheck) < llmPrewarmRecheck {
		llmWarm.mu.Unlock()
		return
	}
	// Après un échec, on laisse passer le recul avant de reconstruire le préfixe
	// (connexions MCP, sondage Crawl4AI) : rien ne doit tourner en boucle.
	if !llmWarm.warmed && time.Now().Before(llmWarm.nextTry) {
		llmWarm.mu.Unlock()
		return
	}
	llmWarm.lastCheck = time.Now()
	llmWarm.mu.Unlock()

	// Phase 2 : construction du préfixe HORS verrou. Elle peut être lente (les
	// serveurs MCP se connectent, Crawl4AI est sondé) et ne doit jamais retarder
	// cancelLLMPrewarmForTurn, appelé au démarrage d'un tour utilisateur.
	cfg := ReadConfig()
	if isExternalConfig(cfg) || isCloudConfig(cfg) {
		return
	}
	ep := resolveChatEndpoint()
	if ep.URL == "" {
		return
	}
	system, tools, ok := prewarmPrefix(globalCaps())
	if !ok {
		return
	}
	sig := prewarmSignature(ep, system, tools)

	// Phase 3 : décision et lancement, sous verrou, en revérifiant l'état (un
	// tour a pu démarrer pendant la phase 2).
	llmWarm.mu.Lock()
	if llmWarm.sig != sig {
		llmWarm.sig, llmWarm.warmed, llmWarm.attempts = sig, false, 0
	}
	if llmWarm.warmed || llmWarm.inflight || llmInFlight.Load() > 0 || llmTurnsActive.Load() > 0 ||
		llmWarm.attempts >= llmPrewarmMaxTries || time.Now().Before(llmWarm.nextTry) {
		llmWarm.mu.Unlock()
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), llmPrewarmTimeout)
	llmWarm.cancel = cancel
	llmWarm.inflight = true
	llmWarm.mu.Unlock()

	payload := prewarmPayload(ep, system, tools)
	go func(sig string) {
		err := sendLLMPrewarm(ctx, ep, payload)
		cancel()
		llmWarm.mu.Lock()
		llmWarm.inflight = false
		llmWarm.cancel = nil
		// Un autre préfixe a pu prendre la main pendant l'envoi (changement de
		// preset, de prompt système…) : son état ne doit pas être écrasé par le
		// résultat de l'ancien.
		if llmWarm.sig == sig {
			if err != nil {
				llmWarm.attempts++
				// Recul : un moteur qui refuse ce corps n'a pas besoin d'être
				// resondé toutes les deux secondes.
				llmWarm.nextTry = time.Now().Add(time.Duration(llmWarm.attempts) * 15 * time.Second)
			} else {
				llmWarm.warmed = true
				llmWarm.attempts = 0
			}
		}
		llmWarm.mu.Unlock()
	}(sig)
}

// cancelLLMPrewarmForTurn coupe un pré-chauffage en cours quand un tour
// utilisateur démarre : sa demande passe devant tout travail interne. On note le
// cache comme chauffé — le tour va de toute façon traiter (et garder) le préfixe.
func cancelLLMPrewarmForTurn() {
	llmWarm.mu.Lock()
	cancel := llmWarm.cancel
	if cancel != nil {
		llmWarm.cancel = nil
		llmWarm.warmed = true
	}
	llmWarm.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

// sendLLMPrewarm envoie la requête de pré-chauffage et draine la réponse (pour
// laisser la connexion réutilisable par le vrai message).
func sendLLMPrewarm(ctx context.Context, ep chatEndpoint, payload map[string]any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, ep.URL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if ep.External {
		if ep.Key != "" {
			req.Header.Set("Authorization", "Bearer "+ep.Key)
		}
	} else {
		authHeader(req)
	}
	// Client par défaut : même pool de connexions que le chat, donc le premier
	// message réutilise la connexion déjà ouverte.
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<16))
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("pré-chauffage : HTTP %d", resp.StatusCode)
	}
	return nil
}
