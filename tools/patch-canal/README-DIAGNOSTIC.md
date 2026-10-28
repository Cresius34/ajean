# Diagnostic d'installation — lecture seule, sans droits

Ajouté le 2026-10-28, parce que le point sur l'installation d'une machine voisine
ne pouvait pas être fait autrement. La mesure qui l'impose : chez la voisine,
18190 est OUVERT et protégé (401), mais 18080 (moteur), 18081 (encodeur) et 18090
(interface) ne répondent pas depuis le réseau. Le pont n'échange que du texte. Un
voisin dont le tour d'agent est éteint n'a donc AUCUN MOYEN de lire son propre
disque : il ne peut que répondre « non vérifié » — et c'est la bonne réponse.

## Ce qui remplace quoi (le 2026-10-28)

Trois fichiers faisaient la même chose à des étages différents, et deux portaient
la sonde **en une ligne dans le `.cmd`** : le premier guillemet mal échappé
cassait tout, et l'erreur remontait dix lignes plus loin sans dire quelle sonde
était morte. `Diagnostic-court.cmd`, `Diagnostic-installation.cmd` et
`Diagnostic-Bekka.ps1` sont retirés. Il reste :

- `Diagnostic-Installation.ps1` — la sonde, huit blocs, toute la logique.
- `Diagnostic-Canal.cmd` — un lanceur de 20 lignes pour le double-clic : il
  vérifie que le `.ps1` est à côté de lui, l'appelle, et ne contient aucune
  logique.

## Les huit blocs, et ce qu'ils concluent

| Bloc | Ce qu'il lit | Ce qu'on en conclut |
|---|---|---|
| 1) installations | trouvées **par la présence du binaire**, jamais par nom de dossier | cohérence du nom avec la version ; un dossier sans `ajean.exe` est un reste |
| 2) ports en écoute | le **chemin** du processus qui tient 18090/18190/18080/18081, avec sa version **chargée en mémoire** | quel dossier tourne vraiment, pas seulement ce qui est posé |
| 3) encodeur | `dims=512` ou `dims=768` | **512 = EmbeddingGemma 2 ; 768 = l'ancien Gemma** (le patch n'est pas passé, ou le moteur n'a pas redémarré) |
| 4) moteur | `GET http://127.0.0.1:18080/health` | Strata en service, ou rien sur 18080 |
| 5) pont | `GET http://127.0.0.1:18090/api/bridge` + `/api/peers` | la carte « Pont Ajean ↔ Ajean » existe-t-elle, et qui est déclaré comme voisin |
| 6) paquet | cherché **par motif** `patch-canal-*`, empreinte comparée à son `PAQUET.txt` | intègre, divergent, ou **ILLISIBLE** = quarantaine antimalware |
| 7) outils | présence des outils du canal dans `data\scripts` | ce que la pose a vraiment laissé sur le disque |
| 8) antimalware | `Get-MpThreatDetection` filtré sur nos fichiers | un de nos binaires est-il mangé |

## Règles mesurées, à ne pas « simplifier »

- **La sonde du moteur est `/health`, pas `/v1/models`.** Celui-ci exige une clé
  et rend 401 alors que Strata est chargé. Un port qui répond 401 n'est pas un
  port mort. Premier tirage de ces scripts : faux négatif « moteur NON en
  service » sur une machine où Strata était chargé.
- **`/api/bridge` ne rend PAS les voisins.** Lu dans `parallel_bridge.go`
  (`parallelBridgeStatus`, lignes 676-702) : la liste des pairs vient de
  `/api/peers`. Annoncer « 0 voisin » sur une clé absente ferait câbler en double.
- L'encodeur se lit sur `http://127.0.0.1:18090/api/embeddings` ; la marque de
  l'encodeur est `config.dimensions` : **512 = EmbeddingGemma 2, 768 = l'ancien
  Gemma**.
- **Un binaire en quarantaine est ILLISIBLE** : `Get-FileHash` échoue. La sonde le
  dit (« LECTURE REFUSEE ») au lieu d'annoncer « absent ». Mesuré le 2026-10-28 :
  `Trojan:Win32/Bearfoos.B!ml` sur notre propre `ajean.exe` (faux positif de
  heuristique, `!ml`), sur le binaire posé **et** sur celui du paquet.
- Les exclusions Defender sont illisibles sans administrateur : la sonde le
  signale, elle ne déduit pas.
- **Aucun sha ni millésime en dur.** L'ancienne sonde attendait `9FF0415C…` et
  nommait `patch-canal-18161` : au build suivant elle annonçait « paquet absent »
  sur un paquet parfaitement présent. On cherche par motif et on compare à ce que
  le paquet dit de lui-même dans son manifeste.
- Dans un `[ordered]@{}`, une clé numérique lève « index hors des limites de la
  plage » : employer `[string]$port`.

Encodage : `.cmd` **sans BOM** et sans accents ; `.ps1` **avec BOM UTF-8**. Un
`.cmd` avec BOM s'affiche avec des caractères parasites et la première commande
est mal lue. Un `.ps1` sans BOM UTF-8 se lit en ANSI : les accents deviennent du
mojibake, et c'est ce qui est arrivé à ce fichier.
