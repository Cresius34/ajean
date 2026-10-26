Corrections de stabilité : conversation du mode Jean, presets modifiés et moteur ExLlama.

## Corrections

- **Mode Jean : conversation remplacée par une version vieille de plusieurs jours.** Si la conversation enregistrée ne pouvait pas être lue au démarrage (par exemple juste après une mise à jour, quand l'ancien processus tient encore la base), AJEAN repartait d'une conversation vide sans le signaler, puis rouvrait une ancienne copie du fil Jean et l'écrivait par-dessus la conversation à jour. Le fil Jean enregistré est maintenant toujours préféré à cette ancienne copie. Une conversation illisible est mise de côté (copie de secours) avant toute réécriture, et la lecture au démarrage est retentée plus longtemps. Merci pour le rapport détaillé (#175).
- **Modifier le preset actif restait sans effet.** Après l'enregistrement, le moteur continuait à tourner avec l'ancienne configuration, même après un redémarrage, jusqu'à passer par un autre preset. Enregistrer une modification du preset actif propose maintenant de l'appliquer tout de suite (#176).
- **ExLlama : « chargement… » affiché pendant une réponse.** Pendant la lecture d'un long prompt, le moteur ExLlama tarde à répondre aux vérifications d'état, et l'interface concluait à tort que le modèle était en cours de chargement. Un moteur qui a répondu récemment est maintenant considéré comme occupé, pas en chargement.

## Conseil ExLlama

Une répartition manuelle des cartes trop serrée (champ « répartition GPU » du preset) peut remplir une carte à quelques Mo près et faire échouer la lecture des longues conversations par manque de mémoire. La répartition automatique, avec une réserve de 512 Mo par carte, est plus sûre.

## Mise à jour

```
ajean update
```

Testé sur un serveur Linux à deux cartes avec un preset ExLlama (conversation de plus de 50 000 tokens). La proposition d'appliquer un preset modifié n'a été vérifiée que par les tests automatiques, pas à la main dans l'interface. Le correctif du mode Jean est couvert par un test qui rejoue la situation décrite dans #175 ; la cause exacte de la lecture ratée au démarrage n'a pas été reproduite.
