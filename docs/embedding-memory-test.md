# AJEAN Mémoire TEST — version complète, révision 5

Base : Ajean 0.18.1, commit `e7b024adda191dd6f64746a454b8714629a83435`. Version affichée : **0.18.1-embedding-test.5**. Le prototype reste portable et indépendant de l’installation habituelle. La phase MoE attend la livraison et la validation de cette révision.

## Correctif de livraison R5

Le 16 octobre 2026, les nouveaux réglages ont été signalés absents sur le poste utilisateur après livraison R4. L’exécutable de la R4 conservée contient ces contrôles ; leur présence dans une archive ne suffit pas à identifier l’instance ouverte sur le poste. L’ancien lanceur pouvait réutiliser une instance du même dossier sans vérifier sa révision. Cette possibilité est établie par le code ; la cause exacte sur le poste reste à confirmer.

La R5 affiche **TEST mémoire · R5** sous le logo dès le HTML servi, même avant les appels JavaScript. Le lanceur vérifie la version Windows du fichier, la révision et le chemin de l’instance active. Une ancienne instance doit être quittée depuis son icône près de l’horloge ; elle n’est plus rouverte silencieusement. La page d’ouverture est l’interface principale avec les presets et les paramètres. La page avancée reste accessible par **Mémoire sémantique · EmbeddingGemma 2**.

Après remplacement, lancer **Lancer-Test.cmd** et vérifier le repère **TEST mémoire · R5** ainsi que `0.18.1-embedding-test.5`. Le repère doit apparaître sur `http://127.0.0.1:18090/`. Les versions annoncées par le fichier et le serveur ne suffisent pas à valider une inférence GPU.

## Mise à jour et premier lancement

Quitter le TEST depuis son icône près de l’horloge, extraire le ZIP et remplacer les fichiers du programme dans le même dossier. **Conserver `data/` et `encoder/`** : notes, conversations, réglages, modèles et moteurs existants. Les interfaces et le serveur multimodal sont intégrés à l’exécutable ; les nouveaux installeurs et le fichier de dépendances sont aussi à conserver.

Premier dossier : extraire tout le ZIP puis lancer `Lancer-Test.cmd`. Le moteur CPU est fourni ; le premier lancement télécharge le GGUF texte Q8_0 (310 Mo) avec contrôle SHA-256. `Copier-Presets.cmd` copie depuis `C:\ProgramData\ajean\presets` les presets, moteurs/DLL, poids, projecteur et draft. Une autre source peut être indiquée par `Copier-Presets.cmd -Source "D:\Ajean\presets"`. Les fichiers source sont lus uniquement ; les presets déjà présents sont conservés.

Les chemins externes sélectionnés dans l’éditeur d’un preset sont copiés au moment de l’enregistrement ou de l’activation. Une copie identique est réutilisée ; une collision différente est signalée sans remplacement. La première copie de gros poids peut durer plusieurs minutes : le formulaire affiche l’enregistrement en cours. Les lectures de statut et le lancement ne copient pas automatiquement les dépendances.

Arrêter le moteur Swift habituel avant de charger sa copie locale afin de libérer la VRAM. Le TEST ne ferme pas cette autre application. Pour réutiliser explicitement son API, la connexion locale de la page mémoire reste disponible et crée un preset éditable.

## Général : Classique ou Hybride

Dans **Paramètres → Général → Système mémoire**, choisir :

| Choix | Recherche | Ressources |
|---|---|---|
| Classique | Recherche native par mots dans les notes | Aucun encodeur supplémentaire |
| Hybride | Recherche native + similarité sémantique, fusion RRF | Un encodeur mémoire CPU ou GPU ; index en RAM |

Le **?** explique ces différences et la priorité des presets. Ce choix général est une valeur par défaut ; un preset réglé explicitement sur Classique ou Hybride la remplace. L’état affiché précise si un preset est prioritaire et si l’index est prêt. En cas d’indisponibilité ou d’index modifié, la recherche utilise le mode Classique et indique la raison.

Ce réglage ne remplace pas le mode d’accès aux notes (toujours, recherche, à la demande, désactivé), ni leur chiffrement. Les Markdown restent la source de vérité.

## Preset : sous Vision et Projecteur vision sur le CPU

Le bloc **Mémoire** est placé avant le téléchargement du modèle. Il permet de choisir le système mémoire, le moteur, son backend et son modèle, CPU/GPU, le périphérique, les modalités actives, les dimensions et les threads. Les valeurs explicites s’appliquent à ce preset ; **Reprendre tous les réglages généraux** retire ces surcharges. L’enregistrement n’active pas un preset modifié : le sélectionner ensuite pour l’appliquer.

