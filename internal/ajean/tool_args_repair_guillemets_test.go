package ajean

// Rejoué le 2026-10-28 : comment mes trois `peer ask` ont-ils pu rendre la liste
// des voisins ? Le message contenait des guillemets doubles ASCII non échappés
// (j'écrivais « ajean peer ask cresius "test symetrie" » dans une chaîne JSON).
// Ce tableau rejoue les formes réellement émises, telles quelles, et regarde ce
// que la réparation en fait. But : établir le mécanisme, pas le deviner.
//
// Ce qui compte à la lecture du chemin d'appel (llm_client.go:1644-1654) :
//   - arguments INVALIDES et non réparables → badArgs → « arguments illisibles »,
//     message VISIBLE pour le modèle ;
//   - arguments VIDES → json.Valid("") est faux, mais TrimSpace("")=="" donc
//     badArgs n'est PAS posé : l'appel devient {} et est exécuté. `action` vide
//     → l'ancien repli rendait la liste. C'est le seul chemin connu vers le
//     silence observé.

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestReparationFormesEmisesVersListeMuette(t *testing.T) {
	formes := []struct {
		nom string
		s   string
	}{
		{"appel nu (aucun argument)", ""},
		{"objet vide", "{}"},
		{"action seule, message perdu", `{"action":"ask","peer":"bekka"}`},
		{"message long, guillemets ASCII imbriques",
			`{"action":"ask","peer":"bekka","message":"Deux verifications : « ajean peer ask cresius "test symetrie: repond juste echo" » doit repondre"}`},
		{"message long, guillemets imbriques en debut de phrase",
			`{"action":"ask","peer":"bekka","message":"appelle-moi depuis chez toi, un mot suffit. Si « reponse » arrive dis "recu" et le temps.`},
		{"chaine ouverte a la fin (coupure)",
			`{"action":"ask","peer":"bekka","message":"liste les outils que TU as chez toi (ceux que ton agent peut appeler)`},
	}
	for _, f := range formes {
		obj, ok := repairArgsObject(f.s)
		valide := json.Valid([]byte(f.s))
		action, present := obj["action"]
		_, msgPresent := obj["message"]
		t.Logf("%-46s json_valide=%-5v repare=%-5v action=%-6v (present=%-5v) message_present=%v",
			f.nom, valide, ok, presentAction(action, present), present, msgPresent)
		if !present && msgPresent {
			t.Logf("   => MASQUAGE : message present mais action perdue.")
		}
		if !present && !msgPresent && strings.TrimSpace(f.s) == "" {
			t.Logf("   => le chemin SILENCIEUX : arguments vides → badArgs non pose → execute avec {} → l'ancien repli rendait la liste.")
		}
	}
}

func presentAction(v any, present bool) string {
	if !present {
		return "ABSENTE"
	}
	s, _ := v.(string)
	if s == "" {
		return "VIDE"
	}
	return s
}
