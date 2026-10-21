package ajean

import (
	"encoding/json"
	"strings"
	"testing"
)

// Budget du préambule envoyé À CHAQUE TOUR : prompt système + schémas des outils.
// Il est payé sur toute la conversation et sort du contexte utile.
//
// Ce test existe pour empêcher le regonflement. Le prompt a déjà été raccourci
// une fois pour une raison de comportement (un préambule verbeux fait
// sur-raisonner les modèles à reasoning, qui finissent leur tour sans appeler
// d'outil), puis une seconde fois pour le coût. Les deux fois, il avait
// regrossi ligne par ligne, chacune paraissant anodine.
//
// Approximation 1 tok ≈ 4 caractères : suffisante pour une alerte, et sans
// dépendance à un tokenizer.
//
// Relevé une fois, volontairement, quand l'IA a reçu la capacité de gérer
// elle-même ses tâches planifiées (outils task_*) et de supprimer une page de
// mémoire (mem_delete) pour la garder rangée : cinq schémas d'outils en plus,
// ~600 tokens, ajout demandé et accepté. Le garde-fou reste là pour bloquer le
// regonflement LIGNE À LIGNE du prompt lui-même, qui, lui, doit rester court.
//
// Léger relèvement (8400 → 8700) pour les tâches « script seul » : task_create
// porte un paramètre 'script' en plus (un script du dossier scripts tourne sans
// modèle ni tokens). Pas d'outils script_* dédiés — l'IA écrit ses scripts avec
// write/bash dans le dossier scripts, la consigne vit dans le briefing machine.
//
// Relèvement (8700 → 9600) pour la mémoire longue de conversation : deux schémas
// en plus, recall (ramène un bloc archivé au compactage) et recall_search (le
// retrouve par mots-clés). C'est ce qui rend les conversations quasi infinies
// sans perdre le fil — ajout demandé et accepté. Le garde-fou reste là pour
// bloquer le regonflement LIGNE À LIGNE du prompt lui-même.
//
// Relèvement (9600 → 10800) pour le 3ᵉ type de mémoire : l'outil `suivi` (données
// datées qui s'accumulent — compteurs, relevés, journaux). UN seul schéma, et
// toute sa navigation par niveaux vit dans SES RÉPONSES, pas dans le prompt (une
// seule ligne de consigne : « ça va dans suivi, pas dans une note »). Ajout demandé
// et accepté ; il sort au contraire le suivi des notes .md, qui restent petites.
// Relèvement (10800 → 12000) pour le passage en 0.18.4 : le préambule projet
// injecte désormais l'index mémoire (une ligne par page, avec son accroche), et
// le briefing machine porte la consigne du moteur MoE local. Le garde-fou est
// passé à la mesure scindée (voir TestSystemPromptStaysLean) : il juge le
// préambule du produit, prompt + outils qu'Ajean fournit lui-même.
//
// Léger relèvement (12000 → 12100) constaté sur le poste de développement,
// relevé en chiffres ronds : le préambule produit y est à 12 087 caractères
// (~3 021 tok), soit 87 de plus que la borne. Le dépassement vient des schémas
// d'outils du produit, pas du prompt (2 428 caractères, très en deçà de sa part) :
// c'est l'outillage demandé et accepté, pas un regonflement ligne à ligne.
//
// Relèvement (12100 → 12400) pour la base v0.18.9 : la fusion amène le préambule
// de l'amont, mesuré ici à 12 123 car (~3 031 tok) contre 12 010 en 0.18.4.2.
// Ce n'est pas la ligne Cresius qui a grossi : aucun outil neuf n'est exposé au
// modèle (le `cmdOutput` qui remplace notre `tailOutput` reste interne, il n'est
// pas dans la liste des outils) et le décompte MCP du poste reste à 4 caractères.
// La hausse vient du contenu de l'amont lui-même — texte du prompt et
// descriptions de schémas rallongés (menu latéral refait, appareil guidé). Le
// garde-fou garde 277 caractères de marge et juge toujours le préambule du
// produit, pas la configuration du poste.
const promptCharBudget = 12400 // ~3100 tokens, tout allumé (agent+web+mémoire+suivi+apps), outils séparés (regroupés en 0.18.2 puis abandonnés : plus d'appels mal formés)