- **GGUF · texte compact** : utilise le modèle texte quantifié et llama-server. Le moteur CPU livré n’utilise pas de VRAM. Pour le GPU, choisir un moteur ROCm/Vulkan/CUDA compatible dans les backends copiés ; le sélecteur reprend le moteur local du preset quand il est renseigné. Exemples de périphériques : `ROCm0`, `Vulkan0`, `CUDA0`, selon `--list-devices` de ce binaire.
- **Complet · multimodal** : utilise le checkpoint officiel et un Python portable avec SentenceTransformers/PyTorch. Il charge les encodeurs sélectionnés et sert la même recherche native. Le texte est toujours nécessaire aux requêtes. La vidéo utilise l’encodeur vision.
- **GPU du moteur complet** : PyTorch nomme les périphériques `cuda:0`, `cuda:1`, aussi avec ROCm/HIP. Un runtime PyTorch compatible avec le GPU et le pilote est nécessaire. L’installeur fourni prépare la variante **CPU**, sans installer de pilote ni toolkit. Ne pas copier seulement un `python.exe` ou supposer qu’un venv externe est déplaçable.
- **768 / 512 / 256 / 128 dimensions** : réduit la taille du cache de vecteurs et le travail de comparaison, sans réduire les poids chargés. La troncature est suivie d’une renormalisation L2 ; documents et requêtes utilisent la même dimension. 128 vise surtout le texte.

La vision du modèle de conversation et le placement de son projecteur restent indépendants de l’encodeur mémoire.

## Installer les modalités supplémentaires

Lancer **Installer-Multimodal.cmd** une seule fois si les images, l’audio ou la vidéo sont utiles. Il prépare dans `data/embedding-python` un Python Windows x64 embarqué, ses dépendances CPU et le modèle officiel dans `data/models/embeddinggemma-2-full`. Prévoir plusieurs Go libres. Aucun PATH, service ou Python système n’est modifié. Un dossier existant non géré par l’installeur est conservé.

Le modèle complet est téléchargé à une révision figée. Son serveur fonctionne ensuite hors ligne. Les composants non sélectionnés ne sont pas instanciés ; cela a été vérifié sur CPU avec le vrai checkpoint :

| Modalités | Paramètres effectivement chargés lors du test CPU |
|---|---:|
| Texte | 271 002 624 |
| Texte + images | 438 760 448 |
| Texte + audio | 576 613 664 |
| Texte + images + audio + vidéo | 744 371 488 |

Ces nombres sont des comptes de paramètres observés, pas des mesures de RAM/VRAM ni des benchmarks sur la Radeon. La précision est float32 sur CPU ; le moteur complet choisit bfloat16 sur GPU compatible, sinon float32. Il n’utilise pas float16.

## Médias et recherche

Placer des **copies** de notes avec `Copier-Memoire.cmd` ou utiliser les trois notes fictives de démonstration. La page **Mémoire sémantique** permet de comparer mots-clés et résultats hybrides, de configurer les valeurs générales, d’indexer et d’arrêter l’encodeur.

Les modalités supplémentaires portent sur les fichiers locaux référencés dans les notes canoniques, par exemple :

```md
# Observations
![Capture du viewport](pieces/capture.png)
[Extrait sonore](pieces/prise.wav)
[Essai en mouvement](pieces/essai.mp4)
```

Les résultats renvoient toujours au fichier Markdown et à ses lignes pour `mem_read`. Pas de téléchargement de média distant, d’inventaire récursif ou de lecture hors du dossier mémoire du projet. Les liens sortants et traversées de chemins sont refusés. Les médias restent désactivés dans un coffre chiffré ; les notes texte peuvent continuer à être indexées en RAM, puis purgées au verrouillage.

Limites explicites du prototype : PNG/JPEG/WebP/BMP, WAV/FLAC/MP3/OGG, MP4/WebM/MOV ; 64 Mio par média, au plus quatre images, un audio et une vidéo référencés par note. Les vidéos sont décodées par PyAV : une image par seconde, au plus les 16 premières images retenues. Cela borne le contexte partagé de 8 192 tokens. Le son doit rester un extrait court compatible avec ce contexte. Ce traitement ne constitue pas une analyse exhaustive d’une vidéo longue.

Les notes et médias sont re-hachés à la recherche ; un changement de contenu, même de même taille/date, invalide l’index. Les blocs font au plus 1 800 caractères. Cache dérivé en RAM uniquement, au plus quatre projets et 12 000 blocs par projet ; aucune copie de vecteurs/snippets n’est enregistrée dans la DB. MEMORY.md, archives, backups, fichiers cachés et liens symboliques de notes sont exclus.

