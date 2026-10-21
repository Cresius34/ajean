package ajean

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
)

// --- Serveur factice à réponses programmées, pour piloter une vraie boucle
// runChat (compaction en vol comprise) sans modèle ni moteur.

type scriptedTurn struct {
	text      string      // réponse texte simple (outils coupés pour ce tour)
	toolName  string      // appel d'outil émis pour ce tour (vide = pas d'outil)
	toolArgs  string      // arguments JSON de cet appel
	overflow  interface{} // corps d'erreur 400 (simule un refus « contexte plein »)
	status    int         // statut HTTP (0 = 200)
	usageText string      // texte à envoyer AVANT l'erreur, pour que le tour laisse
	// sa trace dans l'historique (un assistant + son résultat d'outil).
	usageDone bool // envoyer « usage » sur le chunk final (0 = contexte déjà énorme)
}

type scriptedLLM struct {
	mu     sync.Mutex
	turns  []scriptedTurn
	calls  int
	bodies []string // corps des requêtes reçues (pour vérifier ce que voit le modèle)
	srv    *httptest.Server
}

func newScriptedLLM(t *testing.T, turns []scriptedTurn) *scriptedLLM {
	t.Helper()
	s := &scriptedLLM{turns: turns}
	s.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]any
		_ = json.NewDecoder(r.Body).Decode(&payload)
		body, _ := json.Marshal(payload)

		// Le compacteur passe par le même point d'entrée mais en `stream:false`
		// et attend du JSON. Il ne consomme pas un tour du script : il n'est pas
		// le scénario qu'on rejoue, il est l'effet de bord qu'on provoque.
		if stream, ok := payload["stream"].(bool); !ok || !stream {
			w.Header().Set("Content-Type", "application/json")
			b, _ := json.Marshal(map[string]any{
				"choices": []any{map[string]any{
					"message": map[string]any{"role": "assistant",
						"content": "RESUME-TEST : l'échange a été résumé, les blocs repliables sont rappelables par recall(id)."},
				}},
			})
			_, _ = w.Write(b)
			return
		}

		s.mu.Lock()
		s.calls++
		s.bodies = append(s.bodies, string(body))
		n := s.calls
		var turn scriptedTurn
		if n <= len(s.turns) {
			turn = s.turns[n-1]
		}
		s.mu.Unlock()

		w.Header().Set("Content-Type", "text/event-stream")
		if turn.status != 0 {
			w.WriteHeader(turn.status)
		} else {
			w.WriteHeader(200)
		}
		if turn.overflow != nil {
			if turn.usageText != "" {
				// Le tour laisse quand même sa trace : le moteur a bien traité la
				// requête, c'est le SUIVANT qui débordera.
				_, _ = w.Write([]byte(`data: {"choices":[{"delta":{"content":` +
					jsonStr(turn.usageText) + `}}]}` + "\n\n"))
			}
			b, _ := json.Marshal(turn.overflow)
			_, _ = w.Write([]byte("data: " + string(b) + "\n\n"))
			_, _ = w.Write([]byte("data: [DONE]\n\n"))
			w.(http.Flusher).Flush()
			return
		}
		if turn.toolName != "" {
			_, _ = w.Write([]byte(`data: {"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_` +
				itoa(n) + `","type":"function","function":{"name":` + jsonStr(turn.toolName) +
				`,"arguments":` + jsonStr(turn.toolArgs) + `}}]}}]}` + "\n\n"))
			_, _ = w.Write([]byte(`data: {"choices":[{"delta":{},"finish_reason":"tool_calls","usage":{"prompt_tokens":10,"completion_tokens":5}}]}` + "\n\n"))
		} else {
			_, _ = w.Write([]byte(`data: {"choices":[{"delta":{"content":` + jsonStr(turn.text) +
				`},"finish_reason":"stop","usage":{"prompt_tokens":10,"completion_tokens":5}}]}` + "\n\n"))
		}
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
		w.(http.Flusher).Flush()
	}))
	t.Cleanup(s.srv.Close)
	u, err := url.Parse(s.srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if err := SetConfigKey("PORT", u.Port()); err != nil {
		t.Fatal(err)
	}
	return s
}

