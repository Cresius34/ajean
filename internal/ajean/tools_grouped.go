package ajean

// tools_grouped.go — appels d'outils REGROUPÉS par domaine (memory, task,
// recall, web, browser, see, jean, file, avec un paramètre `action`).
//
// La 0.18.2 envoyait au modèle un outil par domaine ; c'est abandonné (le modèle
// appelait nettement plus souvent de travers) et il revoit les outils séparés.
// Restent les conversations de cette période, et un modèle qui imiterait leurs
// appels : expandToolCall retraduit un appel regroupé vers l'outil interne AVANT
// tout le reste, et toolCallLegacy relit l'historique sous les noms internes.

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

type toolGroup struct {
	name, intro string
	// action → outil interne, dans l'ordre d'affichage.
	members [][2]string
	// renommage de paramètres pour un membre (conflit avec `action`) :
	// outil interne → {nom interne: nom dans le groupe}.
	rename map[string]map[string]string
}

var toolGroups = []toolGroup{
	{name: "file", intro: "Write files on disk (read them with bash).",
		members: [][2]string{{"write", "write"}, {"edit", "edit"}}},
	{name: "memory", intro: "Persistent memory pages of the current project.",
		members: [][2]string{{"search", "mem_search"}, {"grep", "mem_grep"}, {"read", "mem_read"}, {"add", "mem_add"}, {"edit", "mem_edit"}, {"delete", "mem_delete"}}},
	{name: "task", intro: "Your scheduled tasks: they run later on their own.",
		members: [][2]string{{"list", "task_list"}, {"create", "task_create"}, {"update", "task_update"}, {"delete", "task_delete"}}},
	{name: "recall", intro: "Blocks archived when this conversation was compacted.",
		members: [][2]string{{"read", "recall"}, {"search", "recall_search"}}},
	{name: "web", intro: "Internet access.",
		members: [][2]string{{"search", "web_search"}, {"images", "web_images"}, {"open", "web_open"}, {"read", "web_read"}, {"grep", "web_grep"}}},
	{name: "browser", intro: "Drive a real web browser on this machine.",
		members: [][2]string{{"open", "browser_open"}, {"snapshot", "browser_snapshot"}, {"find", "browser_find"}, {"click", "browser_click"}, {"click_xy", "browser_click_xy"},
			{"type", "browser_type"}, {"key", "browser_key"}, {"scroll", "browser_scroll"}, {"screenshot", "browser_screenshot"}}},
	{name: "see", intro: "Look at an image or a video file from disk.",
		members: [][2]string{{"image", "see_image"}, {"video", "see_video"}}},
	{name: "jean", intro: "Your own memory as Jean: profile, fiches, journal, lessons, rules, projects.",
		members: [][2]string{{"search", "jean_search"}, {"remember", "jean_remember"}, {"forget", "jean_forget"}, {"note", "jean_note"}, {"save", "jean_save"},
			{"read", "jean_read"}, {"lesson", "jean_lesson"}, {"patch", "jean_patch"}, {"rule", "jean_rule"}, {"projects", "jean_projects"},
			{"project_mem", "jean_project_mem"}, {"project_file", "jean_project_file"}},
		rename: map[string]map[string]string{"jean_rule": {"action": "rule_action"}}},
}

// groupOf : groupe et action d'un outil interne.
var groupOf = func() map[string][2]string {
	m := map[string][2]string{}
	for _, g := range toolGroups {
		for _, mb := range g.members {
			m[mb[1]] = [2]string{g.name, mb[0]}
		}
	}
	return m
}()

// expandToolCall traduit un appel d'outil de domaine vers l'outil interne et
// ses arguments. Un outil hors groupe passe tel quel. errMsg non vide : action
// manquante ou inconnue (l'appel n'est pas exécuté).
func expandToolCall(name string, args map[string]any) (string, map[string]any, string) {
	var g *toolGroup
	for i := range toolGroups {
		if toolGroups[i].name == name {
			g = &toolGroups[i]
		}
	}
	if g == nil {
		return name, args, ""
	}
	act := strings.ToLower(strings.TrimSpace(str(args["action"])))
	// see : l'action se déduit de l'extension quand elle manque.
	// recall(id) sans action (comme le citent les résumés de compactage) = lire.
	if act == "" && g.name == "recall" {
		act = "read"
	}
	if act == "" && g.name == "see" {
		act = "image"
		if videoExt(str(args["file"])) {
			act = "video"
		}
	}
	legacy := ""
	names := []string{}
	for _, mb := range g.members {
		names = append(names, mb[0])
		if mb[0] == act {
			legacy = mb[1]
		}
	}
	if legacy == "" {
		return name, args, fmt.Sprintf("[erreur] %s : action %q inconnue. Actions : %s.", name, act, strings.Join(names, ", "))
	}
	out := map[string]any{}
	for k, v := range args {
		if k != "action" {
			out[k] = v
		}
	}
	for in, ext := range g.rename[legacy] {
		if v, ok := out[ext]; ok {
			delete(out, ext)
			out[in] = v
		}
	}
	return legacy, out, ""
}

