package ajean

import (
	"strings"
	"testing"
)

func TestJeanMergeKeepsAllAndTriggers(t *testing.T) {
	testHome(t)
	if _, err := JeanSave("newsletter", "Toute demande sur la newsletter", "# Envoi\n- envoyer avec send.py"); err != nil {
		t.Fatal(err)
	}
	if _, err := JeanSave("desinscription-newsletter", "Retirer des adresses avant un envoi", "# Liste\n- lire unsub.csv avant chaque envoi"); err != nil {
		t.Fatal(err)
	}
	if _, err := JeanMerge("desinscription-newsletter", "newsletter"); err != nil {
		t.Fatal(err)
	}
	s, _ := jeanRead(ficheFile("newsletter"))
	for _, want := range []string{"lire unsub.csv avant chaque envoi", "envoyer avec send.py", "## desinscription newsletter", "aussi : Retirer des adresses"} {
		if !strings.Contains(s, want) {
			t.Errorf("fiche fusionnée sans %q :\n%s", want, s)
		}
	}
	if got, _ := jeanRead(ficheFile("desinscription-newsletter")); got != "" {
		t.Error("la fiche absorbée devait être archivée")
	}
	if got, _ := jeanRead(archiveFile("desinscription-newsletter")); got == "" {
		t.Error("archive absente")
	}
}

// Une fiche fusionnée plus grande que la limite ordinaire reste retouchable.
func TestJeanMergedFicheStaysEditable(t *testing.T) {
	testHome(t)
	big := "PASSAGE UNIQUE A RETIRER\n" + strings.Repeat("ligne de procédure utile numéro x\n", 400) // ~14 000 car.
	if _, err := JeanSave("grosse", "Faire la grosse tâche", big); err != nil {
		t.Fatal(err)
	}
	if _, err := JeanSave("autre-morceau", "Une partie annexe du travail", strings.Repeat("détail annexe à garder\n", 330)); err != nil {
		t.Fatal(err)
	}
	if _, err := JeanMerge("autre-morceau", "grosse"); err != nil {
		t.Fatal(err)
	}
	s, _ := jeanRead(ficheFile("grosse"))
	if n := len([]rune(parseFiche("grosse", s).Content)); n <= jeanFicheMaxChars {
		t.Fatalf("le test veut une fiche fusionnée au-delà de %d car., elle en fait %d", jeanFicheMaxChars, n)
	}
	if _, err := JeanPatch("grosse", "PASSAGE UNIQUE A RETIRER", ""); err != nil {
		t.Fatalf("retouche qui raccourcit refusée : %v", err)
	}
	s, _ = jeanRead(ficheFile("grosse"))
	f := parseFiche("grosse", s)
	if _, err := JeanSave("grosse", f.When, f.Content); err != nil {
		t.Fatalf("réécriture à l'identique refusée : %v", err)
	}
}