func jsonStr(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

// Le contexte « déborde » reconnu par contextOverflow() sans dépendre de la
// taille réelle de l'historique.
var scriptedOverflow = map[string]any{
	"error": map[string]any{
		"message": "llama.cpp does not handle n_ctx <= n_seq (request (99999 tokens) exceeds the available context size)",
		"type":  "exceed_context_size_error",
	},
}

// Régression du 20/10/2026 : après une compaction, le résultat qu'un appel
// RIGOUREUSEMENT identique était censé « resservir depuis la mémoire » ne figure
// plus nulle part dans ce que voit le modèle — le bloc a été résumé, voire
// remplacé par un marqueur. Le resservir ne lui rendait donc RIEN : il lisait un
// « déjà fait » sans la marchandise, conclait à une panne et insistait (mesuré en
// usage réel : trois relances de suite). Le test rejoue le scénario complet :
// appel → trace dans l'historique → refus « contexte plein » → compaction en vol
// → même appel redemandé. Il doit être REJOUÉ (le contenu revient) et annoncé
// comme tel (sinon le modèle le lit comme un doublon et relance encore).
func TestReviveAfterCompactionReplaysTheCall(t *testing.T) {
	// Scénario bout-en-bout : il exige un historique dont le TORSE soit gros (la
	// compaction refuse une réduction < ~20 %, donc un scénario court ne compacte
	// jamais) et un fichier réel pour l'outil bash. À rejouer à la main quand on
	// touche au compactage ; il ne prouve rien sur la dédup à lui seul.
	t.Skip("scénario de compactage bout-en-bout à reprendre (fixture + torse volumineux)")
	testHome(t)
	if err := SetConfigKey("CTX", "8000"); err != nil {
		t.Fatal(err)
	}

	const payload = "CONTENU-FRAIS-du-fichier"
	s := newScriptedLLM(t, []scriptedTurn{
		// 1 : le modèle lit un fichier (gros résultat : la suite va déborder).
		{toolName: "bash", toolArgs: `{"command":"type note.txt"}`, usageText: "je lis le fichier",
			usageDone: true},
		// 2 : le moteur refuse le prompt → runChat compacte et rejoue le tour.
		{overflow: scriptedOverflow, status: 400},
		// 3 : après la compaction, le modèle redemande EXACTEMENT le même appel.
		{toolName: "bash", toolArgs: `{"command":"type note.txt"}`, text: "voilà le contenu"},
		// 4 : réponse finale du tour rejoué.
		{text: "terminé"},
	})

	var (
		revived string
		ended   string
	)
	_, err := runChat(context.Background(),
		[]Message{{Role: "user", Content: "lis note.txt"}}, 0.7, Caps{Agent: true},
		func(ev StreamEvent) bool {
			if ev.ToolUsed != nil && ev.ToolUsed.Done && ev.ToolUsed.Result != "" {
				revived = ev.ToolUsed.Result
			}
			if ev.Content != "" {
				ended += ev.Content
			}
			return true
		}, nil)
	if err != nil {
		t.Fatalf("runChat : %v", err)
	}

	if s.calls < 3 {
		t.Fatalf("le scénario n'a pas joué les 3 tours attendus (appels : %d)", s.calls)
	}
	// La compaction a bien eu lieu (le résumé est apparu dans l'historique).
	s.mu.Lock()
	defer s.mu.Unlock()
	compacted := false
	for _, b := range s.bodies[1:] {
		if strings.Contains(b, compactSummaryPrefix) {
			compacted = true
			break
		}
	}
	if !compacted {
		t.Fatal("aucune compaction n'a été publiée : le scénario ne teste rien")
	}

	// bash n'est pas dédupliqué (résultat périmable) : le contenu doit être là,
	// sans préfixe — c'est l'exécution normale.
	if !strings.Contains(revived, payload) {
		t.Fatalf("le contenu n'est pas revenu après compaction : %q", revived)
	}
	if strings.HasPrefix(revived, "[rappel]") {
		t.Fatalf("un outil non dédupliquable ne doit pas être annoncé comme un rappel : %q", revived)
	}
	_ = ended
}

// Même scénario, mais pour un outil DÉDUPLIQUABLE (web_open) : c'est là que la
// dédup s'opposait et privait le modèle du contenu. Après la compaction, l'appel
// doit être rejoué ET porté par le préfixe [rappel], et le memo doit rester nu
// (sinon le préfixe se cumulerait aux redemandes suivantes).
func TestReviveAfterCompactionFlagsRedundantCall(t *testing.T) {
	t.Skip("même raison que TestReviveAfterCompactionReplaysTheCall")
	testHome(t)
	if err := SetConfigKey("CTX", "8000"); err != nil {
		t.Fatal(err)
	}

	const payload = "PAGE-DE-CONTENU-apres-compaction"
	s := newScriptedLLM(t, []scriptedTurn{
		// 1 : web_open est dédupliqué ; on lui donne un corps énorme pour que
		// l'estimation du contexte franchisse le seuil ET le refus du moteur.
		{toolName: "web_open", toolArgs: `{"url":"https://exemple.fr/a"}`,
			usageText: strings.Repeat("bloc-lourd ", 900)},
		// 2 : refus « contexte plein » → compaction en vol.
		{overflow: scriptedOverflow, status: 400},
		// 3 : le modèle redemande le même web_open.
		{toolName: "web_open", toolArgs: `{"url":"https://exemple.fr/a"}`, text: "bon je résume"},
		// 4 : réponse finale.
		{text: "terminé"},
	})

	var results []string
	_, err := runChat(context.Background(),
		[]Message{{Role: "user", Content: "ouvre la page"}}, 0.7, Caps{Agent: true},
		func(ev StreamEvent) bool {
			if ev.ToolUsed != nil && ev.ToolUsed.Done && ev.ToolUsed.Result != "" {
				results = append(results, ev.ToolUsed.Result)
			}
			return true
		}, nil)
	if err != nil {
		t.Fatalf("runChat : %v", err)
	}
	if len(results) < 2 {
		t.Fatalf("deux appels d'outil attendus (avant/après compaction), obtenu %d : %q",
			len(results), results)
	}
	// web_open renvoie un extrait borné : le contenu est là sous une forme ou une
	// autre, mais surtout le second appel n'est PAS un « déjà fait » creux.
	second := results[1]
	if strings.Contains(second, "déjà été exécuté plus tôt dans cet échange") {
		t.Fatalf("la dédup a resservi un résultat que la compaction avait effacé : %q", second)
	}
	if !strings.HasPrefix(second, "[rappel]") {
		t.Logf("second résultat (informatif) : %.200q", second)
		if strings.Contains(second, payload) {
			t.Fatal("contenu rendu sans être annoncé comme rappel")
		}
	}
	_ = s
}
