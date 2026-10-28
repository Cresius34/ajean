package ajean

// Non-régression : un appel `peer` dont l'action est perdue en route ne doit pas
// se muer en liste muette quand un message était pourtant demandé.
//
// FAUTIF MESURÉ le 2026-10-28, deux fois de suite, depuis CE poste : j'appelle
// ask avec un message de plusieurs lignes, l'outil rend LA LISTE DES VOISINS —
// sans un mot d'erreur. Vue d'en face comme vue de moi, la sortie ressemble à une
// réponse du canal ; c'est mon propre outil qui a laissé tomber la question pour
// répondre à autre chose. Le coupable exact n'est pas établi (deux hypothèses
// ouvertes : le JSON de l'appel émis avec des guillemets doubles ASCII imbriqués
// dans le message, que sanitizeArgs recoud en perdant les clés qui suivaient ; ou
// l'appel tronqué à la limite de sortie). Ce qui est établi, c'est le MASQUAGE :
// `action` absente tombait sur « list », un état de repos qui n'a aucune raison
// d'être le sort d'une question.
//
// Le correctif ne prétend pas réparer l'appel abîmé : il refuse de le laisser
// passer pour un succès.

import (
	"strings"
	"testing"
)

func TestPeerToolAskSansActionExplicite(t *testing.T) {
	parallelTestHome(t)

	// 1. Un message présent sans action = ask. On ne peut pas aller jusqu'au
	//    réseau ici (aucun voisin joignable dans le bac à sable), mais on peut
	//    prouver qu'il ne part pas dans la liste, et qu'il ne se tait pas.
	got := runPeerTool(map[string]any{"message": "test, réponds juste echo"})
	if strings.Contains(got, "voisins déclarés") {
		t.Fatalf("un ask sans action explicite a rendu la liste des voisins : %q", got)
	}
	if !strings.Contains(got, "voisin") {
		t.Fatalf("le refus doit désigner le voisin visé, pas un succès muet : %q", got)
	}

	// 2. Rien demandé du tout = la liste, sans appel réseau. C'est le cas d'usage
	//    d'origine, qu'il ne faut pas casser en passant tout au ask.
	liste := runPeerTool(map[string]any{})
	if !strings.Contains(liste, "aucun voisin") {
		t.Fatalf("sans argument, la liste des voisins doit répondre : %q", liste)
	}

	// 3. Avec un voisin déclaré, ask sans message doit le dire — pas relancer la
	//    question dans le vide.
	if _, err := peerAddDeclared("voisine", "http://192.168.1.99:18190/v1", "sk-cle", ""); err != nil {
		t.Fatal(err)
	}
	vide := runPeerTool(map[string]any{"action": "ask", "peer": "voisine", "message": "   "})
	if !strings.Contains(vide, "message vide") {
		t.Fatalf("ask sans message doit le dire : %q", vide)
	}
}
