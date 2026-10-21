Tâches et consolidation de Jean plus fiables, accès internet simplifié.

## Corrections

- **Tâche planifiée qui coupait une réponse en cours** : une tâche épinglée sur un autre preset rechargeait le moteur avant de vérifier si une conversation était en train de répondre. La réponse était coupée, la tâche renonçait, et son modèle restait actif. Désormais la tâche attend la fin de la génération sans rien toucher, puis rend le preset d'avant une fois terminée. Une tâche épinglée sur un autre modèle provoque donc deux chargements à chaque passage (aller et retour).
- **Sommeil de Jean qui coupait une réponse** : même défaut la nuit, la bascule vers le preset du sommeil se faisait avant de vérifier l'occupation. La vérification passe maintenant en premier.
- **Consolidation perdue sur mémoire verrouillée** : après un démarrage à froid avec mémoire chiffrée, la consolidation de 3 h échouait puis n'était plus jamais retentée. Elle est maintenant reprogrammée, et le déverrouillage de la mémoire la relance.
- **Consolidation décalée par la mise en veille** : le minuteur suivait une horloge qui s'arrête pendant la veille (WSL2, Mac, Linux), et partait en retard de toute la durée de la veille, parfois hors de la nuit. Il compare maintenant l'heure réelle à son échéance.
- **Machine éteinte la nuit : consolidation jamais faite** : sur un poste éteint le soir et rallumé le matin, la consolidation était reportée à chaque démarrage au 3 h suivant, lui aussi manqué. Une nuit manquée est maintenant rattrapée environ 10 minutes après le démarrage, une seule fois. Une machine allumée la nuit ne voit aucune différence.
- **Contexte du moteur MoE** : avec la carte d'aide en mode hors RAM, le moteur plafonne le contexte à 131 072 jetons, mais le curseur des réglages permettait d'aller jusqu'à 262 144 sans effet, et le preset affichait une valeur jamais appliquée. Le curseur s'arrête maintenant au maximum réel de la configuration choisie, avec une explication, et le preset enregistre la valeur qui tourne.

## Changement

- **Accès internet : moteur intégré uniquement** : le serveur externe Crawl4AI n'est plus pris en charge. Le moteur web intégré, qui ne demande aucune installation, est désormais le seul. Les réglages de serveur (URL, clé, choix du moteur) disparaissent de l'interface et de la commande `ajean internet`, qui se limite à `on`, `off` et `status`. Une installation qui utilisait Crawl4AI passe automatiquement au moteur intégré. Les pages entièrement rendues en JavaScript ne sont plus lisibles.

## Non testé

- Le rattrapage de la consolidation et le minuteur après une veille sont couverts par des tests automatiques, mais n'ont pas encore été observés sur une vraie nuit.

## Mise à jour

```
ajean update
```
