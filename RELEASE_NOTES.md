Le premier message d'une nouvelle conversation obtient sa réponse beaucoup plus vite, et un arrêt pendant une série d'outils ne bloque plus la conversation.

## Nouveauté

- **Pré-chauffage du prompt** : dès qu'une nouvelle conversation est ouverte, le moteur lit à l'avance les consignes et la description des outils. Le premier message ne paie plus cette lecture. Mesuré avec le moteur MoE : le premier mot arrive en 0,3 s au lieu de 4,3 s en mode Projet, et en 0,3 s au lieu de 5,9 s en mode Jean. Avec llama-server, le gain porte surtout sur le premier message après le démarrage du moteur. Le pré-chauffage ne part que si la conversation ouverte est vide : une conversation en cours garde son cache. Il est actif par défaut et se coupe dans Paramètres > Général (« pré-chauffage du prompt »). Contribution de youtsuhodev (#163).

## Corrections

- **Conversation bloquée après un arrêt** : un arrêt pendant une série d'appels d'outils laissait dans l'historique des appels sans réponse. Le moteur refusait ensuite chaque nouveau message (erreur 400), et la conversation restait inutilisable. Les réponses manquantes sont maintenant complétées, y compris dans les conversations déjà bloquées par une version précédente. Une image affichée par l'IA au milieu d'une série d'outils est aussi replacée après la série. Contribution de oh-codes (#171).
- **Mode Jean, saisie au clavier** : après une réponse, taper une lettre ne rouvrait plus la bulle de saisie. Les caractères partaient dans le champ caché. Contribution de oh-codes (#164).

## Mise à jour

```
ajean update
```

Testé sur un serveur Linux avec le moteur MoE et avec llama-server (Qwen 3.8, MTP). Le pré-chauffage n'a pas été testé avec une API externe : il ne s'y applique pas.
