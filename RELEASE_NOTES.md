# Notes de version — AJEAN 0.18.4.1+rev9

**Français** · [English](#english)

Révision locale **rev9** appliquée sur la base officielle **v0.18.4**
(`76f60c0`). Elle n'est pas publiée sur `nathaninline/ajean` : elle vit sur
le dépôt `Cresius34/ajean`, étiquetée **`v0.18.4.1+rev9`**.

Cinq chantiers :

| Chantier | Ce que ça change pour vous |
| --- | --- |
| Moteur **MoE** sur Windows | Installer un modèle MoE (AMD **HIP** ou NVIDIA **CUDA**) depuis l'onglet Moteur, sans script. |
| **Mémoire sémantique** | Recherche des notes en mode **Hybride** : les mots **et** le sens, avec un moteur d'embeddings local. |
| **Lecture des pages JavaScript** | Les pages rendues par JavaScript se lisent via un serveur **Crawl4AI** que vous hébergez. |
| **Base de données portable** | Une installation recopiée garde les chemins de l'installation d'origine ; un utilitaire les relocalise. |
| **Périmètre du dépôt** | `data/` et `models/` ne sont plus jamais commités. |

## Moteur MoE sur Windows

L'onglet **Moteur** installe un modèle **MoE** — famille **Swift 1.5**
(quantifications **IQ2_XS**, **IQ3_XXS**) ou **Classique**, le Qwen3.8-Flash-Next
original (**Q2_0**, **IQ2_XS**, **IQ3_XXS**, **IQ3_S**). Deux packs, selon votre carte :

- **AMD HIP** — ROCm **6.4** sur Windows, cartes **RX 7000**, **RX 9000**.
  Pilote **AMD Software: Adrenalin Edition 25.93** ou plus récent.
  Réserve de mémoire vidéo **4096 Mo** par défaut, réglable de **0** à
  **65536 Mo**. Le pack HIP est **texte uniquement** : son encodeur vision
  n'est pas fourni, donc activez la vision sur un pack CUDA.
- **NVIDIA CUDA** — CUDA **12.2+** ou **13.2+** selon le pilote. Vision
  disponible.

Le contexte va de **32 768** à **262 144** tokens par paliers de **4 096**
(**86 016** par défaut).

Ce que l'installation fait, dans l'ordre :

1. Elle vérifie l'espace libre, le pilote, le backend, le toolkit, le
   contexte et la réserve avant de rien télécharger.
2. Elle télécharge le paquet, vérifie sa taille et ses empreintes.
3. Elle installe Python et les bibliothèques, vérifie les fichiers livrés,
   vérifie que Python importe, lit les capacités réelles du GPU.
4. Elle télécharge le modèle, vérifie sa taille et sa clé de quantification.
5. Elle écrit la configuration, crée les presets, et s'arrête si la
   configuration est invalide.

Chaque étape marque l'installation **en cours** ; rien n'est supprimé
pendant ce temps. L'annulation supprime le paquet, les presets, et le
dossier de l'installation si l'étape était atteinte.

**Le moteur MoE n'apparaît dans la liste qu'à partir du moment où le
preset existe.** Un moteur sans preset n'est pas listé, et une liste vide
n'est pas une panne : c'est l'absence du fichier de preset. La cause la
plus fréquente est une installation lancée depuis un binaire compilé avec
la variable d'environnement `AJEAN_HOME` pointée sur l'emplacement d'une
autre installation : l'installateur a écrit les presets dans l'autre
`data/`, et le moteur est bien installé. Redémarrez le bon binaire, sans
cette variable.

Détails complètes : [docs/moe-windows-prototype.md](docs/moe-windows-prototype.md).

## Mémoire sémantique

Mode d'accès aux notes : **Classique** (mots) ou **Hybride** (mots et
sens). Le choix vit dans Paramètres → Général ; les presets peuvent le
remplacer et choisir leur moteur d'embeddings, modèle, CPU/GPU, modalités,
dimensions et threads.

- Moteur d'embeddings local, processus séparé, **CPU par défaut**, avec un
  binaire GPU compatible en option.
- Modèle **EmbeddingGemma 2**, chemin texte compact via `llama-server`.
- Fusion **RRF** des résultats mots + sens. Les notes restent des notes
  Markdown, lues avec `mem_read`.

Contrôles mesurés et conditions de bascule :
[docs/embedding-memory-test.md](docs/embedding-memory-test.md).

## Lecture des pages JavaScript

