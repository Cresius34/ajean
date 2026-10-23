package ajean

import (
	"strings"
	"testing"
)

func TestCUGuardErrorPage(t *testing.T) {
	out := capCUOutput("[ok] cliqué [6]\n\n# Page Not Found - Boutique\nhttps://exemple.fr/x\n\n3 élément(s)")
	if !strings.Contains(out, "page d'erreur ou de blocage") {
		t.Fatalf("avertissement absent :\n%s", out)
	}
	if out := capCUOutput("# Acheter un téléphone - Boutique\nhttps://exemple.fr"); strings.Contains(out, "page d'erreur") {
		t.Fatal("faux positif sur une page normale")
	}
}
