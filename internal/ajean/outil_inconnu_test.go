package ajean

import (
	"strings"
	"testing"
)

// TestOutilInconnuNommeLeDomaine : un nom bâti par analogie (mem_grep, calqué sur
// web_grep) doit rendre les outils du domaine visé, pas le seul nom rejeté.
func TestOutilInconnuNommeLeDomaine(t *testing.T) {
	got := outilInconnu("mem_grep")
	t.Log("mem_grep -> " + got)
	for _, want := range []string{"mem_search", "mem_read", "mem_add", "mem_edit", "mem_delete"} {
		if !strings.Contains(got, want) {
			t.Errorf("mem_grep: %q absent du message: %s", want, got)
		}
	}
	if strings.Contains(got, "web_grep") {
		t.Errorf("mem_grep ne doit pas enumerer les outils d'autres domaines: %s", got)
	}

	// web_foo : meme mecanisme, autre domaine.
	got = outilInconnu("web_foo")
	t.Log("web_foo -> " + got)
	if !strings.Contains(got, "web_open") {
		t.Errorf("web_foo: web_open absent: %s", got)
	}
	if strings.Contains(got, "mem_read") {
		t.Errorf("web_foo ne doit pas enumerer mem_read: %s", got)
	}

	// Hors domaine : la liste des domaines, pour ne rien inventer.
	got = outilInconnu("zzz_chose")
	t.Log("zzz_chose -> " + got)
	if !strings.Contains(got, "domaines") || !strings.Contains(got, "memory") {
		t.Errorf("nom hors domaine doit lister les domaines: %s", got)
	}

	// Le nom rejeté reste visible : sans lui, le modele ne sait pas ce qu'il a tapé.
	if !strings.Contains(outilInconnu("mem_grep"), "mem_grep") {
		t.Error("le nom appele doit rester dans le message")
	}
}