Toujours activé par défaut : `web_search`, `web_open`, `web_read`,
`web_grep`. Pour les pages qui exécutent du JavaScript, ajoutez un serveur
**Crawl4AI** que vous hébergez :

```bash
ajean internet on
ajean internet engine crawl4ai http://127.0.0.1:11235 --bearer VOTRE_CLE
ajean internet engine ajean
```

Deux points qui font perdre du temps :

- `/crawl` et `/execute_js` s'appellent avec **Bearer**. L'authentification
  par en-tête `X-Api-Key` ne concerne que les endpoints de fichiers
  (`/health`, `/urls`, `/version`) ; Crawl4AI renvoie `401` sur `/crawl`
  avec `X-Api-Key`. La clé se lit en **démarrant** le serveur, pas en
  l'arrêtant.
- Le préfixe sert à **composer** l'URL de l'endpoint. Saisir
  `http://localhost:11235/crawl` comme base donne `404` : la base est
  `http://localhost:11235`.

## Base de données portable

Une installation recopiée sur un autre poste garde les chemins **absolus**
de l'installation d'origine dans sa base (`data/ajean.db`) : moteur MoE,
config, Python, moteur d'embeddings, modèle, binaire et modèles de
conversation. L'utilitaire les convertit en chemins **relatifs**, qui
suivent le dossier.

```bash
go build -o dbreloc.exe ./cmd/dbreloc
dbreloc.exe "data\ajean.db"            # diagnostic, lecture seule
dbreloc.exe "data\ajean.db" --write    # applique
```

Clés relocalisées : `MOE_CONFIG`, `MOE_SOURCE`, `MOE_PYTHON`,
`MEMORY_EMBED_BIN`, `MEMORY_EMBED_MODEL`, `BIN`, `MODEL`, `MMPROJ`,
`MODEL_DRAFT`. L'outil refuse d'écrire si une clé est incohérente ; il
n'y a rien à faire dans ce cas.

## Périmètre du dépôt

`data/` et `models/` sont exclus par `.gitignore`. Un contrôle de
transmission refuse désormais une livraison qui les inclurait, et
`git status --porcelain data/ models/` doit rester vide. Un dépôt de
travail propre **n'est pas** la preuve que les règles de transmission sont
posées : vérifier les règles, pas seulement le dépôt.

## Clés de configuration ajoutées

Dans `ajean edit`, ou `POST /api/config` avec `{"key":"…","value":"…"}`.

| Clé | Défaut | Rôle |
| --- | --- | --- |
| `MEMORY_SYSTEM` | `classic` | Système de recherche des notes : `classic` (les mots) ou `hybrid` (les mots et le sens). Choisit aussi dans Paramètres → Général. |
| `MEM_MODE` | `always` | Mode d'accès aux notes : `always`, `search`, `ondemand`, `off`. Réglable aussi par projet. |
| `MEMORY_EMBED_BIN` | vide | Moteur d'embeddings. Vide = désactivé, même en mode hybride. |
| `MEMORY_EMBED_MODEL` | vide | Modèle d'embeddings. Vide = désactivé. |
| `MEMORY_EMBED_PROVIDER` | `llama_cpp` | Moteur d'embeddings : `llama_cpp` (texte compact) ou `transformers` (encodeur complet). |
| `MEMORY_EMBED_DEVICE` | `cpu` | `cpu` ou `gpu`. |
| `MEMORY_EMBED_GPU_DEVICE` | vide | Identifiant du périphérique GPU (`ROCm0`, `Vulkan0`, `cuda:0`). Obligatoire en `gpu` ; `transformers` exige `cuda:N`, aussi sous ROCm. |
| `MEMORY_EMBED_MODALITIES` | `text` | Modalités de l'encodeur : `text` (obligatoire), `image`, `audio`, `video` — ces trois-là exigent `transformers`. |
| `MEMORY_EMBED_DIMENSIONS` | `768` | Dimensions : `128`, `256`, `512` ou `768`. |
| `MEMORY_EMBED_THREADS` | `8` | Threads d'encodage. |
| `MOE_RUNTIME` | vide | `strata-windows` active le moteur MoE Windows. |
| `MOE_BACKEND` | `hip` | `hip` (AMD) ou `cuda` (NVIDIA). |
| `MOE_TOOLKIT` | `13` | Version CUDA : `12` ou `13`. |
| `MOE_SOURCE` | vide | Dossier de l'installation MoE. |
| `MOE_PYTHON` | vide | Interpréteur Python de cette installation. |
| `MOE_CONFIG` | vide | Configuration du moteur MoE. |
| `MOE_KV` | `int8` | Quantification des clés. |
| `MOE_SPEC` | `3` | Spéculation. |
| `MOE_VISION` | `0` | Encodeur vision. |
| `MOE_VRAM_RESERVE` | `4096` | Mémoire vidéo laissée libre, en Mo. |

