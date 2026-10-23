package ajean

import "strings"

// Mode Jean : chaque outil exige un champ « description », une phrase qui dit à
// l'utilisateur ce que l'IA est en train de faire et pourquoi. Elle est affichée
// dans la bulle de l'outil et reste dans le fil (persistée avec le tool_used).
// Le champ est retiré des arguments avant l'exécution : les outils (MCP compris)
// ne le voient jamais.

const toolDescParam = "description"

// withToolDesc renvoie une copie des outils avec le champ description ajouté en
// premier paramètre requis.
func withToolDesc(tools []Tool) ([]Tool, map[string]bool) {
	out := make([]Tool, len(tools))
	set := map[string]bool{}
	for i, t := range tools {
		out[i] = t
		p, ok := t.Function.Parameters.(map[string]any)
		if !ok {
			p = map[string]any{"type": "object"}
		}
		np := make(map[string]any, len(p)+1)
		for k, v := range p {
			np[k] = v
		}
		props := map[string]any{}
		if old, ok := p["properties"].(map[string]any); ok {
			for k, v := range old {
				props[k] = v
			}
		}
		if _, taken := props[toolDescParam]; taken {
			continue // l'outil a déjà son propre champ description : on n'y touche pas
		}
		props[toolDescParam] = map[string]any{
			"type": "string",
			"description": "Une phrase claire, en français, destinée à l'utilisateur : ce que tu fais avec cet appel et pourquoi. " +
				"Exemple : « Je cherche dans ses notes la date de son rendez-vous chez le dentiste. »",
		}
		np["properties"] = props
		req := []string{toolDescParam}
		switch r := p["required"].(type) {
		case []string:
			req = append(req, r...)
		case []any:
			for _, x := range r {
				if s, ok := x.(string); ok {
					req = append(req, s)
				}
			}
		}
		np["required"] = req
		out[i].Function.Parameters = np
		set[t.Function.Name] = true
	}
	return out, set
}

// takeToolDesc retire la description des arguments et la renvoie, seulement
// pour un outil où on l'a ajoutée (set).
func takeToolDesc(set map[string]bool, name string, args map[string]any) string {
	if args == nil || !set[name] {
		return ""
	}
	d, _ := args[toolDescParam].(string)
	delete(args, toolDescParam)
	return strings.TrimSpace(d)
}
