package ajean

import (
	"reflect"
	"testing"
)

// checkToolPairs reproduit la règle du moteur : après un assistant porteur de
// tool_calls, une réponse `tool` par appel, dans l'ordre et sans interruption.
func checkToolPairs(t *testing.T, msgs []Message) {
	t.Helper()
	for i := 0; i < len(msgs); i++ {
		m := msgs[i]
		if m.Role == "tool" {
			t.Fatalf("réponse tool orpheline à l'index %d : %+v", i, m)
		}
		if m.Role != "assistant" || len(m.ToolCalls) == 0 {
			continue
		}
		for k, tc := range m.ToolCalls {
			if tc.ID == "" {
				t.Fatalf("appel %d de l'index %d sans id", k, i)
			}
			j := i + 1 + k
			if j >= len(msgs) || msgs[j].Role != "tool" || msgs[j].ToolCallID != tc.ID {
				t.Fatalf("appel %q (index %d) non suivi de sa réponse à l'index %d", tc.ID, i, j)
			}
		}
		i += len(m.ToolCalls) // saute le bloc qui vient d'être vérifié
	}
}

func TestRepairToolPairs(t *testing.T) {
	sys := Message{Role: "system", Content: "profil"}
	user := Message{Role: "user", Content: "bonjour"}
	tc := func(id, name string) ToolCall {
		return ToolCall{ID: id, Type: "function", Function: ToolCallFunc{Name: name, Arguments: "{}"}}
	}
	tool := func(id, out string) Message { return Message{Role: "tool", ToolCallID: id, Content: out} }
	// Média injecté par see_image / see_video : il coupait le bloc d'appels.
	media := Message{Role: "user", Content: []map[string]any{
		{"type": "text", "text": "Image :"},
		{"type": "image_url", "image_url": map[string]any{"url": "data:image/png;base64,AAAA"}},
	}}

	cases := []struct {
		name string
		in   []Message
		want []Message
	}{
		{
			name: "paire complète inchangée",
			in:   []Message{sys, user, {Role: "assistant", ToolCalls: []ToolCall{tc("call_1", "bash")}}, tool("call_1", "ok"), {Role: "assistant", Content: "fini"}},
			want: []Message{sys, user, {Role: "assistant", ToolCalls: []ToolCall{tc("call_1", "bash")}}, tool("call_1", "ok"), {Role: "assistant", Content: "fini"}},
		},
		{
			// Relevé en base le 2026-10-22 : un `write {}` sans aucune réponse,
			// dernier message de la conversation active.
			name: "appel unique sans réponse",
			in:   []Message{sys, user, {Role: "assistant", ToolCalls: []ToolCall{tc("call_00_h80I1oyYBSnaMTHXKD537077", "write")}}},
			want: []Message{sys, user, {Role: "assistant", ToolCalls: []ToolCall{tc("call_00_h80I1oyYBSnaMTHXKD537077", "write")}}, tool("call_00_h80I1oyYBSnaMTHXKD537077", toolInterruptedMarker)},
		},
		{
			// Deuxième forme relevée : deux appels, une seule réponse (stop entre les deux).
			name: "série partielle",
			in: []Message{
				sys, user,
				{Role: "assistant", ToolCalls: []ToolCall{tc("call_00_KBHfzaBD9l4po9ROg7SU7934", "panel_list_workflows"), tc("call_01_CWXhb9RtReVfTInNWEXN9009", "bash")}},
				tool("call_00_KBHfzaBD9l4po9ROg7SU7934", "[erreur] panneau injoignable"),
			},
			want: []Message{
				sys, user,
				{Role: "assistant", ToolCalls: []ToolCall{tc("call_00_KBHfzaBD9l4po9ROg7SU7934", "panel_list_workflows"), tc("call_01_CWXhb9RtReVfTInNWEXN9009", "bash")}},
				tool("call_00_KBHfzaBD9l4po9ROg7SU7934", "[erreur] panneau injoignable"),
				tool("call_01_CWXhb9RtReVfTInNWEXN9009", toolInterruptedMarker),
			},
		},
		{
			name: "réponse orpheline supprimée",
			in:   []Message{user, tool("call_inconnu", "résultat"), {Role: "assistant", Content: "salut"}},
			want: []Message{user, {Role: "assistant", Content: "salut"}},
		},
		{
			name: "id vide synthétisé de façon déterministe",
			in:   []Message{user, {Role: "assistant", ToolCalls: []ToolCall{{Type: "function", Function: ToolCallFunc{Name: "bash", Arguments: "{}"}}}}},
			want: []Message{user, {Role: "assistant", ToolCalls: []ToolCall{tc("call_repair_1_0", "bash")}}, tool("call_repair_1_0", toolInterruptedMarker)},
		},
		{
			name: "média intercalé replacé après le bloc",
			in: []Message{
				user,
				{Role: "assistant", ToolCalls: []ToolCall{tc("call_a", "see_image"), tc("call_b", "bash")}},
				tool("call_a", "image chargée"),
				media,
				tool("call_b", "ok"),
			},
			want: []Message{
				user,
				{Role: "assistant", ToolCalls: []ToolCall{tc("call_a", "see_image"), tc("call_b", "bash")}},
				tool("call_a", "image chargée"),
				tool("call_b", "ok"),
				media,
			},
		},
		{
			name: "system de tête et assistant simple préservés",
			in:   []Message{sys, user, {Role: "assistant", Content: "je réponds"}},
			want: []Message{sys, user, {Role: "assistant", Content: "je réponds"}},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := repairToolPairs(c.in)
			if !reflect.DeepEqual(got, c.want) {
				t.Fatalf("réparation inattendue\n obtenu : %+v\n voulu  : %+v", got, c.want)
			}
			checkToolPairs(t, got)
			// Idempotence : une seconde passe ne doit plus rien changer.
			if again := repairToolPairs(got); !reflect.DeepEqual(again, got) {
				t.Fatalf("non idempotente\n 1re passe : %+v\n 2e passe  : %+v", got, again)
			}
		})
	}
}

// TestRepairToolPairsNeModifiePasLEntree verrouille le contrat « copie » : la
// vue modèle peut être réparée pendant qu'un autre appareil la lit.
func TestRepairToolPairsNeModifiePasLEntree(t *testing.T) {
	in := []Message{
		{Role: "assistant", ToolCalls: []ToolCall{{ID: "", Type: "function", Function: ToolCallFunc{Name: "bash", Arguments: "{}"}}}},
	}
	_ = repairToolPairs(in)
	if in[0].ToolCalls[0].ID != "" {
		t.Fatalf("l'appel d'entrée a été modifié : id=%q", in[0].ToolCalls[0].ID)
	}
}
