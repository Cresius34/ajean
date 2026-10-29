# Notes de version — AJEAN `0.18.16.2 Cresius` / étiquette `v0.18.16+rev2`

**Projet dérivé, usage personnel.** Cette version est construite sur le dépôt
`Cresius34/ajean`, fork de [**nathaninline/ajean**](https://github.com/nathaninline/ajean)
— le projet principal, et la seule source des versions publiées d'AJEAN. Les
correctifs décrits ci-dessous forment une **ligne privée** : ils ne sont ni
validés sur d'autres machines, ni garantis transposables. Pour installer AJEAN,
prenez les versions publiées sur
[nathaninline/ajean](https://github.com/nathaninline/ajean/releases/latest).

Cette version **absorbe l'amont `v0.18.16`** — donc aussi
[`v0.18.12`](#amont-v01812--pré-chauffage-du-prompt-paires-doutils-incomplètes),
[`v0.18.13`](#amont-v01813--moteur-exllama-exl3-en-un-clic),
[`v0.18.14`](#amont-v01814--cartes-exllama-qui-déportaient-llamacpp) et
[`v0.18.15`](#amont-v01815--conversation-de-jean-preset-modifié-exllama) — dans la
ligne privée `Cresius`. Le socle est celui de l'amont ; la ligne privée se réduit à ce qu'elle apporte encore.

## En-tête

Le socle amont est `v0.18.16` : l'**installation d'ExLlama depuis l'interface** ne
casse plus sans console (#177), les **binaires CUDA officiels** de llama.cpp arrivent
sous Linux avec carte NVIDIA, et l'**affichage du chat** devient continu. Côté ligne
privée, cette révision 1 sur la nouvelle base porte le **canal Ajean↔Ajean** : le pont
rend un véritable tour d'agent de l'autre côté sous double autorisation, **dit pourquoi
il ne le rend pas**, et l'auteur de chaque message du canal est distingué (un ordre
d'un humain qui vise un poste lie l'IA de ce poste, un ordre d'une IA n'en lie aucune).
Le **journal de Jean** et la **mémoire sémantique** partagent toujours un **seul
encodeur**, EmbeddingGemma 2 — un seul GGUF, un seul moteur, un seul espace vectoriel
sur le poste — et le **moteur MoE Strata** reste proposé par carte, y compris sur AMD (HIP).

## Amont `v0.18.16` — ExLlama depuis l'interface, CUDA Linux, chat plus fluide

Installation d'ExLlama depuis l'interface, binaires CUDA officiels sous Linux et
affichage du chat plus fluide. Trois commits d'amont, 15 fichiers.

- **ExLlama : l'installation échouait depuis l'interface** avec « write /dev/stderr:
  Descripteur non valide » à l'étape de création de l'environnement Python, alors que
  `ajean exl3 install` fonctionnait dans un terminal. Sans console (application lancée
  en fenêtre ou en service), la copie du journal vers le terminal échouait et
  interrompait toute l'installation. Cette copie ignore maintenant l'absence de
  console ; le journal de l'interface reçoit toujours tout. Merci pour le rapport (#177).
- **Linux + carte NVIDIA : llama.cpp est désormais compilé en CUDA** dans les
  binaires pré-construits officiels, au lieu du Vulkan plus lent sur ces cartes.
  L'interface propose le binaire correspondant à la carte détectée.
  (Fait par l'amont ; non testé chez nous : ce poste est AMD.)
- **Chat : rendu continu.** Le texte s'affiche sans clignotement — un élément
  déjà dessiné garde ses nœuds, il n'est plus reconstrifié à chaque fragment —, le
  défileur suit la réponse sans s'accrocher brutalement, le fil ne saute plus si
  l'on scrolle vers le haut pendant l'écriture, chaque mot apparaît en fondu, et
  sur iPhone le fil ne bouge plus tant qu'un doigt touche l'écran, ce qui faisait
  annuler les appuis par Safari.

**Résolution de la fusion de notre côté** : cinq fichiers seulement étaient touchés
des deux côtés (`RELEASE_NOTES.md`, `backend_build.go`, `run.go`, `ui/index.html`,
`ui/src/styles.css`). Dans les deux Go qui comptent, l'amont touche `runStep`/
`lenientWriter` et nous touchons `requireTools` en build portable : des zones
distinctes, fusion automatique, rien à arbitrer. `index.html` n'a **pas** été résolu
à la main — c'est un fichier **généré** (`go:generate` → `tools/assemble-ui`) ; il a
été **régénéré** depuis `ui/src/` fusionné, puis contrôlé par présence des chaînes
propres à chaque côté (`revealState`, `aj-word-in`, `li-wait` d'amont ;
`renderBridge`, `loadPeers`, `bridgeToggle`, `loadMoe`, `moeVerify` de la ligne
Cresius). Dans `styles.css`, les deux côtés ajoutaient en fin de fichier : les deux
blocs sont gardés. `RELEASE_NOTES.md` a gardé notre structure (document de ligne
privée avec l'archive des sections amont) et la section amont a été réécrite ici.

Contrôles sur l'état fusionné : `go vet ./...` sans sortie ; `go test ./internal/ajean
-count=1` = ok (88 s, zéro échec) ; les trois tests de millésime passent.

## Amont `v0.18.15` — conversation de Jean, preset modifié, ExLlama

Corrections de stabilité : conversation du mode Jean, presets modifiés et moteur
ExLlama.

- **Mode Jean : conversation remplacée par une version vieille de plusieurs
  jours.** Si la conversation enregistrée ne pouvait pas être lue au démarrage
  (par exemple juste après une mise à jour, quand l'ancien processus tient encore
  la base), AJEAN repartait d'une conversation vide sans le signaler, puis
  rouvrait une ancienne copie du fil Jean et l'écrivait par-dessus la
  conversation à jour. Le fil Jean enregistré est maintenant toujours préféré à
  cette ancienne copie. Une conversation illisible est mise de côté (copie de
  secours) avant toute réécriture, et la lecture au démarrage est retentée plus
  longtemps. Merci pour le rapport détaillé (#175).
- **Modifier le preset actif restait sans effet.** Après l'enregistrement, le
  moteur continuait à tourner avec l'ancienne configuration, même après un
  redémarrage, jusqu'à passer par un autre preset. Enregistrer une modification
  du preset actif propose maintenant de l'appliquer tout de suite (#176).
- **ExLlama : « chargement… » affiché pendant une réponse.** Pendant la lecture
  d'un long prompt, le moteur ExLlama tarde à répondre aux vérifications d'état,
  et l'interface concluait à tort que le modèle était en cours de chargement. Un
  moteur qui a répondu récemment est maintenant considéré comme occupé, pas en
  chargement.
- **Conseil ExLlama** : une répartition manuelle des cartes trop serrée (champ
  « répartition GPU » du preset) peut remplir une carte à quelques Mo près et
  faire échouer la lecture des longues conversations par manque de mémoire. La
  répartition automatique, avec une réserve de 512 Mo par carte, est plus sûre.

## Amont `v0.18.14` — cartes ExLlama qui déportaient llama.cpp

- **Modèle llama.cpp qui ne démarre plus après un modèle ExLlama** : un preset
  ExLlama enregistrait ses cartes dans le réglage de cartes de la machine,
  conservé d'un modèle à l'autre. En revenant sur un modèle llama.cpp réglé pour
  un autre ordre de cartes, la répartition était inversée et le chargement
  échouait par manque de mémoire. Les cartes d'un preset ExLlama sont rangées à
  part, et le réglage de la machine n'est plus modifié. Un preset ExLlama
  enregistré avec la 0.18.13 est corrigé automatiquement à la bascule. Une
  machine déjà touchée remet son ordre avec `ajean gpu <ordre>`.

## Amont `v0.18.13` — moteur ExLlama (EXL3) en un clic

AJEAN fait tourner les modèles EXL3 avec ExLlamaV3 à côté de llama.cpp :
installation en un clic, mise à jour, réglages dans l'éditeur de modèle.

- **Installation sans prérequis** : une ligne « ExLlama » dans la section Moteur
  installe TabbyAPI et ExLlamaV3 avec leurs dépendances précompilées (PyTorch,
  CUDA 12 ou 13 selon le pilote). Python est téléchargé et géré par AJEAN.
  Environ 4 Go à télécharger, 10 Go sur le disque ; l'installation par défaut
  prend une version validée, dépendances figées.
- **Mise à jour et désinstallation** depuis la même fenêtre (versions installées
  affichées, version en place utilisable jusqu'à la fin de l'installation,
  modèle ExLlama actif relancé). La désinstallation conserve les modèles.
- **Téléchargement de modèles EXL3** par le lien d'une branche Hugging Face ; les
  fichiers arrivent dans `models/exl3`.
- **Choix du moteur par modèle** : l'éditeur propose « ExLlama » à côté des
  moteurs llama.cpp, avec ses réglages propres (contexte, cache, utilisateurs
  simultanés, décodage spéculatif MTP/n-gram/brouillon, cartes utilisées,
  répartition en Go par carte, vision).
- Le moteur sert sur le même port et avec la même clé que llama-server : chat,
  vision, bench, accès distant et endpoint OpenAI fonctionnent sans autre
  réglage. Ligne de commande : `ajean exl3 install|update|download|status|uninstall`.

> **Sur ce poste (RX 9070 XT / RX 6900 XT)** : ExLlama est annoncé pour les
> cartes **NVIDIA** sous Linux et Windows. Le contrôle préalable (`exl3Env`)
> refuse l'installation si `nvidia-smi` est absent ou si le pilote est antérieur
> à 570 — sur une carte AMD, la ligne « ExLlama » reste donc affichée mais
> indiquera « ExLlama demande une carte NVIDIA ». Nos moteurs à nous restent
> llama.cpp et Strata (HIP). Amont non testé sur ce poste.

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

## Ce que cette version ajoute à l'amont `v0.18.15`

- **Fusion sur quatre versions d'amont** : `v0.18.12` → `v0.18.15` absorbées
  d'un coup (17 commits, 37 fichiers). Les deux correctifs d'interface touchés
  par l'amont ont été **fusionnés et non remplacés** : la ligne `allSettled` de
  `loadAll()` garde notre `loadMemoryPreferences()` **et** le `loadExl3()`
  amont ; `saveItem()` garde notre garde-fou de session (`seq === openSeq`,
  timeout 15 min pour un preset) **et** la proposition d'appliquer le preset
  actif modifié (#176). `index.html` n'a pas été résolu à la main : il est
  **régénéré** par `go generate ./internal/ajean` (outil `tools/assemble-ui`)
  depuis les sources `ui/src/`, ce qui efface les 33 zones de conflit du fichier
  généré.
- **Nos deux normalisations de preset cohabitent** avec le correctif `v0.18.14` :
  dans `applyPresetFile`, le bloc amont (`isExl3Config` →
  `CUDA_VISIBLE_DEVICES` → `EXL3_GPUS`) est appliqué **d'abord**, puis notre
  normalisation portable (`prepareParallelPreset`) avec son empreinte `before`.
  Cet ordre est délibéré : remonter `before` ferait réécrire le fichier preset
  d'un ExLlama externe et exposerait une erreur sur `parallelWritablePath`.

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

## Le pont Ajean↔Ajean : deux Ajean du réseau local qui se parlent

**Front entièrement absent de l'amont** — c'est la plus grosse partie de la ligne
privée actuelle (`parallel_bridge.go` +993 lignes, `parallel_peer.go` +858,
`parallel_bridge_agent.go` +526, plus `parallel_config/_engine/_import/_profile/
_memory.go`, la carte d'interface `13b-bridge.js` et quatre fichiers de tests).

Ce n'est **pas** un modèle emprunté ni un partage de charge : **chaque côté garde
son propre cerveau.** Le pont est un canal entre deux IA, pas un modèle prêté.

| Côté | Ce qui est exposé |
| --- | --- |
| **La carte** (interface) | « Pont Ajean ↔ Ajean », qui n'apparaît que si `/api/bridge` répond. Un interrupteur, un port réglable, et **l'adresse à coller chez l'autre Ajean** |
| **L'API** | `/api/bridge` : `GET` = état, `POST {enabled, port}` = réglage. La surface offerte au voisin est **`/v1` seule** — ni l'interface, ni la mémoire, ni les outils |
| **L'outil** | `peer`, quatre actions : `list`, `status`, `ask`, `forget` |

**Ce que le pont transporte, et ce qu'il ne transporte pas.** Une question et une
réponse textuelles. Le voisin **ne voit jamais notre disque** : il répond avec ses
propres outils sur sa propre machine, et il peut refuser. Nos fichiers ne partent
pas, ses fichiers n'arrivent pas.

**Deux règles d'autorité, mesurées puis corrigées :**

- **L'auteur de chaque message est marqué des deux côtés.** Un ordre venu d'un
  humain vise le poste où il est saisi : il lie l'IA de ce poste. Un ordre venu
  d'une IA n'en lie aucune. Le pont qui prête **dit pourquoi il n'a pas
  déclenché son agent** (turn déjà en cours, conversation ouverte, ou demande
  relayée trop de fois) au lieu de se taire.
- **Une action perdue ne doit pas se muer en autre chose.** Mesuré deux fois le
  2026‑10‑28 : `ask` envoyé avec un message de plusieurs lignes, l'`action`
  arrivait vide, et l'outil rendait **la liste des voisins** sans un mot
  d'erreur. Vue d'en face, cela ressemblait à une réponse du voisin — c'était
  notre propre outil qui avait laissé tomber la question. Un `action` absent avec
  un `message` présent est maintenant un `ask`.

**Outillage d'installation** (`tools/patch-canal/`, 26 outils, aucun équivalent
amont) : poseur, diagnoseur, fabricateur de paquet, recherche d'installations. Le
poseur ne reconnaît plus sa cible au nom du lanceur — **il la reconnaît
structurellement** (le dossier au‑dessus du paquet, qui doit contenir un
`ajean.exe`). Mesuré : là où le lanceur s'appelait `Lancer-Cresius-principale.ps1`
et non `Lancer-Cresius.ps1`, la reconnaissance échouait et le poseur sortait sur
« installation cible introuvable » **sans rien poser**. Le nom du lanceur posé est
lu dans `Lancer-Cresius.cmd`, pas supposé.

Deux règles d'encodage à ne pas « nettoyer » : les `.ps1` exigent le **BOM UTF‑8**
dès qu'ils portent des accents (illisibles sous PowerShell 5.1 sinon), les `.cmd`
n'en portent **jamais**. Un `.exe` n'est jamais recodé en texte (mesure :
19 946 496 → 29 027 591 octets).

## La génération d'images : un prototype mesuré, hors dépôt

**À lire avant tout le reste : la génération d'images n'est pas une fonction de ce
build.** Le dépôt **supprime explicitement** le générateur d'images de la
configuration du moteur — `delete(out, "images")` dans `backend_moe_strata.go`,
commenté « aucun générateur d'images étranger n'est installé ni démarré ». Ce qui
suit décrit un **prototype conduit sur une machine, hors dépôt, non paqueté, non
installable par `ajean update`**. Il est consigné ici parce qu'il a produit des
mesurages utiles, pas parce qu'il serait livré.

Ce que le prototype a établi, mesure à l'appui :

- **`sd-cli` compile en HIP sur notre carte** (`gfx1201`). La cause de l'échec
  précédent n'était pas ROCm : c'était notre *toolset* MSVC.
- **Z‑Image Turbo (Q3_K, 4 pas) en 512×512** : 16,1 s poids en VRAM ; **18,2 s en
  `--offload-to-cpu` avec 0 Mo de VRAM**. Le repli `--backend cpu` coûte un
  facteur ~4 (24,2 s contre 5,85 s en GPU à 256 px). **L'offload est la réponse**,
  pas le CPU.
- **Le moteur est déterministe sans amorce** : prompt identique → fichier identique
  au MD5 près. C'est ce qui rend une comparaison d'images fiable, et ce qui a
  permis de prouver qu'un réglage de mémoire ne changeait rien au rendu.
- **Z‑Image ne comprend pas le français.** Test à conditions égales, seule la
  langue change : le prompt français rate **les trois éléments demandés** (canidé
  gris dans des herbes, sans rocher ni mousse) ; le prompt anglais les réussit
  **tous les trois**. Conséquence pratique : tout prompt destiné à ce modèle doit
  être écrit en anglais.
- **La boucle « l'IA écrit le prompt → le modèle image génère → l'IA relit
  l'image et juge » tient de bout en bout** (~20 s par tour). Son verrou n'était
  ni la génération ni la traduction : **c'était le jugement.** Un juge à note
  globale attribuait 10/10 à tout, y compris à une image manifestement ratée, et
  ne rendait même pas de note sur elle. Remplacé par une **liste binaire
  numérotée** (`1 : OUI` / `2 : NON`) dont **le script fait le compte** : la note
  devient déterministe et comparable, et l'image ratée est détectée (5/5 conforme
  sur un banc d'images dont l'issue était connue d'avance).

**Ce qui reste à décider avant d'en faire une fonction** : le choix du modèle et sa
licence, l'installation et la mise à jour du générateur, la place dans
l'interface, et ce que « générer » veut dire quand aucune carte n'est disponible.

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
| Version affichée | `0.18.16.2 Cresius` |
| Étiquette Git | `v0.18.16+rev2` |
| Socle amont | `v0.18.16` |
| Moteur MoE | Strata `0.2.4` — paquet moteur `0.1.48`, contrôle `0.1.49.2`, cinq paquets par carte, SHA-256 vérifiés |
| Encodeur d'embeddings | EmbeddingGemma 2 `Q8_0`, 768 dimensions (mémoire **et** journal) |
| Précédent | `0.18.16.1 Cresius`, étiquette `v0.18.16+rev1` (patch canal) |

La règle de nommage de la ligne privée : **le numéro de l'amont, tel quel, suivi
de notre numéro de révision**. `0.18.16` + révision `2` → `0.18.16.2 Cresius`,
étiquette `v0.18.16+rev2`. Les deux désignent le même build.

## Ce que la révision 2 ajoute sur la base `v0.18.16`

Trois correctifs, chacun avec son test :

- **La réserve VRAM choisie dans l'application est enfin appliquée.** Le champ
  « Réserve VRAM » de l'écran Moteur écrivait dans le preset… que le moteur
  n'écoutait pas : la clé `user_args` du profil gagnait à chaque démarrage. Le
  réglage est maintenant marqué « appliqué par l'application » et propagé dans
  `user_args` (`args_written`), donc l'écran dit vrai. **Le défaut a vécu du
  18/10 au 29/10.** Tests : `TestStrataReserveSurvivesUserArgs`,
  `TestStrataReserveLeftAloneWhenUnset`, `TestStrataVisionOffRemovesFlagFromUserArgs`.
- **Le seuil de compactage se règle en pourcentage de la fenêtre** (`COMPACT_FRAC`),
  plus seulement en jetons absolus. Il était codé en dur à 75 % ; `COMPACT_MAX` ne
  pouvait que l'**abaisser**. Un curseur est apparu dans les réglages. Tests :
  `chat_compact_max_test.go`.
- **L'outil du pont ne peut plus perdre une question en silence.** Une `action`
  absente avec un `message` présent est maintenant un `ask`. Mesuré deux fois :
  l'appel partait avec `ask`, l'action arrivait vide, et l'outil rendait **la liste
  des voisins** sans un mot d'erreur — vu d'en face, cela ressemblait à une réponse
  du voisin. Tests : `parallel_peer_ask_fallback_test.go`,
  `tool_args_repair_guillemets_test.go`.

## Installation

```
ajean update
```

ou, depuis le dépôt :

1. `git clone https://github.com/Cresius34/ajean`
2. `git checkout v0.18.16+rev1`
3. `go build -o ajean.exe ./cmd/ajean`

---

# English

# Release notes — AJEAN `0.18.16.2 Cresius` / tag `v0.18.16+rev2`

**Derived project, personal use.** This build comes from the `Cresius34/ajean`
repository, a fork of [**nathaninline/ajean**](https://github.com/nathaninline/ajean)
— the upstream project, and the only source of published AJEAN releases. The
fixes below form a **private patch line**: they are not validated on other
machines and are not guaranteed transposable. To install AJEAN, use the
releases published at
[nathaninline/ajean](https://github.com/nathaninline/ajean/releases/latest).

This build **absorbs upstream `v0.18.16`** — and therefore
[`v0.18.12`](#upstream-v01812--prompt-prewarm-incomplete-tool-pairs),
[`v0.18.13`](#upstream-v01813--exllama-engine-exl3-in-one-click),
[`v0.18.14`](#upstream-v01814--exllama-cards-that-broke-llamacpp) and
[`v0.18.15`](#upstream-v01815--jean-conversation-modified-preset-exllama) — into the
`Cresius` patch line. The base is upstream's; the private line is reduced to what it still adds.

## Headline

The upstream base is `v0.18.16`: installing **ExLlama from the interface** no longer
breaks when there is no console (#177), official llama.cpp **CUDA binaries** arrive on
Linux with an NVIDIA card, and **chat rendering** becomes continuous. On the private
side, this revision 1 on the new base carries the **Ajean↔Ajean channel**: the bridge can
run a real agent turn on the other machine under double authorisation, **says why it did
not**, and every channel message carries its author — an order from a human aimed at a
machine binds that machine's AI, an order from an AI binds none. **Jean's journal** and
**semantic memory** still share a **single encoder**, EmbeddingGemma 2 — one GGUF, one
engine, one vector space on the machine — and the **Strata MoE engine** is still offered
per card, AMD (HIP) included.

## Upstream `v0.18.16` — ExLlama from the interface, CUDA on Linux, smoother chat

- **ExLlama: installation from the interface failed** with "write /dev/stderr: invalid
  descriptor" at the Python environment step, while `ajean exl3 install` worked in a
  terminal. Without a console (app launched in a window or as a service), copying the log
  to the terminal failed and aborted the whole install. That copy now ignores the missing
  console; the interface log still receives everything. Thanks for the report (#177).
- **Linux + NVIDIA card: llama.cpp is now built with CUDA** in the official prebuilt
  binaries instead of Vulkan, which is slower on those cards. (Done upstream; not tested
  here — this machine is AMD.)
- **Chat: continuous rendering.** Text no longer flickers (already-drawn elements keep
  their nodes instead of being re-tokenised per fragment), the scroller follows the answer
  smoothly, scrolling up during generation no longer jumps back down, each word fades in,
  and on iPhone the thread stays put while a finger touches the screen.

**Our side of the merge**: only five files were touched on both sides
(`RELEASE_NOTES.md`, `backend_build.go`, `run.go`, `ui/index.html`, `ui/src/styles.css`).
In the two Go files that matter, upstream changed `runStep`/`lenientWriter` while we
changed `requireTools` in the portable build — different regions, clean auto-merge.
`index.html` was **not** merged by hand: it is a **generated** file (`go:generate` →
`tools/assemble-ui`), so it was **regenerated** from the merged `ui/src/`, then verified
by the presence of strings specific to each side. In `styles.css` both sides appended at
the end of the file: both blocks kept.

Checks on the merged state: `go vet ./...` silent; `go test ./internal/ajean -count=1` =
ok (88 s, no failures); the three version-flavour tests pass.

## Upstream `v0.18.15` — Jean conversation, modified preset, ExLlama

Stability fixes: Jean mode conversation, modified presets and the ExLlama engine.

- **Jean mode: conversation replaced by a days-old copy.** If the stored
  conversation could not be read at startup (for instance right after an update,
  while the old process still holds the database), AJEAN started from an empty
  conversation without saying so, then reopened an old copy of the Jean thread
  and wrote it over the up-to-date one. The stored Jean thread is now always
  preferred to that old copy. An unreadable conversation is set aside (backup
  copy) before anything is rewritten, and the startup read is retried for longer.
  Thanks for the detailed report (#175).
- **Editing the active preset had no effect.** After saving, the engine kept
  running the previous configuration, even across a restart, until another preset
  was selected. Saving a change to the active preset now offers to apply it
  right away (#176).
- **ExLlama: "loading…" shown while answering.** While reading a long prompt,
  the ExLlama engine is slow to answer health checks, and the interface wrongly
  concluded the model was still loading. An engine that answered recently is now
  treated as busy, not loading.
- **ExLlama advice**: a too-tight manual card split (the preset's "GPU split"
  field) can fill a card to within a few MB and make long conversations fail to
  load for lack of memory. The automatic split, with a 512 MB reserve per card,
  is safer.

## Upstream `v0.18.14` — ExLlama cards that broke llama.cpp

- **A llama.cpp model that would no longer start after an ExLlama model**: an
  ExLlama preset stored its cards in the machine-level card setting, which is
  carried from one model to the next. Going back to a llama.cpp model tuned for a
  different card order inverted the split and the load failed for lack of memory.
  An ExLlama preset's cards are now kept apart and the machine setting is no
  longer modified. An ExLlama preset saved with 0.18.13 is fixed automatically on
  switch. A machine already affected restores its order with `ajean gpu <order>`.

## Upstream `v0.18.13` — ExLlama engine (EXL3) in one click

AJEAN can now run EXL3 models with ExLlamaV3 alongside llama.cpp: one-click
install, update, and settings inside the model editor.

- **No prerequisites**: an "ExLlama" row in the Engine section installs TabbyAPI
  and ExLlamaV3 with precompiled dependencies (PyTorch, CUDA 12 or 13 depending
  on the driver). Python is downloaded and managed by AJEAN. About 4 GB to
  download, 10 GB on disk; the default install takes a validated version with
  pinned dependencies.
- **Update and uninstall** from the same window (installed versions shown, the
  running version stays usable until the install finishes, an active ExLlama
  model is restarted). Uninstalling keeps the models.
- **EXL3 model downloads** from a Hugging Face branch link; files land in
  `models/exl3`.
- **Engine chosen per model**: the editor offers "ExLlama" next to the llama.cpp
  engines, with its own settings (context, cache, simultaneous users, speculative
  decoding MTP/n-gram/draft, cards used, per-card GB split, vision).
- The engine serves on the same port and key as llama-server: chat, vision,
  bench, remote access and the OpenAI endpoint work with no extra setting.
  Command line: `ajean exl3 install|update|download|status|uninstall`.

> **On this machine (RX 9070 XT / RX 6900 XT)**: ExLlama is advertised for
> **NVIDIA** cards on Linux and Windows. The preflight check (`exl3Env`) refuses
> the install when `nvidia-smi` is missing or the driver predates 570 — on an AMD
> card the "ExLlama" row stays visible but reports "ExLlama requires an NVIDIA
> card". Our own engines remain llama.cpp and Strata (HIP). Upstream code, not
> tested on this machine.

## Upstream `v0.18.12` — prompt prewarm, incomplete tool pairs

- **Prompt prewarm**: as soon as a new conversation is opened, the engine reads
  the instructions and the tool descriptions in advance, so the first message no
  longer pays for that read. Measured with the MoE engine: first token in 0.3 s
  instead of 4.3 s in Project mode, 0.3 s instead of 5.9 s in Jean mode. Prewarm
  only starts when the open conversation is empty, and it is on by default
  (Settings ▸ General). Contribution by youtsuhodev (#163).
- **Conversation blocked after a stop**: stopping during a run of tool calls left
  unanswered calls in the history, and the engine then rejected every new message
  (error 400). Missing answers are now completed, including in conversations
  already blocked by an earlier version. Contributions by oh-codes (#171, #164).


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

## What this build adds on top of upstream `v0.18.15`

- **Fusion over four upstream releases**: `v0.18.12` → `v0.18.15` absorbed at
  once (17 commits, 37 files). The two interface files touched upstream were
  **merged, not replaced**: `loadAll()`'s `allSettled` line keeps our
  `loadMemoryPreferences()` **and** upstream's `loadExl3()`; `saveItem()` keeps
  our session guard (`seq === openSeq`, 15-minute timeout for a preset) **and**
  the offer to apply a modified active preset (#176). `index.html` was not
  resolved by hand: it is **regenerated** with `go generate ./internal/ajean`
  (the `tools/assemble-ui` tool) from the `ui/src/` sources, which clears the 33
  conflict regions of the generated file.
- **Our two preset normalizations coexist** with the `v0.18.14` fix: in
  `applyPresetFile`, upstream's block (`isExl3Config` →
  `CUDA_VISIBLE_DEVICES` → `EXL3_GPUS`) runs **first**, then our portable
  normalization (`prepareParallelPreset`) with its `before` fingerprint. That
  order is deliberate — hoisting `before` would rewrite an external ExLlama
  preset's file and expose a `parallelWritablePath` error.

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

## The Ajean↔Ajean bridge: two Ajean instances on one local network talking

**Entirely absent upstream** — this is the largest part of the current private
line (`parallel_bridge.go` +993 lines, `parallel_peer.go` +858,
`parallel_bridge_agent.go` +526, plus `parallel_config/_engine/_import/_profile/
_memory.go`, the `13b-bridge.js` interface card and four test files).

This is **not** a borrowed model and not shared load: **each side keeps its own
brain.** The bridge is a channel between two AIs, not a lent model.

| Side | What is exposed |
| --- | --- |
| **The card** (interface) | "Ajean ↔ Ajean bridge", which only appears if `/api/bridge` answers. A switch, an adjustable port, and **the address to paste on the other Ajean** |
| **The API** | `/api/bridge`: `GET` = state, `POST {enabled, port}` = settings. The surface offered to the neighbour is **`/v1` only** — no interface, no memory, no tools |
| **The tool** | `peer`, four actions: `list`, `status`, `ask`, `forget` |

**What the bridge carries, and what it does not.** A question and a text answer.
The neighbour **never sees our disk**: it answers with its own tools on its own
machine, and it may refuse. Our files do not travel; its files do not arrive.

**Two authority rules, measured then fixed:**

- **The author of each message is marked on both sides.** An order from a human
  targets the machine where it was typed: it binds that machine's AI. An order
  from an AI binds none. A bridge that declines **says why it did not start its
  agent** (turn already running, conversation open, or a request relayed too many
  times) instead of staying silent.
- **A lost action must not turn into something else.** Measured twice on
  2026‑10‑28: `ask` sent with a multi-line message, the `action` arrived empty,
  and the tool returned **the list of neighbours** without a word of error. Seen
  from the other side, that looked like the neighbour's answer — it was our own
  tool dropping the question. A missing `action` with a `message` present is now
  an `ask`.

**Install tooling** (`tools/patch-canal/`, 26 tools, no upstream equivalent):
installer, diagnoser, package builder, install finder. The installer no longer
recognises its target by launcher name — **it recognises it structurally** (the
folder above the package, which must contain an `ajean.exe`). Measured: where the
launcher was named `Lancer-Cresius-principale.ps1` rather than
`Lancer-Cresius.ps1`, recognition failed and the script exited on "target install
not found" **without installing anything**. The installed launcher's name is read
from `Lancer-Cresius.cmd`, not assumed.

Two encoding rules not to "clean up": `.ps1` files require a **UTF‑8 BOM** once
they carry accents (unreadable under PowerShell 5.1 otherwise), `.cmd` files never
carry one. An `.exe` is never re-encoded as text (measured: 19,946,496 →
29,027,591 bytes).

## Image generation: a measured prototype, outside the repository

**Read this first: image generation is not a feature of this build.** The
repository **explicitly removes** the image generator from the engine
configuration — `delete(out, "images")` in `backend_moe_strata.go`, commented
"no unrelated image generator is installed or started". What follows describes a
**prototype run on one machine, outside the repository, not packaged, not
installable via `ajean update`**. It is recorded here because it produced useful
measurements, not because it ships.

What the prototype established, measurement attached:

- **`sd-cli` compiles under HIP on our card** (`gfx1201`). The cause of the
  earlier failure was not ROCm: it was our MSVC *toolset*.
- **Z‑Image Turbo (Q3_K, 4 steps) at 512×512**: 16.1 s weights in VRAM; **18.2 s
  with `--offload-to-cpu` at 0 MB VRAM**. The `--backend cpu` fallback costs a
  factor of ~4 (24.2 s vs 5.85 s on GPU at 256 px). **Offload is the answer**, not
  the CPU.
- **The engine is deterministic without a seed**: same prompt → same file, MD5 for
  MD5. That makes image comparison reliable, and it is what proved a memory
  setting changed nothing in the rendering.
- **Z‑Image does not understand French.** Equal conditions, language the only
  variable: the French prompt misses **all three requested elements** (grey canid
  in grass, no rock, no moss); the English prompt hits **all three**. Practical
  consequence: every prompt for this model must be written in English.
- **The loop "the AI writes the prompt → the image model generates → the AI reads
  the image back and judges" holds end to end** (~20 s per turn). Its bottleneck
  was neither generation nor translation: **it was the judgement.** A judge giving
  an overall score awarded 10/10 to everything, including a plainly failed image,
  and did not even return a score for it. Replaced by a **numbered binary list**
  (`1: YES` / `2: NO`) whose **count the script does**: the score becomes
  deterministic and comparable, and the failed image is caught (5/5 correct on a
  bench of images whose outcome was known in advance).

**Still to decide before turning this into a feature**: model choice and its
licence, installing and updating the generator, its place in the interface, and
what "generate" means when no GPU is available.

## What revision 2 adds on top of the `v0.18.16` base

Three fixes, each with its test:

- **The VRAM reserve chosen in the application is finally applied.** The "VRAM
  reserve" field on the Engine screen wrote into the preset… which the engine did
  not listen to: the `user_args` key of the profile won at every start. The
  setting is now marked "applied by the application" and propagated into
  `user_args` (`args_written`), so the screen tells the truth. **The defect lived
  from 10‑18 to 10‑29.** Tests: `TestStrataReserveSurvivesUserArgs`,
  `TestStrataReserveLeftAloneWhenUnset`, `TestStrataVisionOffRemovesFlagFromUserArgs`.
- **The compaction threshold is set as a percentage of the window**
  (`COMPACT_FRAC`), no longer only in absolute tokens. It was hard-coded at 75 %;
  `COMPACT_MAX` could only **lower** it. A slider appeared in the settings.
  Tests: `chat_compact_max_test.go`.
- **The bridge tool can no longer lose a question silently.** A missing `action`
  with a `message` present is now an `ask`. Measured twice: the call went out as
  `ask`, the action arrived empty, and the tool returned **the neighbour list**
  without a word of error — seen from the other side it looked like the
  neighbour's answer. Tests: `parallel_peer_ask_fallback_test.go`,
  `tool_args_repair_guillemets_test.go`.

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
| Version displayed | `0.18.16.2 Cresius` |
| Git tag | `v0.18.16+rev2` |
| Upstream base | `v0.18.16` |
| MoE engine | Strata `0.2.4` — engine package `0.1.48`, control `0.1.49.2`, five packages per card, SHA-256 verified |
| Embedding encoder | EmbeddingGemma 2 `Q8_0`, 768 dimensions (memory **and** journal) |
| Previous | `0.18.16.1 Cresius`, tag `v0.18.16+rev1` (bridge patch) |

The private line's naming rule: **upstream's number, as is, followed by our
revision number**. `0.18.16` + revision `2` → `0.18.16.2 Cresius`, tag
`v0.18.16+rev2`. Both names denote the same build.

## Installation

```
ajean update
```

or, from the repository:

1. `git clone https://github.com/Cresius34/ajean`
2. `git checkout v0.18.16+rev1`
3. `go build -o ajean.exe ./cmd/ajean`
