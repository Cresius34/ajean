package ajean

// Verrouille la décision « un seul modèle d'embeddings pour tout l'appareil » :
// EMBD (EmbeddingGemma 2), plus aucun EmbeddingGemma 300M nulle part. Le 300M et
// EG2 n'occupent pas le même espace vectoriel : un fichier qui garderait l'ancien
// nom, ou un index étiqueté avec lui, mélangerait silencieusement deux espaces. La
// similarité resterait un nombre plausible, elle serait fausse. Ce fichier rend ça
// bruyant plutôt que discret.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ancienModele : écrit avec une concaténation pour que ce fichier ne se repère
// pas lui-même dans la recherche « plus aucune trace ».
const ancienModele = "embeddinggemma-" + "300M"

func TestEG2ModeleUnique(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("lire le répertoire du paquet : %v", err)
	}
	vus := 0
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".go") || name == "jean_embed_eg2_test.go" {
			continue
		}
		b, err := os.ReadFile(name)
		if err != nil {
			t.Fatalf("lire %s : %v", name, err)
		}
		if strings.Contains(string(b), ancienModele) {
			t.Errorf("%s mentionne encore l'ancien modèle", name)
		}
		vus++
	}
	if vus == 0 {
		t.Fatal("aucun fichier Go trouvé : le test ne vérifie rien")
	}

	// Le nom attendu, et lui seul, comme fichier modèle.
	if filepath.Base(jeanEmbedModelFile) != "embeddinggemma-2-Q8_0.gguf" {
		t.Errorf("jeanEmbedModelFile = %q", jeanEmbedModelFile)
	}
	if !strings.Contains(jeanEmbedModelURL, "ggml-org/embeddinggemma-2-GGUF") {
		t.Errorf("jeanEmbedModelURL = %q", jeanEmbedModelURL)
	}
	if jeanEmbedModelSize != 309855456 {
		t.Errorf("jeanEmbedModelSize = %d (fiche ggml-org à la révision épinglée)", jeanEmbedModelSize)
	}
	if len(jeanEmbedModelSHA) != 64 {
		t.Errorf("jeanEmbedModelSHA = %q", jeanEmbedModelSHA)
	}
	// L'étiquette de l'espace vectoriel doit avoir changé avec le modèle : c'est
	// elle qui jette l'ancien index au lieu de le relire comme s'il allait.
	if strings.Contains(jeanEmbedTag, "300") {
		t.Errorf("jeanEmbedTag = %q : l'index de l'ancien modèle serait relu", jeanEmbedTag)
	}
}

// TestEG2CheminsParDefaut : les deux fournisseurs (llama.cpp et transformers)
// doivent retomber sur EG2, pas sur un dépôt Hugging Face au hasard.
func TestEG2CheminsParDefaut(t *testing.T) {
	for _, provider := range []string{"llama_cpp", "", "transformers"} {
		_, model := embeddingDefaultPaths(provider)
		base := filepath.Base(model)
		if provider == "transformers" {
			if base != "embeddinggemma-2-full" {
				t.Errorf("provider %q : modèle par défaut %q", provider, base)
			}
			continue
		}
		if base != "embeddinggemma-2-Q8_0.gguf" {
			t.Errorf("provider %q : modèle par défaut %q", provider, base)
		}
	}
}

// TestEG2UnSeulModelePourDeuxRecherches : le journal de Jean et la mémoire
// sémantique doivent désigner le MÊME fichier. Autrement, deux modèles tournent
// sur la machine et deux espaces coexistent sans que personne le voie.
func TestEG2UnSeulModelePourDeuxRecherches(t *testing.T) {
	_, memModel := embeddingDefaultPaths("llama_cpp")
	if filepath.Base(jeanEmbedModelFile) != filepath.Base(memModel) {
		t.Fatalf("deux modèles différents : journal=%q mémoire=%q", jeanEmbedModelFile, memModel)
	}
	// Le chemin du journal suit la configuration de la mémoire quand le fichier
	// porte le bon nom (un EG2 posé ailleurs sert aux deux).
	if filepath.Base(jeanEmbedModelPath()) != jeanEmbedModelFile {
		t.Errorf("jeanEmbedModelPath() = %q", jeanEmbedModelPath())
	}
}

// TestEG2Prefixes : les préfixes Instruction-Tuned suivent la nomenclature
// officielle d'EG2 (task pour la requête, text pour le document). Sans eux, la
// qualité chute sans la moindre erreur visible.
func TestEG2Prefixes(t *testing.T) {
	e := jeanEntry{Text: "une entrée du journal"}
	if got := jeanEmbedDocText(e); !strings.HasPrefix(got, "title: none | text: ") {
		t.Errorf("préfixe document = %q", got)
	}
	if got := jeanEmbedQueryText("vidange Clio"); got != "task: search result | query: vidange Clio" {
		t.Errorf("requête = %q", got)
	}
	// Le texte du document passe par la troncature : au-delà, l'entrée est coupée
	// mais le préfixe doit rester intact. clipRunes ajoute une ellipse, d'où +1.
	const prefix = "title: none | text: "
	long := jeanEmbedDocText(jeanEntry{Text: strings.Repeat("a", jeanEmbedDocChars+500)})
	if !strings.HasPrefix(long, prefix) {
		t.Error("préfixe document abîmé par la troncature")
	}
	if got := len([]rune(long)); got != jeanEmbedDocChars+len(prefix)+1 {
		t.Errorf("troncature = %d caractères, attendu %d", got, jeanEmbedDocChars+len(prefix)+1)
	}
}

// TestEG2IndexEtiqueteRejete : un index écrit sous l'étiquette de l'ancien modèle
// ne doit jamais être relu pour EG2. C'est le point le plus dangereux de la
// bascule, parce que tous les autres contrôles passent et que rien ne prévient.
func TestEG2IndexEtiqueteRejete(t *testing.T) {
	// Un contenu valide, construit par le code lui-même, puis ré-étiqueté avec
	// l'ancienne chaîne : seul l'en-tête change, le reste est un index d'EG2.
	valide := formatEmbedIndex(map[string][]float32{"a1": normVec([]float64{1, 2, 3})})
	if len(parseEmbedIndex(valide)) != 1 {
		t.Fatal("l'index écrit par le code ne s'est pas relui")
	}
	header := strings.SplitN(valide, "\n", 2)[0]
	ancien := strings.Replace(valide, header, "jean-embed egemma300-q8-v1", 1)
	if ancien == valide {
		t.Fatal("la substitution d'en-tête n'a rien changé")
	}
	if m := parseEmbedIndex(ancien); len(m) != 0 {
		t.Fatalf("index de l'ancien modèle relu : %d entrées", len(m))
	}
	// Le va-et-vient écriture/lecture doit s'accorder.
	if back := parseEmbedIndex(formatEmbedIndex(map[string][]float32{
		"a1": normVec([]float64{1, 2, 3}), "b2": normVec([]float64{4, 5, 6})})); len(back) != 2 {
		t.Fatalf("aller-retour index : %d entrées retrouvées", len(back))
	}
}
