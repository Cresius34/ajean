package ajean

import (
	"strings"
	"testing"
)

// bash ne doit JAMAIS être dédupliqué (relancer la même commande après avoir
// modifié un fichier est légitime) ; les outils d'écriture, si.
func TestDedupableToolExemptsBash(t *testing.T) {
	if dedupableTool("bash") {
		t.Error("bash ne doit pas être déduplicable : relancer la même commande est légitime")
	}
	for _, name := range []string{"edit", "write", "mem_edit", "web_read"} {
		if !dedupableTool(name) {
			t.Errorf("%s devrait rester déduplicable", name)
		}
	}
}

// Piste A (arbitrée le 2026-10-20) : la lecture d'un état VIVANT n'est jamais
// dédupliquée. Sans cette exemption, le blocage mesuré ce jour : au 3ᵉ mem_read
// identique dans le même tour, repeatedCallResult ne rend plus le contenu alors
// que la page mémoire est intacte sur disque (« rappel trop demandé »).
func TestDedupableToolExemptsLiveStateReads(t *testing.T) {
	for _, name := range []string{"mem_read", "mem_search", "recall", "recall_search", "task_list", "tracker", "inspect", "jean_read", "jean_search"} {
		if dedupableTool(name) {
			t.Errorf("%s ne doit PAS être déduplicable : sa source change entre deux appels identiques", name)
		}
	}
	// Les écritures mémoire/tâches gardent le court-circuit (les rejouer produit
	// une fausse erreur « old introuvable »).
	for _, name := range []string{"mem_add", "mem_edit", "mem_delete", "task_create", "task_update", "task_delete"} {
		if !dedupableTool(name) {
			t.Errorf("%s devrait rester déduplicable", name)
		}
	}
}

// Un appel d'outil redemandé à l'identique ne doit rendre son contenu qu'UNE
// fois, et l'avertissement doit être en TÊTE (collé après un résultat de
// plusieurs milliers de caractères, il passait inaperçu et le modèle rebouclait).
func TestRepeatedCallResultEscalates(t *testing.T) {
	page := strings.Repeat("contenu de la page ", 300)

	first := repeatedCallResult(page, 1)
	if !strings.HasPrefix(first, "[déjà fait]") {
		t.Fatalf("1re redemande : l'avertissement doit ouvrir le résultat, obtenu %.40q", first)
	}
	if !strings.Contains(first, page) {
		t.Fatal("1re redemande : le contenu doit encore être rendu")
	}

	for _, n := range []int{2, 3, 7} {
		again := repeatedCallResult(page, n)
		if strings.Contains(again, page) {
			t.Fatalf("redemande n°%d : le contenu ne doit plus être renvoyé", n)
		}
		if !strings.HasPrefix(again, "[déjà fait]") {
			t.Fatalf("redemande n°%d : avertissement manquant", n)
		}
	}
}
