Mode Jean : on voit ce qu'il fait, et son sommeil range vraiment la mémoire.

## Nouveautés

- **Chaque action de Jean est décrite** : en mode Jean, chaque outil utilisé s'accompagne d'une phrase qui dit ce qu'il fait et pourquoi (« Je regarde l'image de midi pour voir si la caméra est propre »). Les étapes restent dans le fil sous forme de liste sobre ; l'étape en cours s'affiche sur la ligne d'état.
- **Terminal en direct dans la scène** : en plein écran, quand Jean lance une commande, une fenêtre de terminal s'ouvre au-dessus de lui. La commande s'écrit lettre par lettre, puis le résultat apparaît.
- **Résultats de recherche web dans la scène** : une recherche ouvre une carte avec la requête et les résultats (site, titre, extrait), qui arrivent en cascade.
- **Heure du sommeil réglable et sommeil à la demande** : l'heure à laquelle Jean range sa mémoire se choisit dans sa mémoire (onglet Sommeil), et un bouton « Dormir maintenant » le lance tout de suite. Pendant le sommeil, Jean dort dans la scène.
- **Historique des sommeils** : l'onglet Sommeil montre, pour chacun des 14 derniers sommeils, ce qui a changé fichier par fichier, avec un bouton « Revenir à avant ce sommeil ». Les fiches archivées y sont listées et peuvent être ressorties.

## Améliorations

- **Sommeil plus efficace** : sur une mémoire volumineuse, le sommeil ne voyait que la liste des fiches et ne fusionnait jamais rien. Les fiches qui se ressemblent lui sont maintenant données en entier, par groupes, et un outil de fusion verse une fiche entière dans une autre avant de l'archiver. Sur une mémoire réelle de 33 fiches, le sommeil en fusionne 9, sans perte d'information.
- **Sommeil plus sûr** : un sommeil interrompu (message de l'utilisateur, redémarrage) est entièrement annulé ; aucun fichier ne peut être écrit pendant le sommeil ; son bilan est toujours noté dans le journal.
- **Mémoire de Jean en onglets** : Profil, Leçons, Fiches, Sommeil et Journal se choisissent avec un sélecteur, au lieu d'une longue page à faire défiler.

## Corrections

- **Images cassées dans les réponses** : une image citée par son seul nom alors qu'elle est rangée dans un sous-dossier est retrouvée. Jean a aussi pour consigne de ne citer que des fichiers existants.

## Mise à jour

```
ajean update
```

Testé sur un serveur Linux avec un modèle local. Le sommeil v2 a été validé sur des copies d'une mémoire réelle, pas encore sur plusieurs nuits consécutives.
