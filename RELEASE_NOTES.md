# Notes de version — AJEAN `0.18.11.1 Cresius` / étiquette `v0.18.11+rev1`

**Projet dérivé, usage personnel.** Cette version est construite sur le dépôt
`Cresius34/ajean`, fork de [**nathaninline/ajean**](https://github.com/nathaninline/ajean)
— le projet principal, et la seule source des versions publiées d'AJEAN. Les
correctifs décrits ci-dessous forment une **ligne privée** : ils ne sont ni
validés sur d'autres machines, ni garantis transposables. Pour installer AJEAN,
prenez les versions publiées sur
[nathaninline/ajean](https://github.com/nathaninline/ajean/releases/latest).

Cette version **absorbe l'amont `v0.18.11`** — donc aussi
[`v0.18.10`](#amont-v01810--mode-jean-actions-décrites-terminal-sommeil-v2) —
dans la ligne privée `Cresius`, qui s'arrêtait à `0.18.9.1+rev11`. Le socle est
celui de l'amont ; la ligne privée se réduit à ce qu'elle apporte encore.

## En-tête

Le navigateur piloté sait remplir les formulaires des sites modernes et s'arrête
quand un site le bloque (amont `v0.18.11`) ; en mode Jean, chaque action est
décrite et le sommeil range vraiment la mémoire (amont `v0.18.10`). Côté ligne
privée : le **journal de Jean** et la **mémoire sémantique** partagent désormais
un **seul encodeur**, EmbeddingGemma 2 — un seul GGUF, un seul moteur, un seul
espace vectoriel sur le poste.

## Amont `v0.18.11` — navigateur piloté, scène de Jean après l'envoi

Le navigateur piloté par l'IA sait remplir les formulaires des sites modernes,
et s'arrête quand un site le bloque.

- **Choix impossibles à cocher sur certains sites** : sur une boutique en ligne,
  les options d'un produit (couleur, capacité…) n'apparaissaient pas dans la
  liste des éléments, et un clic dessus restait sans effet. Le bouton suivant
  restait grisé, et l'IA recommençait sans fin. Ces cases cachées derrière une
  étiquette sont maintenant listées et cliquées à travers l'étiquette, et le
  cochage est vérifié.
- **États visibles** : la liste des éléments indique ce qui est coché,
  sélectionné ou désactivé. Un clic sur un bouton désactivé est refusé, avec
  l'explication qu'un choix obligatoire manque.
- **Lecture du texte de la page** : la recherche dans la page trouve aussi le
  texte simple (une date de livraison, un prix, un message), et plus seulement
  les boutons et les liens.
- **Fin de tâche plus nette** : quand un site répond par une page d'erreur ou de
  blocage après une action normale, l'IA en est avertie et passe à une autre
  piste au lieu d'insister. Une même action répétée sans effet est signalée, et
  un rappel de conclure arrive au bout de 25 actions.
- **Mode Jean, juste après l'envoi d'un message** : après une mise à jour ou un
  rafraîchissement de la page, la scène affichait quelques secondes « Clique sur
  moi pour me parler » au lieu de « Lit le message… ».

## Amont `v0.18.10` — mode Jean : actions décrites, terminal, sommeil v2

Mode Jean : on voit ce qu'il fait, et son sommeil range vraiment la mémoire.

- **Chaque action de Jean est décrite** : en mode Jean, chaque outil utilisé
  s'accompagne d'une phrase qui dit ce qu'il fait et pourquoi (« Je regarde
  l'image de midi pour voir si la caméra est propre »). Les étapes restent dans
  le fil sous forme de liste sobre ; l'étape en cours s'affiche sur la ligne
  d'état.
- **Terminal en direct dans la scène** : en plein écran, quand Jean lance une
  commande, une fenêtre de terminal s'ouvre au-dessus de lui. La commande
  s'écrit lettre par lettre, puis le résultat apparaît.
- **Résultats de recherche web dans la scène** : une recherche ouvre une carte
  avec la requête et les résultats (site, titre, extrait), qui arrivent en
  cascade.
- **Heure du sommeil réglable et sommeil à la demande** : l'heure à laquelle Jean
  range sa mémoire se choisit dans sa mémoire (onglet Sommeil), et un bouton
  « Dormir maintenant » le lance tout de suite. Pendant le sommeil, Jean dort
  dans la scène.
- **Historique des sommeils** : l'onglet Sommeil montre, pour chacun des 14
  derniers sommeils, ce qui a changé fichier par fichier, avec un bouton
  « Revenir à avant ce sommeil ». Les fiches archivées y sont listées et peuvent
  être ressorties.
- **Sommeil plus efficace** : sur une mémoire volumineuse, le sommeil ne voyait
  que la liste des fiches et ne fusionnait jamais rien. Les fiches qui se
  ressemblent lui sont maintenant données en entier, par groupes, et un outil de
  fusion verse une fiche entière dans une autre avant de l'archiver.
- **Sommeil plus sûr** : un sommeil interrompu (message de l'utilisateur,
  redémarrage) est entièrement annulé ; aucun fichier ne peut être écrit pendant
  le sommeil ; son bilan est toujours noté dans le journal.
- **Mémoire de Jean en onglets** : Profil, Leçons, Fiches, Sommeil et Journal se
  choisissent avec un sélecteur, au lieu d'une longue page à faire défiler.
- **Images cassées dans les réponses** : une image citée par son seul nom alors
  qu'elle est rangée dans un sous-dossier est retrouvée.

## Ce que cette version ajoute à l'amont `v0.18.11`

- **Un seul encodeur pour l'appareil** : le journal de Jean demandait son propre
  modèle d'embeddings (l'ancien 300 M, 512 dimensions) pendant que la mémoire
  sémantique et la recherche utilisaient déjà **EmbeddingGemma 2** (768
  dimensions). Deux modèles, donc deux espaces vectoriels sur le même poste : ce
  que le journal retrouvait n'avait rien à voir avec ce que la mémoire
  retrouvait. Le journal utilise maintenant le modèle que l'appareil a déjà
  choisi pour les embeddings, avec les mêmes réglages que l'encodeur mémoire
  (mêmes dimensions, même pooling, même périphérique), et ne relance pas un
  moteur déjà vivant. L'identifiant écrit dans les fichiers `.embed` passe de
  `egemma300-v1` à `egemma2-q8-v1` : les fichiers portant l'ancien sceau sont
  refusés et le journal se recalcule, au lieu de fusionner silencieusement deux
  espaces dans le même index.
- **Contrôles de largeur de vecteur** : une réponse du moteur qui n'a pas la
  largeur attendue (768), qui contient `NaN` ou `±Infini`, ou qui ne renvoie pas
  un vecteur par texte, est rejetée. Sans ces contrôles, l'index se remplit de
  vecteurs faux pendant que tous les autres voyants restent au vert.
- **Moteur MoE Strata** : la release vendue est `0.2.4`, paquet moteur `0.1.48`,
  contrôle `0.1.49.2` (SHA-256 vérifiés, une archive par carte). Voir
  [le point sur le moteur](#le-moteur-moe-où-en-sommes-nous).
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

Ces correctifs Cresius ont été **abandonnés** faute d'objet, l'amont ayant tranché
dans le même sens :

| Correctif Cresius | Sort |
| --- | --- |
| Crawl4AI (`crawl4ai-repair`) | **retiré** — l'amont a supprimé Crawl4AI en `v0.18.5` au profit d'un moteur de lecture natif |
| Moteur d'embeddings local embarqué (`embeddingv2`) | **non repris** — l'amont fournit le même service en `v0.18.7` (mode Jean) |
| Réparation des appels d'outils (`toolfix`) | **non repris** — l'amont l'a corrigé en `v0.18.6` |

## Le moteur MoE, où en sommes-nous

La bascule est **faite** dans ce build : la release vendue est **Strata `0.2.4`**
(commit source `96ca438`), le **paquet moteur `0.1.48`** et le **paquet de
contrôle `0.1.49.2`**. Ces trois numéros sont désormais déclarés séparément dans
`internal/ajean/backend_moe_strata.go` et ne doivent pas être confondus : seul le
numéro du paquet moteur est comparé au cachet `engine/BUILD.json` par le contrôle
préalable.

Ce que `0.2.4` change pour nous, mesuré sur le poste :

- **Les images sur AMD (HIP) sont disponibles.** Le paquet moteur `0.1.48`
  embarque le lecteur d'images AMD : son cachet `BUILD.json` porte
  `vision: "cpu+hip"`, avec `strata-vision-hip.exe` et un repli
  `strata-vision.exe`. Les garde-fous qui refusaient la vision sur HIP sont donc
  levés (contrôle préalable, réglages, fiche moteur). La section vision n'est
  conservée que si le preset la demande (`MOE_VISION=1`) ; sinon elle est retirée,
  comme avant. Un test garde ce comportement : `TestStrataHipVisionPackage`.
- **Une archive par carte, plus d'archive plate.** Le manifeste interne
  (`internal/ajean/moe_windows_manifest.json`) publie le contrôle `0.1.49.2` et
  **cinq** paquets moteur `tar.gz` par carte — `hip-rdna2`, `hip-rdna3`,
  `hip-rx9070` (la carte de ce poste, `gfx1201`), `cuda12`, `cuda13` — avec
  SHA-256 et tailles relevés sur le flux en direct (séquence 112).
- **Le script d'installation portable** (`moe_portable_install.ps1`) défait une
  archive `tar.gz` au lieu d'un `zip`, estampille `engine/BUILD.json` avec
  `source: prebuilt`, et choisit le paquet selon la carte. Il reste lisible en
  PowerShell 5.
- **Notre enveloppe `moe_portable_setup.py`** suit le nouvel appel de `data_folder`
  en `0.2.4` (`request, move, yes, set_default`).

Installation portée de bout en bout et vérifiée sur ce poste : moteur extrait,
`llama.cpp 3cf0325` (commit épinglé par `0.2.4`), poids `IQ3_S`, `mmproj`, pack et
MTP préparés ; moteur exécuté (`strata 0.1.48`, familles `qwen4exp` et
`qwen35moe`) ; réponse réelle obtenue du moteur lancé par l'application.

Un point d'installation à connaître, mesuré : l'emplacement du moteur est dérivé
de la version épinglée (`data\moe\strata-<version>-<carte>`). Un `data` récupéré
d'une installation plus ancienne garde donc l'ancien moteur sous l'ancien nom, et
le contrôle préalable refuse de le charger — c'est le message
« moteur MoE différent de la version figée ». Il faut installer le moteur depuis
l'interface Moteur, ou promouvoir le paquet `0.1.48` à l'emplacement épinglé.

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
| Version affichée | `0.18.11.1 Cresius` |
| Étiquette Git | `v0.18.11+rev1` |
| Socle amont | `v0.18.11` |
| Moteur MoE | Strata `0.2.4` — paquet moteur `0.1.48`, contrôle `0.1.49.2`, cinq paquets par carte, SHA-256 vérifiés |
| Encodeur d'embeddings | EmbeddingGemma 2 `Q8_0`, 768 dimensions (mémoire **et** journal) |
| Précédent | `0.18.9.1 Cresius`, étiquette `v0.18.9.1+rev11` |

La règle de nommage de la ligne privée : **le numéro de l'amont, tel quel, suivi
de notre numéro de révision**. `0.18.11` + révision `1` → `0.18.11.1 Cresius`,
étiquette `v0.18.11+rev1`. Les deux désignent le même build.

## Installation

```
ajean update
```

ou, depuis le dépôt :

1. `git clone https://github.com/Cresius34/ajean`
2. `git checkout v0.18.11+rev1`
3. `go build -o ajean.exe ./cmd/ajean`

---

# English

# Release notes — AJEAN `0.18.11.1 Cresius` / tag `v0.18.11+rev1`

**Derived project, personal use.** This build comes from the `Cresius34/ajean`
repository, a fork of [**nathaninline/ajean**](https://github.com/nathaninline/ajean)
— the upstream project, and the only source of published AJEAN releases. The
fixes below form a **private patch line**: they are not validated on other
machines and are not guaranteed transposable. To install AJEAN, use the
releases published at
[nathaninline/ajean](https://github.com/nathaninline/ajean/releases/latest).

This build **absorbs upstream `v0.18.11`** — and therefore
[`v0.18.10`](#upstream-v01810--jean-mode-described-actions-terminal-sleep-v2) —
into the `Cresius` patch line, which stopped at `0.18.9.1+rev11`. The base is
upstream's; the private line is reduced to what it still adds.

## Headline

The controlled browser can fill modern web forms and stops when a site blocks it
(upstream `v0.18.11`); in Jean mode every action is described and Jean's sleep
really tidies memory (upstream `v0.18.10`). On the private side: **Jean's
journal** and **semantic memory** now share a **single encoder**, EmbeddingGemma
2 — one GGUF, one engine, one vector space on the machine.

## Upstream `v0.18.11` — controlled browser, Jean scene after sending

The AI-controlled browser can fill modern web forms, and stops when a site blocks it.

- **Choices that could not be ticked on some sites**: on an online store, product
  options (colour, capacity…) were missing from the element list, and clicking
  them did nothing. The next button stayed greyed out and the AI looped. These
  checkboxes hidden behind a label are now listed, clicked through the label,
  and the tick is verified.
- **Visible states**: the element list shows what is checked, selected or
  disabled. Clicking a disabled button is refused, with the explanation that a
  required choice is missing.
- **Reading page text**: in-page search now finds plain text (a delivery date, a
  price, a message), not only buttons and links.
- **Cleaner task ending**: when a site answers with an error or block page after
  a normal action, the AI is told and tries another route instead of insisting.
  A repeated no-effect action is flagged, and a reminder to conclude arrives
  after 25 actions.
- **Jean mode, right after sending a message**: after an update or page refresh,
  the scene showed "Click me to talk" for a few seconds instead of "Reading the
  message…".

## Upstream `v0.18.10` — Jean mode: described actions, terminal, sleep v2

Jean mode: you see what he does, and his sleep really tidies memory.

- **Every Jean action is described**: in Jean mode, each tool used comes with a
  sentence saying what it does and why. Steps stay in the thread as a sober
  list; the current step shows on the status line.
- **Live terminal in the scene**: in full screen, when Jean runs a command, a
  terminal window opens above him; the command is typed letter by letter, then
  the result appears.
- **Web search results in the scene**: a search opens a card with the query and
  the results (site, title, snippet), arriving in cascade.
- **Adjustable sleep time and on-demand sleep**: the time at which Jean tidies
  his memory is set in his memory (Sleep tab), and a "Sleep now" button starts
  it immediately. During sleep, Jean sleeps in the scene.
- **Sleep history**: the Sleep tab shows, for each of the last 14 sleeps, what
  changed file by file, with a "Go back to before this sleep" button. Archived
  cards are listed there and can be restored.
- **More effective sleep**: on a large memory, sleep only saw the list of cards
  and never merged anything. Similar cards are now given in full, in groups, and
  a merge tool pours one whole card into another before archiving it.
- **Safer sleep**: an interrupted sleep (user message, restart) is fully rolled
  back; no file can be written during sleep; its report is always logged.
- **Jean memory in tabs**: Profile, Lessons, Cards, Sleep and Journal are chosen
  with a selector instead of one long scrolling page.
- **Broken images in answers**: an image cited by name only while stored in a
  subfolder is now found.

## What this build adds on top of upstream `v0.18.11`

- **One encoder for the whole machine**: Jean's journal asked for its own
  embedding model (the old 300 M, 512 dimensions) while semantic memory and
  search already used **EmbeddingGemma 2** (768 dimensions). Two models meant
  two vector spaces on the same machine: what the journal retrieved had nothing
  to do with what memory retrieved. The journal now uses the model the device
  already chose for embeddings, with the same settings as the memory encoder
  (same dimensions, same pooling, same device), and does not restart an engine
  that is already running. The tag written into each `.embed` file moves from
  `egemma300-v1` to `egemma2-q8-v1`: files carrying the old seal are rejected
  and the journal is recomputed, instead of silently merging two vector spaces
  into one index.
- **Vector width checks**: an engine answer with the wrong width (768),
  containing `NaN` or `±Infinity`, or not returning one vector per text, is
  rejected. Without these checks the index fills with wrong vectors while every
  other indicator stays green.
- **Strata MoE engine**: the vendored release is `0.2.4`, engine package `0.1.48`,
  control `0.1.49.2` (SHA-256 verified, one archive per card). See
  [the engine status](#the-moe-engine-where-we-stand).
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

## The MoE engine, where we stand

The move is **done** in this build: the vendored release is **Strata `0.2.4`**
(source commit `96ca438`), the **engine package is `0.1.48`** and the **control
package `0.1.49.2`**. Those three numbers are now declared separately in
`internal/ajean/backend_moe_strata.go` and must not be conflated: only the engine
package number is compared against the `engine/BUILD.json` stamp by the preflight
check.

What `0.2.4` changes for us, measured on this machine:

- **Pictures on AMD (HIP) are available.** The `0.1.48` engine package ships the
  AMD picture reader: its `BUILD.json` stamp carries `vision: "cpu+hip"`, with
  `strata-vision-hip.exe` and a `strata-vision.exe` fallback. The guards that
  refused vision on HIP are therefore lifted (preflight, settings, engine
  details). The vision section is kept only when the preset asks for it
  (`MOE_VISION=1`); otherwise it is dropped, as before. A test pins this
  behaviour: `TestStrataHipVisionPackage`.
- **One archive per card, no flat archive.** The internal manifest
  (`internal/ajean/moe_windows_manifest.json`) publishes control `0.1.49.2` and
  **five** engine `tar.gz` packages per card — `hip-rdna2`, `hip-rdna3`,
  `hip-rx9070` (this machine's card, `gfx1201`), `cuda12`, `cuda13` — with
  SHA-256 and sizes read from the live feed (sequence 112).
- **The portable install script** (`moe_portable_install.ps1`) unpacks a `tar.gz`
  instead of a `zip`, stamps `engine/BUILD.json` with `source: prebuilt`, and
  picks the package matching the card. It stays readable on PowerShell 5.
- **Our `moe_portable_setup.py` wrapper** follows the new `0.2.4` signature of
  `data_folder` (`request, move, yes, set_default`).

The install was carried end to end and verified on this machine: engine unpacked,
`llama.cpp 3cf0325` (the commit pinned by `0.2.4`), `IQ3_S` weights, `mmproj`,
pack and MTP prepared; engine executed (`strata 0.1.48`, families `qwen4exp` and
`qwen35moe`); a real answer obtained from the engine started by the app.

One install detail worth knowing, measured: the engine location is derived from
the pinned version (`data\moe\strata-<version>-<card>`). A `data` folder carried
over from an older install therefore keeps the old engine under the old name, and
the preflight refuses to load it — that is the "engine differs from the pinned
version" message. Install the engine from the Engine screen, or promote the
`0.1.48` package to the pinned location.

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
| Version displayed | `0.18.11.1 Cresius` |
| Git tag | `v0.18.11+rev1` |
| Upstream base | `v0.18.11` |
| MoE engine | Strata `0.2.4` — engine package `0.1.48`, control `0.1.49.2`, five packages per card, SHA-256 verified |
| Embedding encoder | EmbeddingGemma 2 `Q8_0`, 768 dimensions (memory **and** journal) |
| Previous | `0.18.9.1 Cresius`, tag `v0.18.9.1+rev11` |

The private line's naming rule: **upstream's number, as is, followed by our
revision number**. `0.18.11` + revision `1` → `0.18.11.1 Cresius`, tag
`v0.18.11+rev1`. Both names denote the same build.

## Installation

```
ajean update
```

or, from the repository:

1. `git clone https://github.com/Cresius34/ajean`
2. `git checkout v0.18.11+rev1`
3. `go build -o ajean.exe ./cmd/ajean`
