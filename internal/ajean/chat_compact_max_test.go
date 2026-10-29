package ajean

import "testing"

// Plafond du seuil de compactage (#142) : sans réglage, 75 % de la fenêtre ;
// avec, le plus bas des deux ; un plafond minuscule est relevé.
func TestCompactThresholdCap(t *testing.T) {
	testHome(t)
	set := func(k, v string) {
		t.Helper()
		if err := SetConfigKey(k, v); err != nil {
			t.Fatal(err)
		}
	}
	set("CTX", "1000000")
	set("COMPACT_MAX", "")
	if got := compactThreshold(); got != 750000 {
		t.Fatalf("sans plafond : %d, attendu 750000", got)
	}
	set("COMPACT_MAX", "32768")
	if got := compactThreshold(); got != 32768 {
		t.Fatalf("plafond 32768 : %d", got)
	}
	// fenêtre plus petite que le plafond : la fenêtre décide
	set("CTX", "32768")
	if got := compactThreshold(); got != 24576 {
		t.Fatalf("petite fenêtre : %d, attendu 24576", got)
	}
	set("CTX", "1000000")
	set("COMPACT_MAX", "100")
	if got := compactThreshold(); got != 8192 {
		t.Fatalf("plafond minuscule non relevé : %d", got)
	}
	// le seuil plafonné déclenche bien la compaction
	set("COMPACT_MAX", "32768")
	if !compactWouldTrigger(nil, 40000) || compactWouldTrigger(nil, 20000) {
		t.Fatalf("déclenchement incohérent avec le plafond")
	}
}

// COMPACT_MAX est un réglage de l'appareil : il survit à un changement de preset.
func TestCompactMaxPreserved(t *testing.T) {
	if !isPreservedKey("COMPACT_MAX") {
		t.Fatal("COMPACT_MAX serait effacé par un changement de preset")
	}
	if !isPreservedKey("COMPACT_FRAC") {
		t.Fatal("COMPACT_FRAC serait effacé par un changement de preset")
	}
}

// Le pourcentage du seuil (COMPACT_FRAC). C'est LE levier pour une grosse
// fenêtre : sur 262 144 de contexte, le plafond en jetons proposé par l'interface
// (jusqu'à 128K) ne peut que compacter plus tôt — aucune de ses valeurs n'atteint
// 75 % de la fenêtre. On vérifie ici qu'on peut compacter PLUS TARD.
func TestCompactFracPercent(t *testing.T) {
	testHome(t)
	set := func(k, v string) {
		t.Helper()
		if err := SetConfigKey(k, v); err != nil {
			t.Fatal(err)
		}
	}
	set("CTX", "262144")
	set("COMPACT_MAX", "")

	// sans réglage : le comportement d'origine, 75 %
	set("COMPACT_FRAC", "")
	if got := compactThreshold(); got != 196608 {
		t.Fatalf("défaut : %d, attendu 196608 (75 %% de 262144)", got)
	}
	if got := compactFracPercent(); got != 75 {
		t.Fatalf("frac affiché : %d, attendu 75", got)
	}

	// plus haut : 90 % de la fenetre, encore sous l'ecretement de securite
	set("COMPACT_FRAC", "90")
	if got := compactThreshold(); got != 235929 { // int(262144 x 0,90)
		t.Fatalf("90 %% : %d, attendu 235929", got)
	}

	// 100 % est ecrete par la marge : jamais la fenetre pleine, sinon un gros
	// resultat d'outil deborde AVANT que le compactage se declenche.
	set("COMPACT_FRAC", "100")
	if got := compactThreshold(); got != 249037 { // 262144 - 13107 (5 %)
		t.Fatalf("100 %% : %d, attendu 249037 (fenetre - marge de 5 %%)", got)
	}

	// plus bas que le défaut : l'écrêtement ne remonte rien, on descend bien
	set("COMPACT_FRAC", "50")
	if got := compactThreshold(); got != 131072 {
		t.Fatalf("50 %% : %d, attendu 131072", got)
	}

	// hors bornes et non numerique : on garde le defaut, on compacte pas a l'aveugle
	for _, v := range []string{"101", "49", "0", "-5", "beaucoup"} {
		set("COMPACT_FRAC", v)
		if got := compactThreshold(); got != 196608 {
			t.Fatalf("COMPACT_FRAC=%q : %d, attendu 196608 (repli sur le defaut)", v, got)
		}
	}

	// le suffixe % tolere a la saisie : 90% doit donner 90 %, pas le defaut.
	// (un essai a 75% n'aurait rien prouve : c'est deja le defaut.)
	set("COMPACT_FRAC", "90%")
	if got := compactThreshold(); got != 235929 {
		t.Fatalf("COMPACT_FRAC=90%%%% : %d, attendu 235929", got)
	}

	// le pourcentage et le plafond en jetons se COMBINENT : le plus bas gagne,
	// et le plafond garde son rôle sur les petites fenêtres / petits modèles.
	set("COMPACT_FRAC", "90")
	set("COMPACT_MAX", "32768")
	if got := compactThreshold(); got != 32768 {
		t.Fatalf("frac 90 + plafond 32768 : %d, attendu 32768", got)
	}

	// fenêtre minuscule : la marge ne peut pas manger toute la fenêtre, et le
	// seuil ne doit jamais remonter au-dessus d'elle (le plancher de 8192 est
	// dans compactMaxTokens, pas ici).
	set("COMPACT_MAX", "")
	set("CTX", "8192")
	set("COMPACT_FRAC", "100")
	if got := compactThreshold(); got >= 8192 || got <= 0 {
		t.Fatalf("fenêtre 8192 à 100 %% : %d, attendu un seuil sous la fenêtre", got)
	}
}
