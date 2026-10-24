# Notices

## Ascendance

Ce dépôt est un **fork** de
[**nathaninline/ajean**](https://github.com/nathaninline/ajean), qui est le
projet amont — la version originale, et la seule source des versions publiées.

Les paquets de ce dépôt portent une **ligne de correctifs privée**, écrite pour
les besoins d'une machine précise (moteur MoE **Strata** sur carte AMD : fenêtre
de contexte, quantification clé/valeurs, images sur AMD ; recherche par embeddings ; déduplication
des appels). Elle est signalée par un suffixe à part,
**`.1 Cresius`** : la version lue par le binaire est par exemple
**`0.18.11.1 Cresius`**, pas `0.18.11`. Cette ligne n'est **pas** une version de
l'amont, n'est **pas** testée sur d'autres machines et n'est **pas** garantie
transposable. Pour installer AJEAN, prenez les versions publiées sur
[nathaninline/ajean](https://github.com/nathaninline/ajean/releases/latest).

Le **moteur MoE** utilisé ici provient de
[**Niko1221/Strata**](https://github.com/Niko1221/Strata), dont les packs
Windows sont téléchargés à l'installation du moteur.

| Projet | Rôle |
| --- | --- |
| [nathaninline/ajean](https://github.com/nathaninline/ajean) | amont — projet original et versions publiées |
| [Niko1221/Strata](https://github.com/Niko1221/Strata) | moteur MoE exécuté par le fork |
| ce dépôt | ligne de correctifs privée `0.18.9.1 Cresius` |

## Modèles

Les modèles téléchargés par le programme sont distribués sous leurs propres
licences (voir la page du modèle concerné sur Hugging Face) ; leur utilisation
est régie par les conditions du modèle, pas par la licence d'AJEAN.

## Dépendances

AJEAN s'appuie sur les composants suivants, chacun sous sa licence :

- **llama.cpp** (MIT) — moteur d'inférence, utilisé via les binaires officiels
  publiés sur les releases `ggml-org/llama.cpp`.
- **Strata** (moteur MoE, voir la licence du dépôt `Niko1221/Strata`) — pack
  Windows téléchargé à l'installation du moteur MoE.
- **SQLite** (domaine public) — base de données locale.
