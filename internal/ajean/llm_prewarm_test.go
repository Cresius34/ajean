package ajean

import (
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// Le pré-chauffage ne vaut que si son préfixe est EXACTEMENT celui du vrai tour.
// Sinon le cache KV de llama-server ne se raccorde pas et on paie un préfill pour
// rien. Ce test verrouille l'invariant : le bloc système et les outils construits
// par prewarmPrefix doivent être identiques à ce que InjectSkills +
// normalizeSystemMessages + runChat produisent pour une conversation neuve.
func TestPrewarmPrefixMatchesChatPrefix(t *testing.T) {
	t.Setenv("AJEAN_HOME", t.TempDir())
	if err := setAgentEnabled(true); err != nil {
		t.Fatal(err)
	}
	if err := SetConfigKey("MEM_MODE", "always"); err != nil {
		t.Fatal(err)
	}
	// Prompt de preset : c'est le cas le plus courant, et il est fusionné APRÈS
	// le préambule par InjectSkills.
	const presetPrompt = "Réponds en français, sans préambule."
	if err := putStr(bkState, "sysprompt", presetPrompt); err != nil {
		t.Fatal(err)
	}

	caps := Caps{Agent: true, Mem: MemAlways}
	wantSystem, wantTools, ok := prewarmPrefix(caps)
	if !ok {
		t.Fatal("prewarmPrefix ne propose rien alors que l'agent et la mémoire sont actifs")
	}
	if wantSystem == "" || len(wantTools) == 0 {
		t.Fatalf("préfixe vide : system=%q tools=%d", wantSystem, len(wantTools))
	}

	// Vue modèle réelle d'un premier message (voir Conversation.generate).
	final := InjectSkills([]Message{
		{Role: "system", Content: readSysPrompt()},
		{Role: "user", Content: "bonjour"},
	}, caps)
	final = normalizeSystemMessages(final)
	if len(final) == 0 || final[0].Role != "system" {
		t.Fatalf("le vrai tour ne commence pas par un system : %+v", final)
	}
	gotSystem, _ := final[0].Content.(string)
	if gotSystem != wantSystem {
		t.Fatalf("préfixe système différent du vrai tour\n--- pré-chauffé ---\n%s\n--- réel ---\n%s", wantSystem, gotSystem)
	}

	// Avec un fil neuf, runChat retire recall/recall_search : même liste.
	gotTools := withRecallTools(EnabledTools(caps), nil)
	gb, _ := json.Marshal(gotTools)
	wb, _ := json.Marshal(wantTools)
	if string(gb) != string(wb) {
		t.Fatalf("outils du pré-chauffage différents du vrai tour\n--- pré-chauffé ---\n%s\n--- réel ---\n%s", wb, gb)
	}
}

// Modèle brut (ni agent ni mémoire) : aucun préfixe à chauffer, on ne doit pas
// envoyer de requête inutile au moteur.
func TestPrewarmPrefixEmptyForRawModel(t *testing.T) {
	t.Setenv("AJEAN_HOME", t.TempDir())
	if err := setAgentEnabled(false); err != nil {
		t.Fatal(err)
	}
	if err := SetConfigKey("MEM_MODE", "off"); err != nil {
		t.Fatal(err)
	}
	if _, _, ok := prewarmPrefix(Caps{Mem: MemOff}); ok {
		t.Fatal("pré-chauffage proposé pour un modèle brut, sans système ni outil")
	}
}

// La requête envoyée doit être minimale (un seul jeton) et porter le préfixe.
func TestPrewarmPayload(t *testing.T) {
	ep := chatEndpoint{URL: "http://127.0.0.1:1/v1/chat/completions", Model: "ajean"}
	tools := []Tool{bashTool()}
	p := prewarmPayload(ep, "SYSTEM", tools)
	if p["max_tokens"] != 1 {
		t.Fatalf("max_tokens = %v, attendu 1", p["max_tokens"])
	}
	if p["stream"] != false {
		t.Fatal("le pré-chauffage doit être non streamé")
	}
	if p["cache_prompt"] != true {
		t.Fatal("cache_prompt doit être vrai pour réutiliser le préfixe")
	}
	msgs, _ := p["messages"].([]Message)
	if len(msgs) != 2 || msgs[0].Role != "system" || msgs[0].Content != "SYSTEM" || msgs[1].Role != "user" {
		t.Fatalf("messages inattendus : %+v", msgs)
	}
	if _, ok := p["tools"].([]Tool); !ok {
		t.Fatalf("outils absents du corps : %T", p["tools"])
	}

	// Sans système ni outils, on n'envoie que le message utilisateur minimal.
	p = prewarmPayload(ep, "", nil)
	msgs, _ = p["messages"].([]Message)
	if len(msgs) != 1 || msgs[0].Role != "user" {
		t.Fatalf("messages inattendus sans préfixe : %+v", msgs)
	}
	if _, ok := p["tools"]; ok {
		t.Fatal("la clé tools ne doit pas être envoyée quand il n'y a aucun outil")
	}
}

// Bout en bout : LLMPrewarm envoie une requête au moteur, mémorise le préfixe
// chauffé et ne le rejoue pas au sondage suivant.
func TestLLMPrewarmSendsOnce(t *testing.T) {
	t.Setenv("AJEAN_HOME", t.TempDir())
	if err := setAgentEnabled(true); err != nil {
		t.Fatal(err)
	}
	if err := SetConfigKey("MEM_MODE", "on"); err != nil {
		t.Fatal(err)
	}

	got := make(chan map[string]any, 4)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			http.NotFound(w, r)
			return
		}
		var body map[string]any
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &body)
		got <- body
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":""}}]}`))
	}))
	defer srv.Close()
	_, port, err := net.SplitHostPort(srv.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	if err := SetConfigKey("PORT", port); err != nil {
		t.Fatal(err)
	}
	// Réinitialise l'état global du pré-chauffage (les tests partagent le process).
	llmWarm.mu.Lock()
	llmWarm.healthy, llmWarm.sig, llmWarm.warmed, llmWarm.inflight = false, "", false, false
	llmWarm.attempts, llmWarm.nextTry, llmWarm.cancel = 0, time.Time{}, nil
	llmWarm.mu.Unlock()
	t.Cleanup(func() {
		llmWarm.mu.Lock()
		llmWarm.healthy, llmWarm.sig, llmWarm.warmed, llmWarm.inflight = false, "", false, false
		llmWarm.attempts, llmWarm.nextTry, llmWarm.cancel = 0, time.Time{}, nil
		llmWarm.mu.Unlock()
	})

	LLMPrewarm(true)
	select {
	case body := <-got:
		if body["max_tokens"] != float64(1) {
			t.Fatalf("max_tokens = %v, attendu 1", body["max_tokens"])
		}
	case <-time.After(5 * time.Second):
		t.Fatal("aucune requête de pré-chauffage reçue")
	}
	// Laisse la goroutine refermer son état (inflight) avant le second appel.
	deadline := time.Now().Add(2 * time.Second)
	for {
		llmWarm.mu.Lock()
		inflight := llmWarm.inflight
		llmWarm.mu.Unlock()
		if !inflight || time.Now().After(deadline) {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	LLMPrewarm(true)
	select {
	case <-got:
		t.Fatal("le même préfixe a été pré-chauffé deux fois")
	case <-time.After(300 * time.Millisecond):
	}
}
