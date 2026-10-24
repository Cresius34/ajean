package ajean

// chat_tool_pairs.go — garantir l'appariement assistant(tool_calls) / tool.
//
// Le moteur (llama.cpp, DeepSeek, tout endpoint OpenAI strict) exige qu'un
// message assistant porteur de tool_calls soit IMMÉDIATEMENT suivi d'une réponse
// `tool` par tool_call_id. Observé en usage réel (2026-10-22) : un stop pendant
// une série d'appels laissait dans la vue modèle l'assistant avec TOUS ses appels
// et une partie seulement des réponses. Persistée, cette demi-paire faisait
// refuser tous les tours suivants (400 « An assistant message with 'tool_calls'
// must be followed by tool messages responding to each 'tool_call_id' ») : la
// conversation devenait définitivement inutilisable, « Vider le contexte » étant
// le seul recours. Deux ids orphelins ont été relevés dans ajean.db, dont un en
// dernier message de la conversation active.
//
// La réparation est appliquée à l'envoi ET à la persistance : elle répare donc
// aussi les conversations déjà empoisonnées par une version antérieure.

import "fmt"

// toolInterruptedMarker remplace une réponse d'outil qui n'a jamais eu lieu.
// Même registre que « [erreur] » : le modèle doit comprendre que l'appel n'a pas
// abouti, et surtout ne pas le rejouer en croyant à un échec de l'outil.
const toolInterruptedMarker = "[interrompu] l'appel d'outil n'a pas été exécuté (arrêt demandé)."

// repairToolPairs renvoie une COPIE de msgs où chaque assistant porteur de
// tool_calls est suivi d'exactement une réponse `tool` par appel, dans l'ordre
// des appels. Les réponses présentes sont conservées telles quelles ; celles qui
// manquent reçoivent toolInterruptedMarker.
//
// Trois autres formes sont normalisées au passage :
//   - un id d'appel vide reçoit un id déterministe (call_repair_<i>_<k>) ;
//   - les messages `user` multimodaux injectés par see_image / see_video, qui
//     s'intercalaient entre deux réponses (assistant(a,b) → tool(a) → user(image)
//     → tool(b)), sont replacés APRÈS le bloc `tool` ;
//   - un `tool` sans appel correspondant est supprimé (il n'a plus de sens).
//
// Idempotente : repairToolPairs(repairToolPairs(x)) == repairToolPairs(x).
func repairToolPairs(msgs []Message) []Message {
	if len(msgs) == 0 {
		return msgs
	}
	out := make([]Message, 0, len(msgs))
	for i := 0; i < len(msgs); i++ {
		m := msgs[i]
		if m.Role == "tool" {
			// Atteint seulement hors d'un bloc d'appels : réponse orpheline.
			continue
		}
		if m.Role != "assistant" || len(m.ToolCalls) == 0 {
			out = append(out, m)
			continue
		}
		// Ids attendus, en ordre d'appel. Un id vide est synthétisé à partir de la
		// position : stable d'une passe à l'autre, donc idempotent.
		calls := append([]ToolCall(nil), m.ToolCalls...)
		want := make([]string, len(calls))
		known := make(map[string]bool, len(calls))
		for k := range calls {
			id := calls[k].ID
			if id == "" {
				id = fmt.Sprintf("call_repair_%d_%d", i, k)
				calls[k].ID = id
			}
			want[k] = id
			known[id] = true
		}
		m.ToolCalls = calls
		out = append(out, m)

		// Ramasse ce qui suit et appartient encore au bloc : les réponses des
		// appels, plus les messages média du harnais qui les coupaient.
		got := make(map[string]Message, len(calls))
		var deferred []Message
		for i+1 < len(msgs) {
			n := msgs[i+1]
			if n.Role == "tool" {
				if !known[n.ToolCallID] {
					break // réponse d'un autre appel : traitée (et supprimée) au tour suivant
				}
				if _, dup := got[n.ToolCallID]; !dup {
					got[n.ToolCallID] = n
				}
				i++
				continue
			}
			if hasMediaParts(n) {
				deferred = append(deferred, n)
				i++
				continue
			}
			break
		}
		for _, id := range want {
			if r, ok := got[id]; ok {
				out = append(out, r)
				continue
			}
			out = append(out, Message{Role: "tool", ToolCallID: id, Content: toolInterruptedMarker})
		}
		out = append(out, deferred...)
	}
	return out
}

// hasMediaParts : message `user` multimodal (image/vidéo injectée par le
// harnais, ou pièce jointe). Le contenu est tantôt construit en mémoire
// ([]map[string]any), tantôt relu depuis la base ([]any).
func hasMediaParts(m Message) bool {
	if m.Role != "user" {
		return false
	}
	estMedia := func(p any) bool {
		part, ok := p.(map[string]any)
		if !ok {
			return false
		}
		switch part["type"] {
		case "image_url", "input_video":
			return true
		}
		return false
	}
	switch c := m.Content.(type) {
	case []map[string]any:
		for _, p := range c {
			if estMedia(p) {
				return true
			}
		}
	case []any:
		for _, p := range c {
			if estMedia(p) {
				return true
			}
		}
	}
	return false
}