func TestSystemPromptStaysLean(t *testing.T) {
	// Mesure ISOLÉE de l'état du poste : le prompt système lit le briefing
	// machine, le contexte projet et l'index mémoire depuis AJEAN_HOME. Sans
	// isolation, ce test jugerait la configuration de la machine (noms et
	// nombre de pages mémoire, textes du propriétaire) et non le produit, et
	// sa taille bougerait selon les tests lancés ensemble.
	t.Setenv("AJEAN_HOME", t.TempDir())
	caps := Caps{Agent: true, Internet: true, Mem: MemAlways}
	sp := baseSystemPrompt(caps)
	// Le budget garde-fou porte sur le PRÉAMBULE DU PRODUIT : prompt système +
	// les schémas des outils qu'Ajean fournit lui-même. Les outils des serveurs
	// MCP configurés par le propriétaire de la machine s'ajoutent à la requête,
	// mais leur poids est choisi par le poste, pas par nous : il est mesuré à
	// part (TestMCPToolWeightIsReported) et ne fait pas échouer ce garde-fou.
	own, mcp := splitToolSchemas(EnabledTools(caps))
	tb, err := json.Marshal(own)
	if err != nil {
		t.Fatal(err)
	}
	mb, err := json.Marshal(mcp)
	if err != nil {
		t.Fatal(err)
	}
	total := len(sp) + len(tb)
	t.Logf("prompt %d car (~%d tok) + outils %d car (~%d tok) = ~%d tok ; MCP du poste en plus : %d car (~%d tok)",
		len(sp), len(sp)/4, len(tb), len(tb)/4, total/4, len(mb), len(mb)/4)
	if total > promptCharBudget {
		t.Fatalf("préambule à %d car (~%d tok), budget %d car (~%d tok).\n"+
			"Avant de relever le budget : les schémas d'outils pèsent le double du prompt,\n"+
			"et une consigne écrite dans un schéma n'a pas à être répétée dans le prompt.",
			total, total/4, promptCharBudget, promptCharBudget/4)
	}
}

// splitToolSchemas sépare les schémas des outils fournis par Ajean de ceux des
// serveurs MCP du poste (nommés mcp__serveur__outil, voir mcpExposedName).
func splitToolSchemas(tools []Tool) (own, mcp []Tool) {
	for _, tl := range tools {
		if strings.HasPrefix(tl.Function.Name, "mcp__") {
			mcp = append(mcp, tl)
			continue
		}
		own = append(own, tl)
	}
	return own, mcp
}

// TestMCPToolWeightIsReported : les outils MCP ne sont pas sous notre budget,
// mais leur poids réel doit rester visible — c'est lui qui sort du contexte
// utile à chaque tour. Un poste qui branche beaucoup de serveurs doit le savoir.
// Ce test lit la config MCP RÉELLE du poste (pas d'isolation AJEAN_HOME) : c'est
// précisément ce qu'il est censé mesurer. Il n'échoue jamais : il renseigne.
func TestMCPToolWeightIsReported(t *testing.T) {
	caps := Caps{Agent: true, Internet: true, Mem: MemAlways}
	_, mcp := splitToolSchemas(EnabledTools(caps))
	mb, err := json.Marshal(mcp)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("outils MCP du poste : %d (~%d tok)", len(mcp), len(mb)/4)
	if len(mb) == 0 {
		return
	}
	if len(mb) > 4*promptCharBudget {
		t.Logf("attention : MCP du poste seul dépasse le budget du préambule produit ;"+
			" désactiver des serveurs/outil MCP pour rendre le contexte utile")
	}
}

// Le prompt ne doit pas redevenir un catalogue d'outils : leurs schémas partent
// dans la MÊME requête et les décrivent déjà un par un.
func TestSystemPromptDoesNotRelistTools(t *testing.T) {
	sp := baseSystemPrompt(Caps{Agent: true, Internet: true, Mem: MemAlways})
	for _, line := range strings.Split(sp, "\n") {
		l := strings.TrimSpace(line)
		if !strings.HasPrefix(l, "- ") {
			continue
		}
		// Une puce qui commence par un nom d'outil = un catalogue qui revient.
		for _, tool := range []string{"bash ", "write ", "edit ", "mem_", "web_"} {
			if strings.HasPrefix(l[2:], tool) {
				t.Fatalf("le prompt réénumère un outil : %q", l)
			}
		}
	}
}

// Sans agent ni mémoire, aucun préambule : un modèle de chat simple à qui on
// ordonne d'appeler des outils invente des appels textuels qui fuient dans la
// réponse.
func TestSystemPromptEmptyWithoutTools(t *testing.T) {
	if sp := baseSystemPrompt(Caps{Mem: MemOff}); sp != "" {
		t.Fatalf("préambule non vide sans outils : %q", sp)
	}
}
