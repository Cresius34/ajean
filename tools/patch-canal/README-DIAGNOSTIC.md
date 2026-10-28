Diagnostic d'installation, lecture seule : quel dossier tourne, EmbeddingGemma 2 (dimensions), moteur Strata.

Ajoutés le 2026-10-28 parce que le point sur l'installation d'une machine voisine
ne pouvait pas être fait autrement. Mesure qui l'impose : sur le poste voisin,
18190 est OUVERT et protégé (401), mais 18080 (moteur), 18081 (encodeur) et 18090
(interface) ne répondent pas depuis le réseau. Le pont n'échange que du texte.
Un voisin dont le tour d'agent est éteint n'a donc AUCUN MOYEN de lire son propre
disque : il ne peut que répondre « non vérifié » — et c'est la bonne réponse.

Trois fichiers, du plus court au plus complet :

- Diagnostic-court.cmd — 2 408 o, à coller tel quel dans une fenêtre cmd.
  Sans droits administrateur. Répond aux six questions qui suffisent à un point.
- Diagnostic-installation.cmd — le même en plus large (outils du canal posés,
  journaux Defender, cycle de vie du moteur).
- Diagnostic-Bekka.ps1 — sortie JSON pour lecture par un programme.

Règles mesurées, à ne pas « simplifier » :

- La sonde du moteur est http://127.0.0.1:18080/health, PAS /v1/models :
  /v1/models rend 401 (clé exigée) alors que le moteur tourne. Un port qui répond
  401 n'est pas un port mort. Premier tirage de ces scripts : faux négatif
  « moteur NON en service » sur une machine où Strata était chargé.
- /health de Strata rend service, loaded, model, max_context, api_key.
- L'encodeur se lit sur http://127.0.0.1:18090/api/embeddings ; la marque de
  l'encodeur est config.dimensions : 512 = EmbeddingGemma 2, 768 = l'ancien Gemma.
- Un binaire en quarantaine antimalware est ILLISIBLE : Get-FileHash échoue. Ces
  scripts le disent (« LECTURE_REFUSEE ») au lieu d'annoncer « absent ».
- Les exclusions Defender sont illisibles sans administrateur : le script le
  signale, il ne déduit pas.
- Dans un [ordered]@{}, une clé numérique lève « index hors des limites de la
  plage » : employer [string]$port.

Encodage : .cmd sans BOM et sans accents ; .ps1 avec BOM UTF‑8.