// legacyToolName : nom interne d'après le nom de domaine et l'action (même
// partielle, pendant la frappe). "" si on ne sait pas encore.
func legacyToolName(name, action string) string {
	if _, ok := groupOf[name]; ok {
		return name
	}
	for _, g := range toolGroups {
		if g.name != name {
			continue
		}
		for _, mb := range g.members {
			if mb[0] == strings.ToLower(strings.TrimSpace(action)) {
				return mb[1]
			}
		}
		return ""
	}
	return name
}

// outilInconnu : le message rendu quand un tool_call ne correspond à aucun outil
// annoncé. Le modèle appelle parfois un nom plausible bâti par analogie : « mem_grep »
// était de ceux-là, calqué sur web_grep (cité 6 fois dans le journal du moteur) — il
// est déclaré depuis le 2026-11-01, mais la classe reste : le nom seul n'aide pas, le
// modèle retente la même variante ou se rabat sur un outil sans rapport. On rend donc
// le périmètre visé — les outils du groupe dont le préfixe (mem_, web_, browser_…)
// ouvre l'appel — et, à défaut, la liste
// des domaines. Coût : quelques lignes, seulement sur un tour déjà raté.
func outilInconnu(name string) string {
	s := "[erreur] outil inconnu: " + name
	lower := strings.ToLower(strings.TrimSpace(name))
	vus := map[string]bool{}
	var membres []string
	for _, g := range toolGroups {
		for _, mb := range g.members {
			i := strings.Index(mb[1], "_")
			if i <= 0 {
				continue
			}
			if !strings.HasPrefix(lower, mb[1][:i+1]) {
				continue
			}
			for _, other := range g.members {
				if !vus[other[1]] {
					vus[other[1]] = true
					membres = append(membres, other[1])
				}
			}
			break
		}
	}
	if len(membres) == 0 {
		var domaines []string
		for _, g := range toolGroups {
			domaines = append(domaines, g.name)
		}
		return s + " — domaines d'outils annoncés : " + strings.Join(domaines, ", ")
	}
	return s + " — outils de ce domaine : " + strings.Join(membres, ", ")
}

// memeNomsRefuses : la liste des noms refusés est-elle identique à celle du tour
// précédent ? Le moteur les rend dans l'ordre d'apparition, ce qui dépend de la
// frappe du modèle : on compare donc des ensembles, triés, sans doublon.
//
// Pourquoi ce garde (arbitré par l'humain le 2026-11-01, voie « amont ») : la
// relance avec la liste des outils valides a été mesurée inefficace. Le journal
// du moteur montre deux refus AUX MÊMES NOMS (`L32104` puis `L32180` :
// `mem_grep, web_foo` à chaque fois) — le modèle a réécrit exactement le même
// appel après avoir reçu la liste. Chaque tour gaspillé repaie le préambule
// complet (mesuré : 12 232 caractères, ~3058 tokens) plus l'historique.
//
// La relance garde sa première chance : un nom inconnu croisé pour la première
// fois reçoit bien la liste. Elle est coupée seulement quand le même nom revient,
// là où rien ne dit qu'une autre formulation aurait fait mieux. L'amont, face à
// un appel d'outil fantôme, ne relance pas davantage : « on ignore donc l'appel :
// le modèle répond en texte » (llm_client.go:1292-1298).
func memeNomsRefuses(precedent, courant []string) bool {
	if len(precedent) == 0 || len(courant) == 0 {
		return false
	}
	a := normaliseNoms(precedent)
	b := normaliseNoms(courant)
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// normaliseNoms : noms en minuscules, sans doublon, triés — comparaison
// d'ensembles, insensible à l'ordre et aux répétitions dans un même tour.
func normaliseNoms(noms []string) []string {
	vus := map[string]bool{}
	out := make([]string, 0, len(noms))
	for _, n := range noms {
		k := strings.ToLower(strings.TrimSpace(n))
		if k == "" || vus[k] {
			continue
		}
		vus[k] = true
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func videoExt(f string) bool {
	switch strings.ToLower(filepath.Ext(f)) {
	case ".mp4", ".mov", ".webm", ".mkv", ".avi", ".m4v":
		return true
	}
	return false
}

// toolCallLegacy : un appel d'outil de l'historique, ramené à l'outil interne
// (pour les relectures de l'historique qui cherchent par exemple les mem_read).
func toolCallLegacy(tc ToolCall) (string, map[string]any) {
	var args map[string]any
	_ = json.Unmarshal([]byte(tc.Function.Arguments), &args)
	n, a, _ := expandToolCall(tc.Function.Name, args)
	return n, a
}