Chemins relatifs au dossier d'installation quand ils sont écrits comme
tels. **`MOE_SOURCE`, `MOE_PYTHON` et `MOE_CONFIG` ne se saisissent pas à
la main** : ils sont posés par l'installation depuis l'onglet Moteur.

Les presets MoE portent un nom de la forme
`FLASH NEXT SWIFT 1.5 IQ3_XXS · HIP` — jamais un nom de fichier.

## Version

| Élément | Valeur |
| --- | --- |
| Version de base (`internal/ajean/run.go`) | `const Version = "0.18.4"` |
| Suffixe public (`internal/ajean/parallel_profile.go`) | `buildFlavorSuffix = ".1 Cresius"` |
| Version affichée | `0.18.4.1 Cresius` |
| Étiquette Git | `v0.18.4.1+rev9` |
| Base officielle | `v0.18.4` — `76f60c0` |
| Moteur MoE embarqué | Strata **`0.1.44.1`** — `296b59d2…`, **inchangé** depuis `v0.18.4+rev8` |

### Le moteur MoE n'a pas changé de version

`moe_windows_manifest.json` est **octet pour octet le
même** blob Git (`3b606203…`, SHA-256 du fichier
`00237b50…`) à `8300729` — l'état embarqué dans la livraison `v0.18.4+rev8` —,
à `c0e4cb9` (l'étiquette `v0.18.4+rev8`) et à la tête de `main`. Son champ
`version` porte toujours **`0.1.44.1`**, commit Strata `296b59d2…`, et ses trois
URL de paquets pointent encore vers `releases/download/v0.1.44.1/`.

Le pack Windows HIP embarqué reste donc **Strata 0.1.44.1**. Les fichiers du
dépôt [Niko1221/Strata](https://github.com/Niko1221/Strata) ont bougé depuis
(0.1.45, 0.2.0, puis 0.2.1 et ses correctifs 0.2.1.1 à 0.2.1.3 sont sortis
entre-temps), **mais aucune de ces versions n'est embarquée ici** : ni le
paquet, ni le manifeste, ni le code d'intégration n'ont été touchés pour elles.
Passer à 0.1.45+ est une décision à part : le pack HIP pèse **633 588 636 o**
(taille vérifiée sur le fichier téléchargé
`data\moe\dl\strata-0.1.44.1\strata-windows-x64-hip.zip`, conforme au
manifeste), il faut le retélécharger et re-vérifier la répartition VRAM/RAM sur
la machine.

## Installation

`setup.exe` n'existe pas à ce stade. Installation :

1. `git clone https://github.com/Cresius34/ajean.git && cd ajean`
2. `git checkout v0.18.4.1+rev9`
3. `go build ./cmd/ajean`

Le paquet de test complet et ses contrôles de contrôle final sont décrits
dans [docs/moe-windows-prototype.md](docs/moe-windows-prototype.md).

---

# English

**Français** · English

Local revision **rev9** applied on top of the official base **v0.18.4**
(`76f60c0`). It is not published on `nathaninline/ajean`: it lives in the
`Cresius34/ajean` repository, tagged **`v0.18.4.1+rev9`**.

Five pieces of work:

| Area | What changes for you |
| --- | --- |
| **MoE engine** on Windows | Install a MoE model (AMD **HIP** or NVIDIA **CUDA**) from the Engine tab, no script. |
| **Semantic memory** | Note access in **Hybrid** mode: words **and** meaning, driven by a local embedding engine. |
| **JavaScript pages** | Pages rendered by JavaScript are read through a **Crawl4AI** server you host. |
| **Portable database** | A copied installation keeps the original install's paths; a utility relocates them. |
| **Repository scope** | `data/` and `models/` are never committed. |

## MoE engine on Windows

The **Engine** tab installs a **MoE** model — family **Swift 1.5**
(quantizations **IQ2_XS**, **IQ3_XXS**) or **Classique**, the original
Qwen3.8-Flash-Next (**Q2_0**, **IQ2_XS**, **IQ3_XXS**, **IQ3_S**). Two packs,
depending on your card:

- **AMD HIP** — ROCm **6.4** on Windows, **RX 7000** and **RX 9000** cards.
  Driver **AMD Software: Adrenalin Edition 25.93** or newer. Video memory
  reserve **4096 MB** by default, adjustable from **0** to **65536 MB**.
  The HIP pack is **text-only**: its vision encoder is not supplied, so
  enable vision on a CUDA pack.
- **NVIDIA CUDA** — CUDA **12.2+** or **13.2+** depending on the driver.
  Vision available.

Context runs from **32,768** to **262,144** tokens in steps of **4,096**
(**86,016** by default).

What the installer does, in order: it checks free space, driver, backend,
toolkit, context and reserve before downloading anything; downloads the
pack and verifies its size and checksums; installs Python and its
libraries, verifies the shipped files, verifies Python imports them, reads
the GPU's real capabilities; downloads the model and verifies its size and
quantization key; writes the configuration, creates the presets, and stops
if the configuration is invalid.

Each step marks the installation **in progress**; nothing is deleted in
the meantime. Cancelling removes the pack, the presets, and the
installation folder if that step was reached.

**The MoE engine only appears in the list once the preset exists.** An
engine without a preset is not listed, and an empty list is not a fault:
it is the absence of the preset file. The most common cause is an
installer run from a binary compiled with the `AJEAN_HOME` environment
variable pointing at another installation's location: it wrote the presets
into the other `data/`, and the engine is installed. Restart the right
binary, with no such variable.

Full detail: [docs/moe-windows-prototype.md](docs/moe-windows-prototype.md).

## Semantic memory

Note access mode: **Classic** (words) or **Hybrid** (words and meaning).
The choice lives in Settings → General; presets can override it and pick
their embedding engine, model, CPU/GPU, modalities, dimensions and
threads.

- Local embedding engine, separate process, **CPU by default**, with an
  optional compatible GPU binary.
- **EmbeddingGemma 2** model, compact text path via `llama-server`.
- **RRF** fusion of word and meaning results. Notes stay Markdown notes,
  read with `mem_read`.

Measured controls and cutover conditions:
[docs/embedding-memory-test.md](docs/embedding-memory-test.md).

## JavaScript pages

Always on by default: `web_search`, `web_open`, `web_read`, `web_grep`.
For pages that run JavaScript, add a **Crawl4AI** server you host:

```bash
ajean internet on
ajean internet engine crawl4ai http://127.0.0.1:11235 --bearer YOUR_KEY
ajean internet engine ajean
```

Two points that cost time:

- `/crawl` and `/execute_js` are called with **Bearer**. Header
  `X-Api-Key` authentication only covers the file endpoints (`/health`,
  `/urls`, `/version`); Crawl4AI returns `401` on `/crawl` with
  `X-Api-Key`. Read the key by **starting** the server, not by stopping
  it.
- The prefix is used to **build** the endpoint URL. Entering
  `http://localhost:11235/crawl` as the base gives `404`: the base is
  `http://localhost:11235`.

## Portable database

An installation copied to another machine keeps the **absolute** paths of
the original install in its database (`data/ajean.db`): MoE engine,
config, Python, embedding engine, model, binary and conversation models.
The utility converts them to **relative** paths, which follow the folder.

```bash
go build -o dbreloc.exe ./cmd/dbreloc
dbreloc.exe "data\ajean.db"            # diagnostic, read-only
dbreloc.exe "data\ajean.db" --write    # apply
```

Relocated keys: `MOE_CONFIG`, `MOE_SOURCE`, `MOE_PYTHON`,
`MEMORY_EMBED_BIN`, `MEMORY_EMBED_MODEL`, `BIN`, `MODEL`, `MMPROJ`,
`MODEL_DRAFT`. The tool refuses to write when a key is inconsistent; there
is nothing to do in that case.

## Repository scope

`data/` and `models/` are excluded by `.gitignore`. A delivery control now
rejects a delivery that would include them, and `git status --porcelain
data/ models/` must stay empty. A clean working repository is **not**
proof that the delivery rules are in place: check the rules, not just the
repository.

## Configuration keys added

In `ajean edit`, or `POST /api/config` with `{"key":"…","value":"…"}`.

| Key | Default | Role |
| --- | --- | --- |
| `MEMORY_SYSTEM` | `classic` | Note search system: `classic` (words) or `hybrid` (words and meaning). Also chosen in Settings → General. |
| `MEM_MODE` | `always` | Note access mode: `always`, `search`, `ondemand`, `off`. Also set per project. |
| `MEMORY_EMBED_BIN` | empty | Embedding engine. Empty = disabled, even in hybrid mode. |
| `MEMORY_EMBED_MODEL` | empty | Embedding model. Empty = disabled. |
| `MEMORY_EMBED_PROVIDER` | `llama_cpp` | Embedding engine: `llama_cpp` (compact text) or `transformers` (full encoder). |
| `MEMORY_EMBED_DEVICE` | `cpu` | `cpu` or `gpu`. |
| `MEMORY_EMBED_GPU_DEVICE` | empty | GPU device id (`ROCm0`, `Vulkan0`, `cuda:0`). Required with `gpu`; `transformers` needs `cuda:N`, also under ROCm. |
| `MEMORY_EMBED_MODALITIES` | `text` | Encoder modalities: `text` (required), `image`, `audio`, `video` — the last three need `transformers`. |
| `MEMORY_EMBED_DIMENSIONS` | `768` | Dimensions: `128`, `256`, `512` or `768`. |
| `MEMORY_EMBED_THREADS` | `8` | Encoding threads. |
| `MOE_RUNTIME` | empty | `strata-windows` enables the Windows MoE engine. |
| `MOE_BACKEND` | `hip` | `hip` (AMD) or `cuda` (NVIDIA). |
| `MOE_TOOLKIT` | `13` | CUDA version: `12` or `13`. |
| `MOE_SOURCE` | empty | MoE installation folder. |
| `MOE_PYTHON` | empty | That installation's Python interpreter. |
| `MOE_CONFIG` | empty | MoE engine configuration. |
| `MOE_KV` | `int8` | Key quantization. |
| `MOE_SPEC` | `3` | Speculation. |
| `MOE_VISION` | `0` | Vision encoder. |
| `MOE_VRAM_RESERVE` | `4096` | Video memory left free, in MB. |

Paths are relative to the installation folder when written that way.
**`MOE_SOURCE`, `MOE_PYTHON` and `MOE_CONFIG` are not typed by hand**: the
Engine tab's installer writes them.

MoE presets carry a name of the form `FLASH NEXT SWIFT 1.5 IQ3_XXS · HIP` —
never a file name.

## Version

| Item | Value |
| --- | --- |
| Base version (`internal/ajean/run.go`) | `const Version = "0.18.4"` |
| Public suffix (`internal/ajean/parallel_profile.go`) | `buildFlavorSuffix = ".1 Cresius"` |
| Version displayed | `0.18.4.1 Cresius` |
| Git tag | `v0.18.4.1+rev9` |
| Official base | `v0.18.4` — `76f60c0` |
| Bundled MoE engine | Strata **`0.1.44.1`** — `296b59d2…`, **unchanged** since `v0.18.4+rev8` |

### The MoE engine did not change version

`moe_windows_manifest.json` is **byte-for-byte the same** Git blob
(`3b606203…`, file SHA-256 `00237b50…`) at `8300729` — the state shipped in
`v0.18.4+rev8` — at `c0e4cb9` (the `v0.18.4+rev8` tag) and at the tip of
`main`. Its `version` field still reads **`0.1.44.1`**, Strata commit
`296b59d2…`, and its three package URLs still point at
`releases/download/v0.1.44.1/`.

The bundled Windows HIP pack is still **Strata 0.1.44.1**: files in
[Niko1221/Strata](https://github.com/Niko1221/Strata) did move since
(0.1.45, 0.2.0, then 0.2.1 with fixes 0.2.1.1 to 0.2.1.3 were released
meanwhile), **but none of those versions is bundled here** — not the package,
not the manifest, not the integration code. Moving to 0.1.45+ is a decision of
its own: the HIP pack weighs **633,588,636 bytes** (measured on the downloaded
file `data\moe\dl\strata-0.1.44.1\strata-windows-x64-hip.zip`, matching the
manifest), it has to be re-downloaded and the VRAM/RAM split re-verified on
the machine.

## Installation

`setup.exe` does not exist at this stage. Install:

1. `git clone https://github.com/Cresius34/ajean.git && cd ajean`
2. `git checkout v0.18.4.1+rev9`
3. `go build ./cmd/ajean`

The full test package and its final controls are described in
[docs/moe-windows-prototype.md](docs/moe-windows-prototype.md).
