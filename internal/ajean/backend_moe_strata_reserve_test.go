package ajean

// Le reglage de reserve VRAM affiche dans l'application etait ignore par le
// moteur : celui-ci remet « user_args » par-dessus « args » a chaque demarrage.
// Ces tests figent le geste de reparation et, surtout, ce qu'il ne doit PAS
// toucher : un drapeau mis a la main par la personne.
// 2026-10-29.

import (
	"encoding/json"
	"reflect"
	"testing"
)

// base avec la signature reelle d'un model prepare par le moteur : la
// preparation y a pose la reserve (setup.py:def explicit_args), et un drapeau
// ajoute a la main par la personne survit de puis.
func strataReserveFixture() map[string]any {
	return map[string]any{
		"exe":  "/x/engine-hip/strata.exe",
		"args": []any{"--pack", "/p", "--prefill", "16384", "--vram-reserve-mib", "700"},
		"user_args": []any{"--vram-reserve-mib", "4096",
			"--control-vector", "0.4", "-0.3", "--spec", "5"},
		"args_written": []any{"--pack", "/p", "--prefill", "16384", "--vram-reserve-mib", "700"},
	}
}

func TestStrataReserveSurvivesUserArgs(t *testing.T) {
	base := strataReserveFixture()
	before, _ := json.Marshal(base)

	c := map[string]string{"MOE_BACKEND": "hip", "MOE_VRAM_RESERVE": "2560", "CTX": "86016"}
	out, err := moeStrataBuildConfig(base, c, "")
	if err != nil {
		t.Fatal(err)
	}

	// 1. « args » porte notre valeur.
	args := moeArgs(t, out)
	if got := argAfter(args, "--vram-reserve-mib"); got != "2560" {
		t.Fatalf("args : --vram-reserve-mib = %q, attendu 2560 (%v)", got, args)
	}

	// 2. « user_args » porte desormais NOTRE valeur : le moteur remet cette liste
	//    par-dessus « args », donc c'est elle qui part a l'engine. C'est ce qui
	//    rend notre reglage effectif (user_args.py:def set_flag, L129-134).
	ua := strataAsStrings(out["user_args"])
	if got := argAfter(ua, "--vram-reserve-mib"); got != "2560" {
		t.Fatalf("user_args : --vram-reserve-mib = %q, attendu 2560 (%v)", got, ua)
	}

	// 3. ce que la personne a mis a la main est intact, dans l'ordre.
	if !reflect.DeepEqual(ua[:4], []string{"--control-vector", "0.4", "-0.3", "--spec"}) {
		t.Fatalf("drapeaux mis a la main deplaces : %v", ua)
	}
	if argAfter(ua, "--spec") != "5" {
		t.Fatalf("--spec de la personne perdu : %v", ua)
	}
	if got := argAfter(ua, "--control-vector"); got != "0.4" {
		t.Fatalf("vecteur de controle abime : %v", ua)
	}

	// 4. « args_written » recopie « args » tel qu'on vient de l'ecrire (stamp),
	//    sinon la prochaine preparation du moteur prendrait nos valeurs pour des
	//    reglages mis a la main.
	if !reflect.DeepEqual(strataAsStrings(out["args_written"]), args) {
		t.Fatalf("args_written != args : %v vs %v", strataAsStrings(out["args_written"]), args)
	}

	// 5. la config d'origine n'est pas modifiee (meme exigence qu'au test voisin).
	after, _ := json.Marshal(base)
	if string(before) != string(after) {
		t.Fatal("base mutée")
	}
}

func TestStrataReserveLeftAloneWhenUnset(t *testing.T) {
	base := strataReserveFixture()
	// Aucun reglage demande : on ne touche rien, la valeur de la personne gagne.
	out, err := moeStrataBuildConfig(base, map[string]string{"MOE_BACKEND": "hip"}, "")
	if err != nil {
		t.Fatal(err)
	}
	ua := strataAsStrings(out["user_args"])
	if argAfter(ua, "--vram-reserve-mib") != "4096" {
		t.Fatalf("user_args devrait garder 4096 : %v", ua)
	}
	if got := argAfter(moeArgs(t, out), "--vram-reserve-mib"); got != "700" {
		t.Fatalf("args ne devait pas bouger : %q", got)
	}
	if !reflect.DeepEqual(strataAsStrings(out["args_written"]),
		[]string{"--pack", "/p", "--prefill", "16384", "--vram-reserve-mib", "700"}) {
		t.Fatalf("args_written modifie a tort : %v", strataAsStrings(out["args_written"]))
	}
}

func TestStrataVisionOffRemovesFlagFromUserArgs(t *testing.T) {
	base := map[string]any{
		"exe":       "/x/strata.exe",
		"args":      []any{"--pack", "/p", "--vision", "--vram-reserve-mib", "700"},
		"user_args": []any{"--vision"},
	}
	out, err := moeStrataBuildConfig(base, map[string]string{"MOE_BACKEND": "hip"}, "")
	if err != nil {
		t.Fatal(err)
	}
	if hasArg(moeArgs(t, out), "--vision") {
		t.Fatal("--vision reste dans args")
	}
	if hasArg(strataAsStrings(out["user_args"]), "--vision") {
		t.Fatalf("--vision reste dans user_args : %v", strataAsStrings(out["user_args"]))
	}
	if _, ok := out["user_args"]; ok {
		t.Fatal("user_args vide aurait du etre retire")
	}
}
