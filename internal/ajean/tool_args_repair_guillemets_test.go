package ajean

// Rejoué le 2026-10-28 : est-ce comme ça que mes deux `peer ask` ont pu rendre la
// liste des voisins ? Le message contenait des guillemets doubles ASCII NON
// échappés (j'écrivais « … "test symetrie: repond juste echo" … » dans une chaîne
// JSON). Ce test ne prouve pas ce qui s'est passé sur le réseau — il montre ce
// que la réparation fait de cette forme, et c'est la seule hypothèse qui rend
// compte d'une action devenue absente sans erreur.

import (
	"strings"
	"testing"
)

func TestReparationArgsAvecGuillemetsImbriques(t *testing.T) {
	casse := `{"action":"ask","peer":"bekka","message":"verifie deux choses : 1) « ajean peer ask cresius "test symetrie: repond juste echo" » doit repondre"}`

	obj, ok := repairArgsObject(casse)
	t.Logf("reparation ok=%v", ok)
	if !ok {
		t.Logf("aucune reparation : l'appel serait signale comme illisible (badArgs)")
		return
	}
	for _, k := range []string{"action", "peer", "message"} {
		v, present := obj[k]
		t.Logf("  cle %-8s presente=%v valeur=%v", k, present, v)
	}
	if _, present := obj["action"]; !present {
		t.Log("=> CONFIRME LE MASQUAGE : « action » a disparu de l'appel reconstitue.")
		t.Log("   runPeerTool recevait alors action=\"\" → l'ancien repli rendait la liste, sans erreur.")
	}
	if m, ok := obj["message"].(string); ok && !strings.Contains(m, "symetrie") {
		t.Log("=> le message a aussi ete tronque par la couture.")
	}
}
