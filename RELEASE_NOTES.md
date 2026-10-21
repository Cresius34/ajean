Mode Jean : une mémoire qui revient d'elle-même, qui retient les correctifs, et un contexte gardé propre.

## Nouveautés

- **Rappel automatique de la mémoire** : à chaque message, Jean retrouve la fiche et les souvenirs du journal qui correspondent nettement à la demande et les joint au message. Les seuils sont stricts : rien n'est ajouté pour un message court ou sans lien (un remerciement, un poème), et une fiche n'est donnée en entier que lorsqu'elle correspond clairement. Une fiche coupée le signale, avec le moyen de lire la suite.
- **Mémoire des correctifs** : quand une commande échoue puis qu'une variante proche réussit, le correctif est noté dans le journal. Si la même erreur revient plus tard, le résultat de la commande rappelle en une ligne ce qui avait marché.
- **Recherche par codes exacts** : la recherche dans la mémoire tient compte des codes d'erreur, noms de fichiers, adresses IP et identifiants écrits à l'identique, en plus de la recherche par le sens.
- **Recherche par le sens active par défaut** : la recherche par le sens dans le journal de Jean est désormais activée, et son modèle se télécharge automatiquement lorsqu'un journal existe. Elle peut être coupée dans les réglages.
- **Reprise après une pause** : après une longue inactivité, le nouveau contexte commence par un court rappel du dernier échange.
- **Consolidation au fait des erreurs récurrentes** : la consolidation nocturne reçoit les erreurs rencontrées plusieurs fois, pour en tirer une fiche ou une leçon.

## Améliorations

- **Contexte allégé avant tout résumé** : quand la conversation approche de la limite, les anciens rappels de mémoire et les longs résultats d'outils des anciens échanges sont d'abord raccourcis (début et fin conservés, texte entier archivé et relisible). Le résumé, plus coûteux et moins précis, n'a lieu que si cela ne suffit pas.

## Mise à jour

```
ajean update
```