## Portabilité

| Élément | Emplacement |
|---|---|
| Programme | `ajean-memoire-test.exe` |
| Données, presets, notes, modèles et moteurs | `data/`, à côté du programme |
| Encodeur GGUF CPU fourni | `encoder/` |
| Interface | `127.0.0.1:18090` |
| Moteur de conversation du TEST | `127.0.0.1:18080` |
| Encodeur mémoire | `127.0.0.1:18081` par défaut |
| Python multimodal et modèle complet optionnels | `data/embedding-python`, `data/models/embeddinggemma-2-full` |

Les chemins sont enregistrés relativement au dossier portable et résolus au lancement. Les processus contrôlés sont seulement les enfants du TEST ; aucun PID/service externe n’est adopté. Les fonctions natives de presets et llama.cpp restent disponibles. L’installation système, la mise à jour de l’application, le tunnel, le pare-feu et le moteur MoE système restent désactivés dans cette démonstration. Les outils de shell activés par l’utilisateur gardent leurs droits Windows habituels.

Journaux : `data/ajean-embedding-test.log` et `data/memory-encoder.log`. Pilote graphique et dépendances système normales restent nécessaires sur un autre PC. La portabilité complète a été éprouvée pour le chemin GGUF sous Linux CPU ; le Python Windows embarqué et les GPU demandent une validation sur la cible.

## Correctif Crawl4AI

`runCrwl` utilise `/execute_js` avec `url` et `scripts` lorsqu’une action JavaScript est demandée. Le crawl simple conserve `/crawl` avec ses configurations, sans `js_code`. Signatures, appelants et parsing des réponses restent identiques. Les deux formes et l’authentification sont vérifiées par serveur HTTP de test. Le déploiement réel Crawl4AI n’a pas été exécuté ici ; si son serveur refuse `/execute_js` avec 403, vérifier son activation explicite `CRAWL4AI_EXECUTE_JS_ENABLED=true` et sa clé.

## Composants et sources

- GGUF : https://huggingface.co/ggml-org/embeddinggemma-2-GGUF — révision `bfcd298762cc34d0357ece5ebdd31791a3a374d8`, Q8_0 de 309 855 456 octets ; SHA-256 `2188ac1deca4b77dffefd603c2776a9d76d9d74ec01841392982ebb840b09135`.
- Modèle complet : https://huggingface.co/google/embeddinggemma-2 — révision `914f7f89142e33e77833254d9c9b90c3cef7303b`. Modèle et GGUF déclarés Apache 2.0 par leurs fiches.
- Encodeur CPU : https://github.com/ggml-org/llama.cpp/releases/tag/b11669 — asset Windows x64, SHA-256 `37b3efcf962847ca869745fd4ccf7f5e7e8dacfeff9fa3523b4ca8cdc66d632b`.
- Dépendances optionnelles : Python embarqué 3.12.10 et bootstrap pip contrôlés par SHA-256 ; PyTorch CPU 2.14.1, torchvision CPU 0.29.1 et versions du fichier `embedding-requirements.txt`.

Les licences d’Ajean/llama.cpp et du modèle sont jointes. Le paquet contient les sources complètes, le patch binaire et les fichiers modifiés/ajoutés pour GitHub. Aucune donnée personnelle, clé, DB ou poids GGUF dans ces sources. Aucun dépôt distant n’est modifié par les scripts.

Construction Windows avec Go : `powershell -NoProfile -File tools/embedding-test/Construire-Portable.ps1`. Point d’entrée : `cmd/ajean-memory-test` ; `cmd/ajean` conserve son identité et ses services habituels. L’interface générée doit être régénérée par `go generate ./internal/ajean` après édition de ses sources.

Tests : `CI=1 go test ./...` ; cas ciblés avec `-race` ; `test_embedding_server.py`, `smoke_portable.py` et `smoke_multimodal.py`. `smoke_ui.py --app CHEMIN_APP --report UI.json` vérifie l’interface réellement servie et les enregistrements dans une fixture jetable ; il nécessite Playwright et Chromium pour le développement. Les intégrations réelles sont opt-in : `AJEAN_TEST_EMBED_BIN` / `AJEAN_TEST_EMBED_MODEL` pour GGUF ; `AJEAN_TEST_EMBED_PYTHON` / `AJEAN_TEST_EMBED_FULL_MODEL` pour le moteur complet. Voir **VALIDATION.txt** pour l’environnement et les résultats exacts. Une compilation Windows et des tests CPU ne démontrent pas le fonctionnement GPU sur le poste cible.
