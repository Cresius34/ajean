package ajean

import "testing"

// TestMemeNomsRefusesCoupeLaRelance : le garde B. Mesuré le 2026-11-01 dans le
// journal du moteur : deux refus AUX MÊMES NOMS (`mem_grep, web_foo` à L32104
// puis à L32180) — le modèle réécrivait exactement le même appel après avoir reçu
// la liste des outils valides. Chaque relance repaie le préambule complet.
// Donc : mêmes noms → on ne relance plus.
func TestMemeNomsRefusesCoupeLaRelance(t *testing.T) {
	courants := []string{"mem_grep", "web_foo"}
	if !memeNomsRefuses(courants, courants) {
		t.Errorf("mêmes noms : doit refuser de relancer")
	}
	// Insensible à l'ordre (le moteur rend les noms dans l'ordre de frappe).
	if !memeNomsRefuses([]string{"web_foo", "mem_grep"}, courants) {
		t.Errorf("mêmes noms dans un autre ordre : doit refuser de relancer")
	}
	// Insensible aux doublons dans un même tour.
	if !memeNomsRefuses([]string{"mem_grep", "web_foo", "mem_grep"}, courants) {
		t.Errorf("doublon en plus : doit refuser de relancer")
	}
	// Insensible à la casse.
	if !memeNomsRefuses([]string{"MEM_GREP", "web_foo"}, courants) {
		t.Errorf("casse différente : doit refuser de relancer")
	}
}

// La relance garde sa première chance : un nom croisé pour la première fois, ou
// un nom qui change, reçoit bien la liste des outils valides.
func TestNomIneditGardeSaRelance(t *testing.T) {
	courants := []string{"mem_grep", "web_foo"}
	cas := map[string][]string{
		"premier tour, aucune référence": nil,
		"référence vide":                 {},
		"un seul des deux noms":          {"mem_grep"},
		"nom inconnu different":          {"read"},
		"arguments colles au nom":        {"mem_search=query"},
		"un nom de plus":                 {"mem_grep", "web_foo", "read"},
		"un nom de moins":                {"mem_grep"},
	}
	for nom, precedents := range cas {
		if memeNomsRefuses(precedents, courants) {
			t.Errorf("%s : doit relancer (la relance garde sa valeur)", nom)
		}
	}
}

// TestNormaliseNoms : la comparaison est un ensemble, pas une séquence.
func TestNormaliseNoms(t *testing.T) {
	got := normaliseNoms([]string{"Web_Foo", " mem_grep ", "", "mem_grep", "web_foo"})
	if len(got) != 2 || got[0] != "mem_grep" || got[1] != "web_foo" {
		t.Errorf("normaliseNoms = %q, voulu [mem_grep web_foo]", got)
	}
	if len(normaliseNoms(nil)) != 0 {
		t.Errorf("nil doit rendre vide")
	}
	if len(normaliseNoms([]string{"", "  "})) != 0 {
		t.Errorf("noms vides doivent rendre vide")
	}
}
