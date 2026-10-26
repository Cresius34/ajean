AJEAN peut désormais faire tourner les modèles EXL3 avec ExLlamaV3, à côté de llama.cpp, avec la même facilité : installation en un clic, mise à jour, et réglages dans l'éditeur de modèle.

## Nouveauté : moteur ExLlama

- **Installation sans prérequis** : une nouvelle ligne « ExLlama » dans la section Moteur installe TabbyAPI et ExLlamaV3 avec leurs dépendances précompilées (PyTorch, CUDA 12 ou 13 selon le pilote). Python est téléchargé et géré par AJEAN : rien n'est à installer sur la machine. Environ 4 Go à télécharger, 10 Go sur le disque. L'installation par défaut prend une version validée, avec des dépendances figées.
- **Mise à jour et désinstallation** depuis la même fenêtre, qui indique les versions installées (ExLlamaV3, TabbyAPI, PyTorch) et si une version plus récente existe. La mise à jour installe la dernière version de TabbyAPI et la version d'ExLlamaV3 qu'elle demande ; la version en place reste utilisable jusqu'à la fin de l'installation, et un modèle ExLlama actif est relancé. La désinstallation conserve les modèles.
- **Téléchargement de modèles EXL3** : il suffit de coller le lien d'une branche d'un dépôt Hugging Face (par exemple `https://huggingface.co/<dépôt>/tree/4.0bpw`). Les fichiers arrivent dans `models/exl3`, le suivi et l'arrêt se font comme pour llama.cpp.
- **Choix du moteur par modèle** : l'éditeur de modèle propose « ExLlama » à côté des moteurs llama.cpp. Les réglages propres à llama.cpp laissent alors place à ceux d'ExLlama : dossier du modèle, contexte, taille et précision du cache, utilisateurs simultanés, lecture du prompt par blocs, décodage spéculatif (MTP, n-gram ou modèle brouillon) avec MTP adaptatif en option, accélération des recopies, raisonnement et budget de réflexion, vision et carte de l'encodeur d'images, cartes utilisées, répartition en Go par carte, parallélisme de tenseurs.
- Le moteur sert sur le même port et avec la même clé que llama-server : chat, vision, bench, accès distant et endpoint OpenAI fonctionnent sans autre réglage.
- **Ligne de commande** : `ajean exl3 install`, `update`, `download <lien>`, `status`, `uninstall`.

Mesuré sur un serveur Linux à deux cartes NVIDIA, avec Qwen 3.8 27B en EXL3 4 bits : 54 jetons/s en génération, 88 en recopie de code, 740 en lecture du prompt, soit la même vitesse qu'un TabbyAPI installé et réglé à la main.

## Mise à jour

```
ajean update
```

ExLlama est proposé sous Linux et Windows avec une carte NVIDIA (pilote 570 ou plus récent). Testé sous Linux uniquement : l'installation sous Windows n'a pas été essayée.
