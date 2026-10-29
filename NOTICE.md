# Notices

## Ascendance

Ce dépôt est un **fork** de
[**nathaninline/ajean**](https://github.com/nathaninline/ajean), qui est le
projet amont — la version originale, et la seule source des versions publiées.

Les paquets de ce dépôt portent une **ligne de correctifs privée**, écrite pour
les besoins d'une machine précise (moteur MoE **Strata** sur carte AMD : fenêtre
de contexte, quantification clé/valeurs, réserve de VRAM, vision sur AMD ;
mémoire sémantique **EmbeddingGemma 2** ; **pont Ajean↔Ajean** ; déduplication
des appels ; réglage du seuil de compactage). Elle est signalée par un suffixe à part,
**`.N Cresius`** : la version lue par le binaire est par exemple
**`0.18.16.2 Cresius`**, pas `0.18.16` (base amont `v0.18.16` + révision `2`).
Cette ligne n'est **pas** une version de
l'amont, n'est **pas** testée sur d'autres machines et n'est **pas** garantie
transposable. Pour installer AJEAN, prenez les versions publiées sur
[nathaninline/ajean](https://github.com/nathaninline/ajean/releases/latest).

## Ce que ce fork ajoute à l'amont

Sept fronts, mesurés commit par commit contre le tag amont `v0.18.16`
(**148 fichiers, +29 296 / −10 349**, **aucun fichier amont supprimé** — chaque
fusion est additive). Le détail chiffré et la surface de conflit sont dans
[RELEASE_NOTES.md](RELEASE_NOTES.md).

| Front | Ce qui est ajouté | Accrochage dans le code amont |
| --- | --- | --- |
| **Moteur MoE Strata** | `backend_moe_strata.go`, `backend_moe_windows/_linux/_other.go`, `moe_probe.go` (route `/api/moe/verify`), `moe_windows_manifest.json`, `moe_portable_install.ps1`, `web_devices.go` (`/api/backends/devices`) | +171 / −163 seulement dans **leur** `backend_moe.go` |
| **Mémoire sémantique EmbeddingGemma 2** | `mem_embeddings.go`, `mem_embedding_backend/_media/_settings/_transformers.go`, `embedding_server.py`, `web_embeddings.go` (`/api/embeddings*`) | leur machinerie `jean_recall.go`, `jean_exact.go`, `jean_fixes.go` **conservée**, rebranchée sur notre encodeur |
| **Pont Ajean↔Ajean** | `parallel_bridge.go`, `parallel_peer.go`, `parallel_bridge_agent.go`, `parallel_config/_engine/_import/_profile/_memory.go`, `13b-bridge.js`, 4 fichiers de tests | **entièrement absent de l'amont** |
| **Déduplication des appels d'outils** | `repeat_exempt.go`, tests `llm_dedup_*` | leurs `chat_tools_loose.go` / `tool_args_repair.go` coexistent |
| **Outillage d'installation Windows** | `tools/patch-canal/` — 26 outils, dont le poseur | aucun équivalent amont |
| **Réglage du seuil de compactage** | `COMPACT_FRAC` (pourcentage) + marge de sécurité | `chat_compact.go` et `sys_service.go` étaient **à zéro écart** avant ce correctif |
| **Ascendance assumée** | ce fichier, entête `LICENSE` (+4 lignes), README FR/EN réécrits, `RELEASE_NOTES.md` | — |

**La génération d'images n'est pas une fonction de ce fork.** Le dépôt
supprime explicitement le générateur d'images de la configuration du moteur
(`delete(out, "images")` dans `backend_moe_strata.go` : aucun générateur n'est
installé ni démarré par AJEAN). Un **prototype mesuré, hors dépôt**, existe et
est décrit dans [RELEASE_NOTES.md](RELEASE_NOTES.md) — il n'est ni installé, ni
paqueté, ni garanti.

Le **moteur MoE** utilisé ici provient de
[**Niko1221/Strata**](https://github.com/Niko1221/Strata), dont les packs
Windows sont téléchargés à l'installation du moteur.

| Projet | Rôle |
| --- | --- |
| [nathaninline/ajean](https://github.com/nathaninline/ajean) | amont — projet original et versions publiées |
| [Niko1221/Strata](https://github.com/Niko1221/Strata) | moteur MoE exécuté par le fork |
| ce dépôt | ligne de correctifs privée, millésime `.N Cresius` (`0.18.16.2 Cresius` posé, base amont `v0.18.16`) |

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
