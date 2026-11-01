package ajean

import (
	"encoding/json"
	"testing"
)

// TestStreamChunkLitLeChampRefusés : le moteur (Strata #3796) rend les appels d'outil
// refusés sous "strata_rejected_tool_calls", à la RACINE du chunk final — lequel a
// "choices":[] sur ce build. Si le nom du champ ne colle pas exactement, on ne lit
// rien et le tour reste silencieux : c'est ce que ce test garde sous contrôle.
func TestStreamChunkLitLeChampRefuses(t *testing.T) {
	// Forme relevée dans le journal du moteur le 2026-11-01 (mem_grep, web_foo).
	const chunk = `{"id":"c1","object":"chat.completion.chunk","choices":[],` +
		`"usage":{"prompt_tokens":164268,"completion_tokens":91,"total_tokens":164359},` +
		`"strata_rejected_tool_calls":[{"name":"mem_grep","reason":"undeclared"},` +
		`{"name":"web_foo","reason":"undeclared"}]}`
	var c streamChunk
	if err := json.Unmarshal([]byte(chunk), &c); err != nil {
		t.Fatalf("chunk refuse : %v", err)
	}
	if len(c.Rejected) != 2 {
		t.Fatalf("Rejected = %d, voulu 2 (le nom du champ ne colle pas ?) — chunk: %s", len(c.Rejected), chunk)
	}
	if c.Rejected[0].Name != "mem_grep" || c.Rejected[1].Name != "web_foo" {
		t.Errorf("noms refuses = %q, %q", c.Rejected[0].Name, c.Rejected[1].Name)
	}
	if c.Rejected[0].Reason != "undeclared" {
		t.Errorf("reason = %q, voulu \"undeclared\"", c.Rejected[0].Reason)
	}
	// Les voisins ne doivent pas casser : usage et timings restent lus.
	if c.Usage == nil || c.Usage.PromptTokens != 164268 {
		t.Errorf("usage perdu sur un chunk qui porte le champ refuses: %+v", c.Usage)
	}
	if len(c.Choices) != 0 {
		t.Errorf("choices voulu vide, got %d", len(c.Choices))
	}

	// Un chunk sans appels refusés (le cas normal) : rien d'inventé.
	var vide streamChunk
	if err := json.Unmarshal([]byte(`{"choices":[{"delta":{"content":"bonjour"},"finish_reason":""}]}`), &vide); err != nil {
		t.Fatalf("chunk ordinaire refuse : %v", err)
	}
	if len(vide.Rejected) != 0 {
		t.Errorf("chunk ordinaire : Rejected = %d, voulu 0", len(vide.Rejected))
	}
	if vide.Choices[0].Delta.Content != "bonjour" {
		t.Error("le contenu d'un chunk ordinaire doit rester lu")
	}
}

// TestOutilInconnuAlimenteLaReInjection : la brique de lecture et celle du message
// doivent se brancher — le nom refuse doit rendre la liste des outils du domaine.
func TestOutilInconnuAlimenteLaReInjection(t *testing.T) {
	for _, nm := range []string{"mem_grep", "web_foo"} {
		msg := outilInconnu(nm)
		if msg == "" {
			t.Errorf("outilInconnu(%q) rend vide", nm)
		}
		t.Logf("%s -> %s", nm, msg)
	}
}
