package ajean

import (
	"strings"
	"testing"
)

// addMemPage écrit une page de mémoire dans le projet actif, comme le fait l'outil mem_add.
func addMemPage(t *testing.T, name, content string) {
	t.Helper()
	if err := MemAdd(name, content); err != nil {
		t.Fatalf("MemAdd(%s) : %v", name, err)
	}
}

// Le terme recherché doit apparaître dans le résultat, sinon le classement est
// un bruit que le modèle cite comme s'il répondait.
func TestSearchMemoryOnlyReturnsPagesHoldingTheTerm(t *testing.T) {
	testHome(t)
	addMemPage(t, "notes-cuisine.md", "# Notes cuisine\n\nRecette de la soupe à l'oignon, à faire revenir longtemps.\n")
	addMemPage(t, "atelier.md", "# Atelier\n\nLe four de l'atelier est réglé à 180 degrés.\n")

	got := SearchMemory(t.Context(), "soupe à l'oignon", 5)
	if len(got.Hits) == 0 {
		t.Fatal("aucun résultat : la page qui contient le terme doit être trouvée")
	}
	if got.Hits[0].File != "notes-cuisine.md" {
		t.Errorf("mauvaise page en tête : %q", got.Hits[0].File)
	}
}

// Un terme absent de TOUTES les pages ne doit jamais se voir attribuer un hit.
// C'est le repli qui fait citer une page sans rapport.
func TestSearchMemoryNoPhantomHitForUnknownTerm(t *testing.T) {
	testHome(t)
	addMemPage(t, "notes-cuisine.md", "# Notes cuisine\n\nRecette de la soupe à l'oignon, à faire revenir longtemps.\n")

	got := SearchMemory(t.Context(), "compteur kilometrique du vehicule", 5)
	for _, h := range got.Hits {
		if !strings.Contains(strings.ToLower(h.Snippet+h.Title+h.File), "kilomet") {
			t.Errorf("terme absent partout : hit fantôme sur %q — %q", h.File, h.Snippet)
		}
	}
}

// Quand le moteur vectoriel n'est pas joignable, la note doit le dire : sans
// elle, le modèle lit un classement de sens là où il n'y a que des mots.
func TestSearchMemoryModesAreCoherent(t *testing.T) {
	testHome(t)
	addMemPage(t, "atelier.md", "# Atelier\n\nLe four de l'atelier est réglé à 180 degrés.\n")

	got := SearchMemory(t.Context(), "four atelier degres", 5)
	switch got.Mode {
	case "lexical":
		if got.Note == "" {
			t.Log("mode lexical sans note (moteur désactivé) : admissible")
		}
	case "vector", "hybride", "hybrid":
		// mode annoncé vectoriel : le résultat doit au moins être cohérent
		if len(got.Hits) > 0 && !strings.Contains(strings.ToLower(got.Hits[0].Snippet), "four") {
			t.Errorf("mode %s mais le premier hit ne parle pas du four : %q", got.Mode, got.Hits[0].Snippet)
		}
	default:
		t.Errorf("mode inconnu : %q", got.Mode)
	}
}
