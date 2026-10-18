# Proposition GitHub : mémoire hybride optionnelle et preuve portable

Base : Ajean 0.18.1 (`e7b024adda191dd6f64746a454b8714629a83435`). Révision 5 : code prêt à examiner, paquet Windows compilé, contrôle de l’instance au lancement et révision visible dans l’interface ; la révision 3 a été déclarée fonctionnelle par l’utilisateur sur son poste Windows. Après un signalement de réglages absents avec la livraison R4, la R5 demande une nouvelle validation utilisateur. Les tests d’interface et les essais CPU Linux ne remplacent pas une validation Windows/GPU.

## Problème et résultat

Une recherche fondée sur les mots peut manquer une note pertinente lorsque l’utilisateur reformule. La proposition conserve la recherche native et lui ajoute, au choix, une recherche sémantique avec EmbeddingGemma 2. La fusion RRF continue à produire des notes Markdown, extraits et lignes à lire avec `mem_read`.

Le choix **Classique/Hybride** vit dans Paramètres → Général, avec une aide. Les presets peuvent hériter de ce choix ou le remplacer et choisir leur moteur mémoire, modèle, CPU/GPU, modalités, dimensions et threads, directement sous les réglages vision du modèle. Les valeurs générales vivent en état ; les seules surcharges explicites vivent dans le preset. Les modes d’accès aux notes et leur chiffrement sont indépendants.

## Implémentation

- Processus local séparé, clé éphémère, boucle locale et supervision limitée à ses propres enfants. Aucun processus de conversation externe n’est repris.
- Chemin GGUF texte compact avec llama-server, CPU par défaut ou binaire GPU compatible sélectionné. Chemin multimodal optionnel avec Python embarqué, SentenceTransformers et PyTorch ; les encodeurs vision/audio non demandés sont omis à la construction.
- Vecteurs 768d, ou troncature MRL à 512/256/128 suivie de normalisation L2. La dimension fait partie de l’identité d’index.
- Préfixes de recherche/document ; blocs de 1 800 caractères ; hashes des sources, y compris médias locaux référencés ; captures du projet avant tout travail asynchrone.
- Cache borné et dérivé en RAM, purge au verrouillage, publication conditionnée à l’époque et à la fraîcheur des sources. Les changements de réglages arrêtent le worker concerné.
- Médias uniquement liés depuis les notes canoniques, sans réseau ni sortie du projet ; résultats toujours attachés aux notes. Pas de médias en clair indexés dans un coffre chiffré. Vidéo bornée et décodée par PyAV, sans FFmpeg système.
- Repli explicite sur la recherche native en cas d’encodeur indisponible, de changement de sources ou de délai dépassé.

La preuve portable possède son point d’entrée `cmd/ajean-memory-test`, son identité Windows et son dossier `data`. Elle restaure les interfaces natives de presets et llama.cpp ; les sources référencées sont copiées et les chemins rendus relatifs. L’installation système, la mise à jour de l’application, le tunnel et le MoE restent hors de cette preuve. Le binaire principal officiel conserve son point d’entrée et ses ressources.

## Correction indépendante Crawl4AI

Dans `runCrwl` seulement, les actions JavaScript sont envoyées à `/execute_js` avec `url`/`scripts`. Le crawl simple garde `/crawl`, sans `js_code`. Aucun changement de signature, de type, d’appelant ou de parsing. Une contribution séparée est possible ; tests HTTP couvrant corps, route, authentification et les deux formes de réponse.

## Validation et limites

Voir le guide `embedding-memory-test.md`, les scripts de test et le `VALIDATION.txt` du paquet. Le vrai GGUF et le vrai modèle complet sont utilisés pour les essais opt-in ; les encodeurs sélectifs sont vérifiés par nombre de paramètres, sortie finie et norme. Le chemin Go → worker HTTP → notes/média → recherche 256d et l’invalidation d’un média supprimé sont testés.

Les essais CPU ne sont pas une preuve d’accélération sur AMD/NVIDIA. L’installeur multimodal fourni prépare CPU ; un runtime GPU doit être compatible avec la cible. Pas d’index persistant, d’évaluation de rappel sur toute la mémoire Evolve ni d’analyse exhaustive de longs médias. Le lancement natif de cette nouvelle révision Windows et ses scripts PowerShell restent à confirmer sur le poste cible.

Titre proposé : **Add optional hybrid memory with global/preset settings and selective EmbeddingGemma 2 encoders**.

Les sources, patchs et fichiers modifiés sont fournis pour examen. Aucun push, PR ou publication n’est effectué automatiquement. La PR MoE Windows nº 144 reste une référence pour une phase ultérieure ; son code n’est pas appliqué ici.
