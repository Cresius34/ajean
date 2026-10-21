package ajean

import "testing"

// La dédup ne doit resservir un résultat que si le contexte n'a pas été réécrit
// depuis (même ère). Après une compaction, le contenu que la dédup rappelait a
// été résumé ou effacé : le resservir ne rend rien au modèle (mesuré en usage
// réel — il conclut à une panne et relance l'appel).
func TestDedupStillHoldsOnlyWithinSameEpoch(t *testing.T) {
	memo := callMemo{result: "le contenu", epoch: 0}

	if !dedupStillHolds(memo, 0) {
		t.Error("même ère : le résultat doit encore pouvoir être resservi")
	}
	if dedupStillHolds(memo, 1) {
		t.Error("ère révolue : le contenu n'est plus dans le contexte, la dédup ne doit pas s'opposer")
	}
}
