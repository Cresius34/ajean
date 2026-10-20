# NOTICE — ascendance et provenance

Ce dépôt (**Cresius34/ajean**) est un **fork personnel** de
[**nathaninline/ajean**](https://github.com/nathaninline/ajean), créé par **nathaninline**.
Le projet principal, sa conception et l'essentiel du code sont à son auteur. Ce fork n'est
**pas** une distribution d'AJEAN : c'est une **branche de travail privée**, tenue pour les
besoins d'une machine précise. Les versions publiées d'AJEAN sortent uniquement de l'amont.

## Ce que cette branche ajoute (0.18.4.1 Cresius)

Modifications propres à ce fork, écrites pour un poste AMD Radeon avec une limite de
commit mémoire contrainte. Elles ne constituent pas une version amont et ne sont pas
testées ailleurs :

- **Moteur MoE Strata** sous Windows/AMD (backend HIP, repli Vulkan), experts chargés à la
  demande, réserve VRAM configurable. Intégration avec
  [Niko1221/Strata](https://github.com/Niko1221/Strata), dont le moteur et les poids de
  modèle restent distribués sous leurs propres termes.
- **Recherche par embeddings** (EmbeddingGemma 2, EmbeddingV2) branchée sur la mémoire.
- **Correctifs Crawl4AI 0.9.5+** : routage vers `/execute_js` quand du JS est demandé (la
  clé `js_code` est rejetée par `/crawl`), robustesse des bords d'appel, délai d'attente.
- **Recherche web** : requêtes explicites, garde-fous anti-boucle.
- **Dédup d'appels d'outil** : une compaction ne peut plus rendre un contenu effacé. Le
  résultat mémorisé porte l'« ère » du contexte ; si l'historique a été réécrit depuis,
  l'appel est rejoué et le contenu revient, au lieu d'un « déjà fait » sans marchandise.
- **Marquage de version** : suffixe « .1 Cresius » dans la version affichée, le bandeau de
  l'interface et la description du fichier exécutable, pour ne jamais confondre avec une
  version amont.

## Ce qui est proposé à l'amont

Un correctif Crawl4AI est tenu à part sur la branche `fix-crawl4ai-execute-js` (un commit,
un fichier, +8/−5) en vue d'une pull request vers `nathaninline/ajean`. Il n'a pas encore
été intégré. Rien d'autre dans cette liste n'est destiné à l'amont sans accord préalable.

## Licences

- AJEAN (amont) : MIT, voir `LICENSE`. L'en-tête de la licence conserve la mention de
  l'amont ; les termes MIT ne sont pas modifiés par ce fork.
- Strata, llama.cpp, Crawl4AI et les poids de modèle téléchargés : leurs propres licences
  et conditions, inchangées ici.
