# Notes de version — AJEAN `0.18.9.1 Cresius` / étiquette `v0.18.9.1+rev11`

**Projet dérivé, usage personnel.** Cette version est construite sur le dépôt
`Cresius34/ajean`, fork de [**nathaninline/ajean**](https://github.com/nathaninline/ajean)
— le projet principal, et la seule source des versions publiées d'AJEAN. Les
correctifs décrits ci-dessous forment une **ligne privée** : ils ne sont ni
validés sur d'autres machines, ni garantis transposables. Pour installer AJEAN,
prenez les versions publiées sur
[nathaninline/ajean](https://github.com/nathaninline/ajean/releases/latest).

Cette version **absorbe l'amont `v0.18.9`** — qui va de
[`v0.18.5`](#amont-v0185--tâches-et-consolidation) à
[`v0.18.9`](#amont-v0189--menu-latéral-paramètres-tâches-api-ajean-link) —
dans la ligne privée `Cresius`, qui s'arrêtait à `0.18.4.2+rev10`. Le socle est
donc celui de l'amont, et la ligne privée se réduit à ce qu'elle apporte encore.

## En-tête

Menu latéral plus clair (amont `v0.18.9`) ; le moteur MoE **Strata** passe en
`0.1.44.1` avec réglage de la fenêtre de contexte et de la quantification
clé/valeurs ; la dédup d'appels ne masque plus une lecture d'état vivant ; la
base s'ouvre en mode immutable pour survivre à une copie maladroite.

## Ce que cette version ajoute à l'amont `v0.18.9`

- **Moteur MoE Strata** : c'est `0.1.44.1` qui est installé (5,47 Go, SHA-256
  vérifié), et non le paquet `0.1.43.1` décrit dans les notes de la `rev10`.
- **Réglage de la fenêtre de contexte (`CtxMax`)** : la jauge et le compactage
  suivent la limite réellement appliquée par le moteur.
- **Quantification clé/valeurs (`KvQuant`)** : quatre choix dans l'interface,
  dont `k8v4` sur les moteurs récents. Une clé **inconnue** est refusée et
  signalée — plus de bascule silencieuse vers un préréglage.
- **Dédup d'appels (correctif A)** : une lecture d'état vivant — `mem_read`,
  `mem_search`, `recall`, `task_list`, `tracker`, `inspect`, et les outils
  propres à un poste (`jean_*`) — n'est **plus jamais** masquée. Le blocage
  observé en `0.18.4.1` (« rappel trop demandé », contenu fantôme rendu par une
  réponse déjà vue) est corrigé ici.
- **Base de données portable** : un utilitaire qui rend transportable une base
  `ajean.db` copiée sur une autre machine (chemins absolus → relatifs) ; la base
  peut s'ouvrir en **mode immutable**, qui évite qu'une copie partielle ne la
  corrompe.
- **Périmètre du dépôt** : `data/` et `models/` sont exclus du dépôt (poids
  ré-installables par le moteur lui-même).

## Ce qui disparaît de la ligne privée, repris par l'amont

Ces Correctifs Cresius ont été **abandonnés** faute d'objet, l'amont ayant tranché
dans le même sens :

| Correctif Cresius | Sort |
| --- | --- |
| Crawl4AI (`crawl4ai-repair`) | **retiré** — l'amont a supprimé Crawl4AI en `v0.18.5` au profit d'un moteur de lecture natif |
| Moteur d'embeddings local (`embeddingv2`) | **non repris** — l'amont fournit le même service en `v0.18.7` (mode Jean) |
| Réparation des appels d'outils (`toolfix`) | **non repris** — l'amont l'a corrigé en `v0.18.6` |

## Amont `v0.18.9` — menu latéral : Paramètres, Tâches, API, AJEAN LINK

- **Connexion d'un appareil guidée** : la ligne « Connecter un nouvel appareil »
  de la section AJEAN LINK ouvre une fenêtre en trois étapes : ouvrir l'adresse
  d'accès distant, vérifier l'empreinte, saisir le code. Le code, valable
  10 minutes et à usage unique, se copie en un clic et peut être renouvelé.
- **API** : une carte Connexion regroupe l'adresse, le modèle et la clé, chacun
  avec son bouton de copie. Les actions sur la clé (générer, définir, retirer)
  sont dans un menu. Une carte Accès regroupe le réseau local et l'accès public.
- **AJEAN LINK** : l'adresse d'accès distant et l'état du tunnel (en ligne ou
  arrêté) sont réunis sur une seule carte. L'état de la sauvegarde s'affiche sous
  son interrupteur.
- **Tâches** : l'interrupteur général, la liste et l'ajout tiennent dans une
  seule carte. Une tâche en échec est signalée par sa pastille rouge, le message
  d'erreur étant visible au survol.
- **Paramètres** : l'état de l'accès internet et du contrôle du navigateur
  s'affiche sous leur interrupteur. Les délais MCP sont des lignes de réglage
  comme les autres.
- **Boutons** : les actions du menu (ajouter, sauvegarder, restaurer,
  déconnecter…) ont une icône et un fond au survol, pour ne plus se confondre
  avec les lignes de réglage. Les boutons de copie confirment la copie par une
  coche.

## Amont `v0.18.8`

Applications mieux intégrées à l'interface, et choix de vue retenu en mode Jean.

## Amont `v0.18.7` — mode Jean

Mode Jean : une mémoire qui revient d'elle-même, qui retient les correctifs, et
un contexte gardé propre.

## Amont `v0.18.6`

Outils plus robustes avec les modèles locaux : appels réparés, recherche web qui
ne s'arrête plus à un moteur, lecture ciblée des longues pages.

## Amont `v0.18.5` — tâches et consolidation

Tâches et consolidation de Jean plus fiables, accès internet simplifié.

## Clés de configuration de la ligne privée

| Clé | Emplacement | Effet |
| --- | --- | --- |
| `AJEAN_MOE_STRATA_VERSION` | variable d'environnement | force la version du pack Strata à installer |
| `AJEAN_MOE_STRATA_DIR` | variable d'environnement | installe le moteur ailleurs que dans `data\moe` |
| `AJEAN_MOE_STRATA_URL` | variable d'environnement | installe depuis une archive locale (pas de téléchargement) |
| `AJEAN_MOE_CTX_MAX` | variable d'environnement ou preset `.env` | fenêtre de contexte demandée au moteur |
| `AJEAN_MOE_KV_QUANT` | variable d'environnement ou preset `.env` | quantification clé/valeurs (`fp16`, `int8`, `q4_0`, `k8v4`) |
| `AJEAN_HOME` | variable d'environnement | **piège connu** : un build parallèle qui la définit écrit son moteur dans un autre dossier |

## Périmètre du dépôt

```
/data/
/models/
```

La règle : on ne pousse que ce qui ne peut pas être ré-installé, ni par le
moteur sur demande, ni par les scripts d'installation du dépôt. Le paquet de
livraison applique la même règle
(`data\scripts\Preparer-Paquet-Sans-Data.ps1`).

## Version

| Élément | Valeur |
| --- | --- |
| Version affichée | `0.18.9.1 Cresius` |
| Étiquette Git | `v0.18.9.1+rev11` |
| Socle amont | `v0.18.9` |
| Moteur MoE | Strata `0.1.44.1` (5,47 Go, SHA-256 vérifié) |
| Précédent | `0.18.4.2+rev10`, étiquette `v0.18.4.2+rev10` |

### Le moteur MoE a changé de version

`rev10` décrivait le pack **Strata `0.1.43.1`**. La présente version installe
**`0.1.44.1`** : le manifeste interne (`internal/ajean/moe_windows_manifest.json`)
pointe sur l'archive `0.1.44.1`, avec son empreinte SHA-256. Aucun pack
`0.1.43.1` n'est présent dans le dépôt.

## Installation

```
ajean update
```

ou, depuis le dépôt :

1. `git clone https://github.com/Cresius34/ajean`
2. `git checkout v0.18.9.1+rev11`
3. `go build -o ajean.exe ./cmd/ajean`

---

# English

# Release notes — AJEAN `0.18.9.1 Cresius` / tag `v0.18.9.1+rev11`

**Derived project, personal use.** This build comes from the `Cresius34/ajean`
repository, a fork of [**nathaninline/ajean**](https://github.com/nathaninline/ajean)
— the upstream project, and the only source of published AJEAN releases. The
fixes below form a **private patch line**: they are not validated on other
machines and are not guaranteed transposable. To install AJEAN, use the
releases published at
[nathaninline/ajean](https://github.com/nathaninline/ajean/releases/latest).

This build **absorbs upstream `v0.18.9`** — which covers
[`v0.18.5`](#upstream-v0185--tasks-and-consolidation) through
[`v0.18.9`](#upstream-v0189--sidebar-settings-tasks-api-ajean-link) — into the
`Cresius` patch line, which stopped at `0.18.4.2+rev10`. The base is now
upstream's; the private line is reduced to what it still adds.

## Headline

A clearer sidebar (upstream `v0.18.9`); the **Strata** MoE engine moves to
`0.1.44.1` with context-window and key/value quantification settings; call
de-duplication no longer masks a live-state read; the database can be opened
immutable to survive a clumsy copy.

## What this build adds on top of upstream `v0.18.9`

- **Strata MoE engine**: it installs `0.1.44.1` (5.47 GB, SHA-256 verified),
  not the `0.1.43.1` pack described in the `rev10` notes.
- **Context window (`CtxMax`)**: the gauge and the compaction follow the limit
  actually applied by the engine.
- **Key/value quantization (`KvQuant`)**: four choices in the interface,
  including `k8v4` on recent engines. An **unknown** key is rejected and
  reported — no more silent fallback to a preset.
- **Call de-duplication (fix A)**: a live-state read — `mem_read`,
  `mem_search`, `recall`, `task_list`, `tracker`, `inspect`, and machine-specific
  tools (`jean_*`) — is **never** masked any more. The `0.18.4.1` failure
  ("recall requested too often", phantom content returned by an already-seen
  answer) is fixed here.
- **Portable database**: a utility that makes a copied `ajean.db` transportable
  across machines (absolute paths → relative); the database can be opened in
  **immutable mode**, which prevents a partial copy from corrupting it.
- **Repository scope**: `data/` and `models/` are kept out of the repository
  (weights the engine can re-install by itself).

## Dropped from the private line, taken over by upstream

These Cresius fixes were **abandoned** as moot — upstream made the same call:

| Cresius fix | Outcome |
| --- | --- |
| Crawl4AI (`crawl4ai-repair`) | **removed** — upstream dropped Crawl4AI in `v0.18.5` for a native reader |
| Local embedding engine (`embeddingv2`) | **not carried over** — upstream delivers the same service in `v0.18.7` (Jean mode) |
| Tool-call repair (`toolfix`) | **not carried over** — upstream fixed it in `v0.18.6` |

## Upstream `v0.18.9` — sidebar: Settings, Tasks, API, AJEAN LINK

- **Guided device pairing**: the "Connect a new device" row in the AJEAN LINK
  section opens a three-step window — open the remote access address, check the
  fingerprint, enter the code. The code is valid for 10 minutes, single-use,
  copyable in one click, and can be renewed.
- **API**: a Connection card groups the address, the model and the key, each
  with its own copy button. Key actions (generate, set, remove) sit in a menu.
  An Access card groups the local network and public access.
- **AJEAN LINK**: the remote access address and the tunnel state (online or
  stopped) share one card. The backup state shows under its switch.
- **Tasks**: the master switch, the list and the add form fit in a single card.
  A failing task is flagged by its red dot, with the error visible on hover.
- **Settings**: the internet access and browser control states show under their
  switch. MCP timeouts are tuning rows like the others.
- **Buttons**: menu actions (add, save, restore, sign out…) get an icon and a
  hover background, so they no longer look like tuning rows. Copy buttons
  confirm the copy with a check mark.

## Upstream `v0.18.8`

Apps are better integrated in the interface, and the chosen view is retained in
Jean mode.

## Upstream `v0.18.7` — Jean mode

Jean mode: memory that comes back on its own, keeps the fixes, and a context
kept clean.

## Upstream `v0.18.6`

More robust tools with local models: repaired calls, web search that no longer
stops at one engine, targeted reading of long pages.

## Upstream `v0.18.5` — tasks and consolidation

Jean's tasks and consolidation are more reliable; internet access is simpler.

## Configuration keys of the private line

| Key | Where | Effect |
| --- | --- | --- |
| `AJEAN_MOE_STRATA_VERSION` | environment variable | forces the Strata pack version to install |
| `AJEAN_MOE_STRATA_DIR` | environment variable | installs the engine somewhere other than `data\moe` |
| `AJEAN_MOE_STRATA_URL` | environment variable | installs from a local archive (no download) |
| `AJEAN_MOE_CTX_MAX` | environment variable or `.env` preset | context window requested from the engine |
| `AJEAN_MOE_KV_QUANT` | environment variable or `.env` preset | key/value quantization (`fp16`, `int8`, `q4_0`, `k8v4`) |
| `AJEAN_HOME` | environment variable | **known trap**: a parallel build that sets it writes its engine to another folder |

## Repository scope

```
/data/
/models/
```

The rule: only push what cannot be re-installed, either by the engine on demand
or by the repository's install scripts. The delivery package applies the same
rule (`data\scripts\Preparer-Paquet-Sans-Data.ps1`).

## Version

| Item | Value |
| --- | --- |
| Version displayed | `0.18.9.1 Cresius` |
| Git tag | `v0.18.9.1+rev11` |
| Upstream base | `v0.18.9` |
| MoE engine | Strata `0.1.44.1` (5.47 GB, SHA-256 verified) |
| Previous | `0.18.4.2+rev10`, tag `v0.18.4.2+rev10` |

### The MoE engine did change version

`rev10` described the **Strata `0.1.43.1`** pack. This build installs
**`0.1.44.1`**: the internal manifest
(`internal/ajean/moe_windows_manifest.json`) points at the `0.1.44.1` archive
with its SHA-256 fingerprint. No `0.1.43.1` pack is present in the repository.

## Installation

```
ajean update
```

or, from the repository:

1. `git clone https://github.com/Cresius34/ajean`
2. `git checkout v0.18.9.1+rev11`
3. `go build -o ajean.exe ./cmd/ajean`
