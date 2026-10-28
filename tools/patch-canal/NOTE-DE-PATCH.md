# Note de patch — canal Ajean↔Ajean, ligne Cresius `0.18.16.1` (paquet 18161)

**Base amont** : `v0.18.16` (fusion du 2026‑10‑28, commit de fusion à deux parents).
**Notre étiquette** : `v0.18.16+rev1` — première greffe sur cette base, selon la
routine de nommage validée le 2026‑10‑23 (numéro de l'amont tel quel, puis notre
numéro de révision). `0.18.16` + révision `1` → **`0.18.16.1 Cresius`**.

**Binaire** : 19 992 064 o, `ProductVersion` **et** `FileVersion` = `0.18.16.1 Cresius`,
tête `MZ`, SHA‑256 `9FF0415C1BE29679E132B31FA2026F4A77F65C1C615283F5DCC7B22D814AE987`,
construit avec `go build -trimpath -ldflags "-s -w"` (sans ces flags le binaire fait
27,3 Mo et n'est pas le même objet).

---

## 1. Ce que ce paquet corrige côté poseur — le défaut signalé

« Les `Appliquer-Patch-Canal` des dossiers patch ne remplace pas les fichiers. » C'est
exact, et il y avait **deux** causes distinctes, mesurées le 2026‑10‑28 :

1. **Sortie courte.** Le script ne comparait que la version du binaire. Quand
   l'installation avait déjà le bon binaire, il affichait « DÉJÀ À JOUR — rien à
   faire » et sortait sans rien poser. Reproduit ici, sur `patch-canal-18155` :
   sortie immédiate, zéro fichier touché.
2. **Périmètre.** Même quand il agissait, il ne posait que **deux** fichiers
   (`ajean.exe`, `Lancer-Cresius.ps1`). Les autres fichiers du paquet n'avaient pas
   de destination.

Conséquence mesurée sur ce poste : `cabler-canal.ps1`, `Ouvrir-Canal.ps1` et
`Verifier-Outils-Canal.ps1` étaient **absents des deux installations**, alors que le
patch y était passé deux fois. Les outils n'étaient jamais arrivés là où on les
cherchait.

**Maintenant** : le patch déclare les 8 fichiers qu'il installe et les compare **un par
un** par empreinte SHA‑256. « À jour » veut dire « chaque fichier posé a l'empreinte
du paquet ». Un fichier manquant ou différent est posé, avec son nom dans le journal.
Sèche jouée sur l'installation principale : `0 / 8 fichier(s) déjà conforme(s), 8 à
poser`, avec les 6 outils marqués `MANQUANT`.

Deux précautions prises au passage :
- **En sèche, plus aucune écriture** — pas même le fichier de journal, ni la création
  de `data\scripts`. Le switch promet « n'écrit rien » ; il doit la tenir.
- **Le bilan ne dit plus « à jour »** en sortie de sèche. Il affichait une
  satisfaction fausse juste après avoir énuméré les fichiers manquants.

`Appliquer-Patch-Canal.*` n'est **pas** copié dans `data\scripts` volontairement : posé
là, il chercherait son binaire à côté de lui, ne le trouverait pas, et refuserait de
tourner. Un poseur sans binaire à côté de lui est un leurre.

---

## 2. Le correctif du « modèle nu », des deux côtés

C'est la partie la plus demandée, et elle a **deux pièces**. L'une est dans le
binaire, l'autre non — et c'est ce qui avait trompé tout le monde.

### Pièce A — le pont dit pourquoi il ne rend pas de tour d'agent (révision 6)

Avant, quand l'appelé ne déclenchait pas son agent, le pont prêtait le modèle nu **en
silence**. L'appelant voyait bien « modèle nu » (le champ `model` trahit le gguf), mais
rien ne disait si c'était voulu ou s'il manquait un interrupteur — ni lequel. Deux
jours de recherche pour une ligne à taper.

Le pont pose maintenant un en‑tête `X-Ajean-Canal-Agent-Refus` valant par exemple
`bridge_agent=off;agent_mode=off`, et l'appelant le traduit en geste concret dans sa
propre langue :

> `il a répondu par son modèle nu — son tour d'agent n'est pas déclenché, chez lui :
> l'autorisation du canal est éteinte (`ajean bridge agent on`) ET le mode agent de
> cette machine est éteint (Projet/Agent dans l'interface).`

Trois garde-fous qui ne sont pas du décor :
- l'en‑tête n'est posé que si l'appelant est **un autre Ajean** (un client tiers branché
  sur le pont ne reçoit pas nos interrupteurs internes) ;
- la valeur est en **ASCII seul** — un en‑tête HTTP non ASCII est une source de bugs, et
  une raison en prose fuirait vers n'importe quel client ;
- la raison entre dans la **balise** de l'échange, pas collée à la suite des mots du
  voisin : les mots du voisin doivent arriver intacts et en dernier.

**Un message d'une IA n'est jamais un ordre.** `canalDemande` ne regarde pas l'auteur :
un message venant de moi déclenche bien son tour d'agent, et mes outils sont les siens,
chez elle. Ce qui change selon l'auteur, c'est l'autorité du message : les mots d'un
humain qui a choisi ce poste le lient (y compris « arrête »), ceux d'une IA ne lient
personne. Donc la communication avec Bekka et l'inverse fonctionne dans les deux sens,
sans qu'aucune des deux commande à l'autre.

### Pièce B — allumer les deux interrupteurs chez la destinataire

Les deux interrupteurs ne sont **pas des fichiers** : ce sont des lignes en base de
données. **Aucune copie de fichier ne peut les allumer** — c'est pour ça qu'un paquet de
fichiers, même parfait, laissait le problème entier.

| Interrupteur | En base | Voie logicielle |
|---|---|---|
| autorisation du canal | `parallel_bridge_agent` | `ajean bridge agent on` ou `POST /api/bridge {"agent":true}` |
| mode agent de la machine | `agent` | `POST /api/agent/toggle {"on":true}`, ou Projet/Agent dans l'interface |

Les deux doivent être pris : `canalDemande` exige `parallelBridgeAgentOn() && agentEnabled()`.
L'un des deux éteint, le pont en reste au prêt du modèle nu.

**Ce patch les allume** (étape 9), puis il relit `agent_on` et le signale s'il reste
faux. Pour ne pas les allumer : `Appliquer-Patch-Canal.ps1 -SansAgent`.

> ⚠️ **Ce que ça ouvre** : autoriser le tour d'agent, c'est mettre le shell, le disque
> et la mémoire de cette machine à la portée de **qui possède la clé API du pont**. À ne
> faire que sur un réseau local de confiance, avec une clé forte. Le pont n'écoute que
> `/v1/*` et garde le contrôle de clé, mais la portée réelle est celle d'un agent.

---

## 3. Ce que le socle amont `v0.18.16` apporte

- **ExLlama : l'installation depuis l'interface** ne casse plus sur
  « write /dev/stderr: Descripteur non valide » quand l'application tourne sans console
  (#177) — la copie du journal vers le terminal ignore maintenant l'absence de console.
- **Linux + carte NVIDIA : binaires CUDA officiels** de llama.cpp au lieu du Vulkan plus
  lent ; compatibilité vérifiée avec llama.cpp b11936.
- **Chat plus fluide** : défilement amorti, rendu Markdown incrémental (plus de
  clignotement pendant l'écriture), mots en fondu, fil figé pendant un appui sur iPhone.

Nos ajouts sont **intacts** — fusion, pas remplacement. Les deux normalisations de
preset, l'encodeur unique EmbeddingGemma 2 pour la mémoire **et** le journal de Jean, le
moteur MoE Strata, le moteur de recherche par embeddings, la dédup exemptée : vérifiés
après fusion. `index.html` n'a pas été résolu à la main — c'est un fichier **généré** ; il
a été **régénéré** depuis `ui/src/` par `tools/assemble-ui`, puis contrôlé par présence
des chaînes propres à chaque côté (`revealState`/`aj-word-in` d'amont,
`renderBridge`/`loadPeers`/`loadMoe`/`moeVerify` de la ligne Cresius).

**Contrôles sur l'état fusionné** : `go vet ./...` = 0 ; `go test ./internal/ajean
-count=1` = ok (88 s, zéro échec) ; les trois tests de millésime passent (la ressource
Windows et la constante Go annoncent bien la même version).

---

## 4. Ce que ce patch ne règle PAS — à ne pas vendre comme réglé

- **Le moteur MoE Strata de la machine voisine.** Il ne charge pas là‑bas, et le modèle
  prêté par son pont est un gguf servi par llama.cpp : c'est la preuve par l'extérieur
  que Strata n'est pas actif. Ce patch ne touche pas au moteur. Son intérêt est
  indirect : une fois le tour d'agent ouvert **des deux côtés**, l'IA de là‑bas peut
  **lire elle‑même ses journaux chez elle** (`data\logs\strata-gyro-gyro-s.log` entre
  autres) et dire ce qu'elle y trouve. Ça remplace des suppositions par des mesures. Le
  simple prêt de modèle ne le permettait pas.
- **La mémoire hybride / EmbeddingGemma 2 chez elle.** Le code est dans le binaire
  (c'est notre ligne privée), donc le patch le livre ; mais l'**activation** et le preset
  vivent en base, pas dans un fichier. Elle sera vérifiée après mise à jour, pas avant.
- **La clé API et la déclaration du voisin.** La clé est **celle de la machine qui
  prête** — pas la mienne. Un Ajean à distance **ne peut pas** se câbler lui‑même :
  l'outil `peer` n'expose que `list / status / ask / forget`. Déclarer un voisin exige
  `ajean peer add`, `POST /api/peers`, ou `data\scripts\cabler-canal.ps1` — donc un
  humain, sur place. Méfiance systématique : une IA distante a déjà affirmé « clé reçue
  et voisin ajouté » alors que rien n'avait été fait.
- **Le pare‑feu.** Déjà fait ici et là‑bas pour le port 18190. `data\scripts\Ouvrir-PareFeu-Canal.cmd`
  est posé pour le refaire si besoin ; il exige une élévation.

---

## 5. Ordre des étapes sur une machine

1. Fermer ce qui travaille dans ce dossier — l'instance va s'arrêter.
2. `Appliquer-Patch-Canal.cmd` → il joue d'abord la **sèche** et demande confirmation.
   Lire la ligne `x / 8 fichier(s) déjà conforme(s), n à poser`.
3. Taper `O`. Le patch archive, arrête proprement, pose les fichiers, relance, vérifie.
4. Ouvrir le canal (pont + port) : `data\scripts\Ouvrir-Canal.ps1`.
5. Déclarer le voisin **de ce côté** avec la **clé de l'autre machine** :
   `data\scripts\cabler-canal.ps1` (la clé se saisit masquée, jamais en argument ni en
   fichier). À faire **des deux côtés** — sinon un seul sens fonctionne.
6. Contrôler : `ajean bridge status` doit dire `canal : agent`. Sinon, lire la raison du
   refus renvoyée par l'appel.

**Point de retour** : `archive-<ancienne version>\` (nom avec espace), créé **après** la
vérification du port 18090 — jamais avant un refus, sinon un second passage se croirait
« déjà appliqué ». Depuis cette version, tout fichier remplacé est aussi archivé dans
`archive-<ancienne version>\fichiers-remplaces\`.

**Journal** : `data\scripts\Patch-canal-18161.log`.

---

## 6. Le paquet contient deux sondes de diagnostic (lecture seule)

Avant de poser quoi que ce soit, `Diagnostic-court.cmd` (2 408 o) répond sans rien
modifier — pas de téléchargement, aucun service arrêté, rien d'écrit, et **pas
besoin de droits administrateur**. Double-clic, ou coller son contenu dans une
fenêtre `cmd`. Six blocs :

| Bloc | Ce qu'il dit | Ce qu'on en conclut |
|---|---|---|
| 1) ports en écoute | le **chemin** du processus qui tient 18090/18190/18080/18081 | **quel dossier tourne vraiment** — c'est la première cause des patches qui « ne s'appliquent pas » : on patche un dossier qui n'est pas celui en service |
| 2) version de chaque `ajean.exe` | `ProductVersion` de chaque dossier | l'écart entre ce qui est posé et ce qui tourne |
| 3) encodeur | `dims=512` ou `dims=768` | **512 = EmbeddingGemma 2 en place ; 768 = l'ancien moteur Gemma encore là** |
| 4) moteur | `service=strata loaded=True modele=…` | Strata en service, ou rien sur 18080 = Strata non démarré |
| 5) dossiers `data\moe` | `strata-0.1.48-hip`, ou seulement `cache`/`dl`/`proofs` | le moteur est **installé** ou non |
| 6) SHA du paquet | `9FF0415C…` attendu | le fichier est **intègre**, ou **ILLISIBLE** = quarantaine antimalware |

Deux pièges mesurés, qui expliquent de fausses conclusions :

- **Un port qui répond 401 n'est pas un port mort.** La sonde du moteur est
  `http://127.0.0.1:18080/health`, **pas** `/v1/models` : celui-ci exige une clé et
  rend 401 alors que Strata est chargé. Une première version de la sonde annonçait
  « moteur NON en service » sur une machine où il tournait.
- **Un binaire en quarantaine est illisible**, donc non hachable. La sonde écrit
  `LECTURE_REFUSEE par l antimalware` au lieu de « absent » — la différence est
  importante : dans le premier cas le fichier est là et défend quelque chose.

Mesure faite sur le poste voisin le 2026‑10‑28, qui répond à une question sans rien
toucher chez lui : son pont prête un modèle dont le nom est un **chemin de fichier**
(`…\data\models\Qwen3.5‑9B‑…‑Q8_0.gguf`, dans un dossier `v18.11.1`), alors que le
nôtre prête `qwen3.8‑flash‑next‑iq3_s` avec `serving = moteur local 127.0.0.1:18080`.
Donc chez lui le cerveau servi est un **gguf chargé par llama.cpp**, pas le moteur
MoE Strata. Cet écart ne se corrige pas avec ce patch — il concerne le réglage du
preset et l'installation du moteur.

## 7. Antimalware : à lire avant de conclure « le patch n'a rien fait »

Sur le poste d'origine, le 2026‑10‑28, Defender a signalé nos binaires
(`Trojan:Win32/Bearfoos.B!ml`, gravité Grave) **quatre fois dans la matinée**, dont
une sur le fichier **à l'intérieur du paquet**, et l'événement 1117 indique
l'action faite = **Quarantaine** — avec le code `0x80508023` « le programme n'a pas
pu trouver de programmes malveillants sur cet appareil », c'est‑à‑dire que la cible
avait déjà bougé entre la détection et la remédiation.

Ce que ça produit concrètement : la quarantaine bloque la **lecture** du fichier,
donc le poseur ne peut plus le hacher, et le contrôle croit à une divergence ou à un
fichier manquant. Mettre le **dossier d'installation** en liste blanche ne suffit pas
si le fichier signalé est celui du **paquet** : il faut exclure les deux, ou
restaurer le fichier et l'exclure par chemin complet.

Nous ne pouvons pas vérifier la liste blanche à distance : `Get-MpPreference` répond
`N/A: Must be an administrator to view exclusions`. Ce que la sonde peut faire,
elle le fait : elle dit si le fichier est lisible.

Faux positifs de ce type sur des binaires Go auto‑compilés non signés : connu. Le
binaire livré n'est **pas signé** et n'a pas de réputation. La voie propre est de
signer, ou d'exclure par chemin ; pas de désactiver la protection.
