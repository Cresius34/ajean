# R6 — Mémoire + MoE Windows portable

Base : [PR mémoire #153](https://github.com/nathaninline/ajean/pull/153), commit
`f4059912458a2deff459d3fbbc519e27933d869f`. Phase 2 demandée explicitement par
Julien le 16 octobre 2026. Version du prototype : `0.18.1-embedding-test.6`.

## Essayer sur le poste Windows

1. Quitter l’ancien TEST depuis son icône près de l’horloge. Extraire le paquet
   R6 complet dans un nouveau dossier, puis lancer `Lancer-Test.cmd`. Vérifier
   **TEST mémoire · R6** sous le logo. Aucun service système n’est installé.
2. Dans la gestion du moteur llama.cpp, ouvrir **Qwen3.8 Flash Next**. Choisir
   **AMD · HIP** pour la Radeon AI PRO R9700, ou **NVIDIA · CUDA** pour une NVIDIA.
   Le pilote graphique doit déjà fonctionner. CUDA 13 est le paquet initial ;
   CUDA 12 est une option de compatibilité, notamment pour les anciennes cartes.
   L’installeur amont contrôle la génération et le pilote de la carte choisie.
3. Choisir la famille et la quantification. Pour un premier essai sur la R9700
   avec 128 Go de RAM : Swift 1.5 / IQ3_XXS, contexte 86 016, KV int8, MTP 3,
   réserve de 4 096 Mio. Ce sont des valeurs de départ, pas un réglage mesuré sur
   cette machine. Le Qwen3.8-Flash-Next original est aussi proposé. Les poids
   MoE sont distincts du Swift 27B utilisé auparavant.
4. Confirmer l’installation : environ 68–84 Go de poids selon le choix, plus les
   environnements et les packs. Le mode mmap peut demander davantage de disque.
   Le dialogue affiche une estimation ; garder une marge supplémentaire. Le
   moteur est préparé, un preset est créé, puis son chargement commence.
5. Une fois le modèle chargé, rouvrir sa fenêtre et cliquer **Tester réponse et
   appel d’outil**. Le test vérifie une réponse exacte puis un appel OpenAI avec
   arguments JSON, sans exécuter cet outil. Un rapport daté est écrit dans
   `data/moe/proofs/`, avec les succès ou l’échec observé. Le bouton Benchmark
   existant reste disponible pour mesurer le moteur sur la cible.

Libérer d’abord la VRAM prise par un autre modèle de conversation. Le TEST
n’arrête jamais l’installation habituelle pour récupérer sa mémoire. La réserve
VRAM est un budget pour le moteur, pas une garantie contre un débordement WDDM
ou contre les besoins variables d’Unreal. Commencer le test sans charge Unreal,
puis mesurer la cohabitation et ajuster contexte/réserve.

Les réglages mémoire dans Général et les presets restent disponibles. Le bouton
**Réglages mémoire de ce preset** ouvre le même éditeur mémoire pour un preset
MoE, en conservant ses clés de moteur. Les encodeurs mémoire et la vision du
modèle de conversation ont des paramètres indépendants.

## Ce qui fait apparaître le moteur dans la liste

La liste des moteurs MoE proposés vient des presets, pas de la carte graphique :
`ListPresets()` lit `<AJEAN_HOME>/presets/*.env`, et un moteur n'est annoncé
« installé » que si l'un de ces fichiers porte `ENGINE=moe` avec
`MOE_RUNTIME=strata-windows` ; une famille n'affiche son modèle que si le nom
du preset correspond. Retirer le seul `FLASH NEXT SWIFT 1.5 IQ3_XXS · HIP.env`
fait disparaître l'entrée du modèle. Le portique matériel reste visible sans
preset : `supported` / `visible` ne dépendent pas des `.env`.

Un preset de moteur llama.cpp classique (clé `BIN`, pas de clé `ENGINE`) n'est
pas un preset MoE et ne compte pas dans cette liste.

## Piège du build parallèle : `AJEAN_HOME` et port 18090

`cmd/ajean-memory-test` (build de test, `BuildFlavor = "embedding-test"`)
**ignore un `AJEAN_HOME` hérité** : `initParallelProfile()` réécrit
`AJEAN_HOME = <dossier de l'exe>\data` (`internal/ajean/parallel_profile.go:45`).
Son dossier de presets est imposé par l'endroit où vit l'exe — déplacer l'exe
change la liste des moteurs, et un exe posé dans un dossier sans
`data\presets\*.env` n'affiche aucun moteur MoE, quel que soit le matériel.

Il écoute de plus **18090 en dur** (`parallel_profile.go:127`) et refuse de
démarrer si ce port est pris :

    port d'essai 18090 occupé ; aucun processus arrêté : listen tcp 127.0.0.1:18090

`app`, `web` et le double-clic sont détournés vers `cmdParallelApp()`. Pour
servir sur un port libre ou mesurer une configuration isolée, utiliser le
binaire officiel `cmd/ajean` : il respecte `AJEAN_HOME` et accepte un port.

## Intégration retenue

[Strata 0.2.4](https://github.com/Niko1221/Strata/releases/tag/v0.2.4),
sources `96ca4383c7b05a8a3e6064c65d9e9977d4b3bb59` (paquet moteur `0.1.48`, controle `0.1.49.2`). Les PR Ajean
[#144](https://github.com/nathaninline/ajean/pull/144), #139 et #137 ont été
examinées : pilotage natif, environnement Python cohérent, journaux et annulation
accessibles depuis l’application Windows. La PR #57 a servi de référence pour
la détection Windows. Leurs essais NVIDIA sont des résultats rapportés par leurs
auteurs ; ils ne valident pas notre intégration AMD.

La R6 réutilise les paquets précompilés HIP, CUDA 13 et CUDA 12 de Strata. Il
n’y a pas de compilation C++ locale, de MSVC ou de SDK graphique installé dans
Windows. La détection AMD utilise CIM/registre en lecture, puis la sonde HIP du
paquet vérifie le périphérique avant les gros téléchargements. L’iGPU du 7950X3D
n’est pas retenu à la place de la R9700.

Tout le nécessaire est contenu sous `data/moe/` :

| Élément | Emplacement |
|---|---|
| Sources, moteur, Python embarqué HIP | `strata-0.1.48-hip/` |
| Sources, moteur, Python CUDA | `strata-0.1.48-cuda13/` ou `-cuda12/` |
| Poids et packs partagés par ces backends | `data/` |
| Archives contrôlées et caches | `dl/`, `cache/` |
| Journaux d’installation et de calcul | `logs/` |
| Configuration de lancement régénérée | `run/` |
| Rapports du test explicite | `proofs/` |

Les ports sont 18090 pour l’interface, 18080 pour la conversation, 18081 pour la
mémoire. Le serveur est limité à la boucle locale. Seuls les processus créés par
le TEST sont contrôlés ; aucun PID étranger ou service existant n’est adopté.

L’installeur appelle directement le setup amont avec Python Windows embarqué
3.12.10 et `--yes --no-start --no-browser`. Il ne lance pas START-HERE.bat et ne
copie pas un Python issu d’un venv. L’adaptation impose des dossiers locaux,
refuse la découverte/déplacement d’autres installations, le repli vers une
release `latest`, les révisions Hugging Face non figées et la compilation de
secours. Les recommandations de calcul du backend amont sont conservées ; les
optimisations Linux/CUDA mesurées par Ajean ne sont pas imposées à HIP.

Les trois chemins MOE_* des presets sont relatifs. Les chemins structurels des
JSON sont stockés avec `${AJEAN_HOME}/` et développés à chaque démarrage. Les
traversées de dossiers et les liens sortant du prototype sont refusés. Le dossier
portable peut être déplacé puis relancé ; un changement de machine peut exiger
un nouveau choix du backend/pilote/GPU.

## Versions et intégrité

`internal/ajean/moe_windows_manifest.json` contient les URL, tailles et SHA-256
figés. Les archives sources, Python, bootstrap pip et les trois moteurs ont été
téléchargés et leur SHA-256 contrôlé pendant la préparation. Les ZIP moteur ont
aussi passé leur contrôle CRC. Les poids restent téléchargés à partir des
révisions du setup amont ; ils ne sont pas inclus dans le paquet R6.

Strata est sous licence MIT ; les licences des DLL sont conservées dans ses
archives. Swift utilise sa propre licence de modèle (Open License 1.0 indiquée
par le setup amont). Consulter la licence de la variante choisie avant de la
redistribuer. Le prototype ne redistribue aucun poids MoE.

## Limites et validation

Une carte par modèle dans ce prototype. Aucun mélange AMD/NVIDIA ni réglage de
carte d’appoint n’est proposé. Le paquet HIP Windows fournit le moteur texte et
la sonde, **aucun encodeur vision** : la vision de conversation est désactivée.
Les paquets NVIDIA fournissent la vision GPU ; elle doit être choisie pendant
la préparation initiale pour que son projecteur soit téléchargé. La mémoire
multimodale reste indépendante et conserve les limites du prototype mémoire.

Les tests automatisés couvrent les paramètres, les configurations et chemins,
la conservation mémoire, l’intégrité/isolement du setup, et le refus de faux
succès du test de réponse/appel d’outil. Le navigateur contrôle l’interface
réellement servie ; seule l’API matériel Windows est simulée, sans installation
ni inférence. Les enregistrements mémoire passent par le vrai serveur Go.
Les résultats exacts sont dans `VALIDATION.txt` et `PREUVES.json` du paquet.

**Aucune exécution native Windows, inférence AMD/NVIDIA ou mesure de RAM/VRAM
avec Unreal n’a été effectuée ici.** Le paquet est compilé et prêt à tester sur
la cible ; il ne constitue pas une validation du fonctionnement GPU.

## Contribution GitHub

Le paquet GitHub fournit les fichiers ajoutés/modifiés et le patch relatif au
commit de la PR #153, ainsi qu’un patch complet relatif à Ajean 0.18.1
`e7b024adda191dd6f64746a454b8714629a83435`. Choisir le patch correspondant à sa
base ; ne pas appliquer les deux. Les sources complètes incluent l’entrée TEST,
ses ressources et les tests. Aucun push ni nouvelle PR n’est effectué.

Vérification développement : `CI=1 go test ./...`, `go vet ./...`,
`python -m unittest discover -s tools/moe-test`. Pour contrôler le contrat avec
le setup figé : définir `AJEAN_STRATA_SOURCE` vers son checkout avant ce dernier
test. L’interface se régénère avec `go generate ./internal/ajean` ; la compilation
Windows utilise `cmd/ajean-memory-test`, sans changer les ressources de
`cmd/ajean`. Les manifests et le code Python/PowerShell sont embarqués dans le
binaire, ils ne doivent pas être oubliés dans une contribution.
