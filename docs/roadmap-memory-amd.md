# Feuille de route de la preuve de concept

## Phase 1 — Mémoire alternative EmbeddingGemma 2

Livrer un prototype portable d'Ajean 0.18.1 avec les fonctions natives de presets et llama.cpp, un encodeur CPU indépendant et la recherche hybride dans `mem_search`.

Critères avant validation utilisateur :

- Importer les fichiers dans le dossier portable et conserver les sources.
- Démarrer, arrêter et redémarrer le moteur du prototype ; vérifier qu'une instance étrangère sur 8080 continue de tourner.
- Comparer les réponses lexicales et hybrides sur plusieurs notes réelles, des reformulations et des noms d'assets exacts.
- Vérifier ajout, modification, suppression, séparation par projet et repli en cas d'arrêt de l'encodeur.
- Déplacer le dossier portable puis relancer moteur, presets et mémoire sans dépendre de son ancien emplacement.
- Valider le lancement Windows natif, le moteur AMD de conversation et l'utilisation sur le poste cible. Les tests de compilation et les essais CPU Linux ne remplacent pas cette étape.

La phase 1 reste ouverte jusqu'à cette validation avec l'utilisateur. Le code, les tests et une proposition de contribution GitHub sont préparés ; aucune publication n'est effectuée par ces scripts.

Retour utilisateur du 16 octobre 2026 : Julien confirme « fonctionnel, je valide » pour la révision 3 sous Windows. Le 16 octobre, il demande une version mémoire finale avec les réglages intégrés à Général et aux presets, les modalités sélectives et le correctif Crawl4AI. Cette révision 4 doit être livrée avant de poursuivre MoE ; ne pas étendre la validation rapportée de la révision 3 aux nouvelles fonctions sans test.

Retour ultérieur du 16 octobre 2026 : les réglages demandés ne sont pas visibles sur le poste après la livraison R4. La R4 archivée les contient, mais l'instance effectivement ouverte n'a pas été identifiée. Défaut constaté : son lanceur ne vérifiait pas la version d'une instance déjà démarrée. La R5 ajoute ce contrôle et un repère permanent dans l'interface.

Validation fermée le 18 octobre 2026 sur le poste cible, à la révision 6, installée sous `Ajean v18.4 maj Cresius` : encodeur et recherche hybrides mesurés en service, réglages visibles dans Général et dans un preset.

## Phase 2 — MoE AMD/NVIDIA sous Windows

Objectif demandé : un moteur MoE fonctionnel sous Windows pour AMD et NVIDIA permettant de faire tourner une version quantifiée de Qwen 3.8 Flash Next, en prenant comme référence l'intégration Linux d'Ajean 0.18.1.

Déclencheur : validation de la phase 1. Cette phase n'est pas lancée automatiquement par une note de mémoire ou cette feuille de route.

Référence fournie par Julien le 16 octobre 2026 : https://github.com/nathaninline/ajean/pull/144. PR ouverte et non fusionnée lors de la lecture de son descriptif à cette date : moteur Windows compilé avec MSVC/CUDA, réutilisation de la détection existante et essais rapportés sur NVIDIA. C'est une base pour Windows/NVIDIA ; le support AMD/ROCm/Vulkan n'est pas établi par ces essais. Lire le diff et l'état courant de cette PR lors de la reprise, après la livraison mémoire. Aucune partie de cette PR n'a été appliquée au prototype mémoire.

Au démarrage de cette phase : vérifier les versions et modèles effectivement disponibles, le format quantifié pris en charge, les backends compatibles AMD/NVIDIA sous Windows, leurs contraintes de licence et de mémoire. Tester sur la Radeon AI PRO R9700 (32 Go) et prévoir la cohabitation avec Unreal et l'encodeur CPU. Le choix d'un moteur et d'une quantification reste à établir par des essais.

Preuve attendue : chargement réel du modèle, génération cohérente, appels d'outils, mesure de préremplissage/décodage, occupation VRAM/RAM, démarrage/arrêt fiables et intégration dans les presets du prototype portable. Une compatibilité annoncée ou une compilation seule ne constitue pas une validation.

## État au 18 octobre 2026

Les deux phases sont closes sur le poste cible, sur une base **0.18.4**. Les preuves sont dans
`docs/embedding-memory-test.md` (mémoire) et `docs/moe-windows-prototype.md` (moteur MoE).

- **Moteur retenu : Strata**, moteur MoE Windows à chargement différé des experts, piloté par
  `internal/ajean/backend_moe_strata.go` et son manifeste embarqué
  `internal/ajean/moe_windows_manifest.json`. La PR #144 n'a pas été fusionnée : elle porte un
  chemin concurrent (llama.cpp compilé en CUDA) dont les structures ne correspondent pas à
  l'installation appelée par `moeRunInstall`.
- **Mesuré en service** : Radeon AI PRO R9700, backend **HIP** (`gfx1201`), poids **Flash Next
  Swift 1.5 IQ3_XXS**, contexte 126 976, profil expert de 24 576 slots chargé, cache d'experts à
  94–97 % de hits, rapport `POST /api/moe/verify` concluant (réponse et appel d'outil), port
  moteur 18080 et interface 18090.
- **Ce que la preuve n'établit pas** : la cohabitation avec Unreal n'a pas été chiffrée, et le
  backend **CUDA** n'a pas été validé sur cette base. Les essais CPU et une compilation seule ne
  valent pas validation NVIDIA.
