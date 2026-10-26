package ajean

// Outils exemptés de déduplication (piste A, arbitrée le 2026-10-20).
//
// La dédup répond d'une seule réponse à trois questions indépendantes : le
// résultat est-il reproductible ? est-il encore dans la fenêtre ? le modèle a-t-il
// besoin de le revoir ? Pour les outils qui lisent un état VIVANT (mémoire de
// session, rappels archivés, tâches planifiées, tracker), la première réponse est
// fausse : la source change sous les pieds de l'appel (mem_edit puis mem_read,
// task_create puis task_list). Les exempter ne coûte rien — ils n'écrivent rien,
// donc les rejouer ne produit ni doublon ni fausse erreur — et supprime le blocage
// observé : à la 3ᵉ demande identique, repeatedCallResult ne rend plus le contenu
// alors que le stockage sur disque est intact.
//
// Les ÉCRITURES (mem_add / mem_edit / mem_delete, write, edit, task_*) restent
// dédupliquées : rejouer une écriture produit des erreurs « old introuvable » qui
// font dérailler le modèle.

// liveStateReadTools : lecture d'état vivant → jamais de rappel mémorisé.
// « tracker » est là pour sa navigation (view) ; ses écritures (add/edit/delete)
// passent par les mêmes arguments, mais rejouter un ajout déjà fait est refusé
// par le moteur lui-même — ce n'est pas la dédup qui protège ici.
var liveStateReadTools = map[string]bool{
	"mem_read":      true,
	"mem_search":    true,
	"recall":        true,
	"recall_search": true,
	"task_list":     true,
	"tracker":       true,
	"inspect":       true,
	// Le canal Ajean↔Ajean est un état VIVANT de l'autre côté : un voisin a sa
	// propre mémoire et sa propre conversation. Rejouer une demande identique
	// depuis le cache supprimerait l'échange — deux fois la même question n'a
	// jamais la même réponse d'une autre IA.
	"peer": true,
	// Le mode Jean lit la même mémoire que mem_* : mêmes raisons.
	"jean_read":   true,
	"jean_search": true,
}

// isLiveStateRead dit si un outil lit un état qui peut avoir changé entre deux
// appels identiques du même tour.
func isLiveStateRead(name string) bool {
	return liveStateReadTools[name]
}
