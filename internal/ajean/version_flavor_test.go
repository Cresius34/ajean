package ajean

import (
	"encoding/json"
	"os"
	"testing"
)

// TestVersionFlavorWindowsSuit : la ressource Windows du build portable
// (cmd/ajean-memory-test/versioninfo.json) doit annoncer exactement la même
// version que displayVersion(). Le lanceur du TEST (Lancer-Test.ps1) compare
// ProductVersion à la valeur attendue ; une divergence rend le paquet refusé,
// comme ce fut le cas en 0.8.0 (constante à jour, ressource ancienne).
func TestVersionFlavorWindowsSuit(t *testing.T) {
	BuildFlavor = "embedding-test"
	t.Cleanup(func() { BuildFlavor = "" })
	want := displayVersion()

	b, err := os.ReadFile("../../cmd/ajean-memory-test/versioninfo.json")
	if err != nil {
		t.Fatal(err)
	}
	var vi struct {
		StringFileInfo struct{ FileVersion, ProductVersion string }
	}
	if err = json.Unmarshal(b, &vi); err != nil {
		t.Fatal(err)
	}
	for name, got := range map[string]string{
		"StringFileInfo.ProductVersion": vi.StringFileInfo.ProductVersion,
		"StringFileInfo.FileVersion":    vi.StringFileInfo.FileVersion,
	} {
		if got != want {
			t.Errorf("%s = %q, attendu %q — pense à `go generate ./cmd/ajean-memory-test` après un bump", name, got, want)
		}
	}
	if want != Version+buildFlavorSuffix {
		t.Errorf("displayVersion() = %q, attendu %q", want, Version+buildFlavorSuffix)
	}
	// Le suffixe doit rester un identifiant de version semver exploitable.
	if len(buildFlavorSuffix) < 2 || buildFlavorSuffix[0] != '+' {
		t.Errorf("buildFlavorSuffix = %q, doit commencer par '+' (portable, sans '+')", buildFlavorSuffix)
	}
}

// TestVersionFlavorPlainEntry : le point d'entrée normal (cmd/ajean) n'ajoute
// aucun suffixe : Version seule.
func TestVersionFlavorPlainEntry(t *testing.T) {
	BuildFlavor = ""
	if got := displayVersion(); got != Version {
		t.Errorf("displayVersion() sans flavor = %q, attendu %q", got, Version)
	}
}
