# Outils du patch canal — pourquoi ils vivent ici

Ces scripts sont la **source** des outils posés dans `data\scripts\` des installations.
Copiés depuis `C:\ProgramData\Ajean v18.15.3 maj Cresius\patch-canal-18161\` le
2026-10-28.

## Pourquoi ce dossier existe

Jusqu'ici, ces scripts ne vivaient que dans des dossiers `build\patch-canal-XXXXX`,
**non suivis par git** (`build/` est ignoré). Constat mesuré le 2026-10-28 : trois
révisions du poseur avaient été corrigées à la main dans l'installation, puis
retrouvées **absentes** du dépôt et du paquet suivant. Une correction qui n'est pas
commise se perd au paquet suivant — c'est le mécanisme derrière le
« le patch ne remplace pas les fichiers » signalé par l'utilisateur : le correctif
était réel, mais il ne se transmettait pas.

## Encodage (règle mesurée, ne pas « nettoyer »)

- `.ps1` : **BOM UTF-8 obligatoire** dès qu'ils contiennent des accents, sinon
  illisibles sous PowerShell 5.1.
- `.cmd` : **jamais de BOM**, jamais d'accent (le console Windows les détruit).
- Ne **jamais** recoder un `.exe` en texte (mesure : 19 946 496 → 29 027 591 octets).

## Le correctif du 2026-10-28, en bref

Le poseur cherchait sa cible en exigeant un fichier nommé `Lancer-Cresius.ps1`.
Là où le lanceur s'appelait `Lancer-Cresius-principale.ps1`, la reconnaissance
échouait et le script sortait sur « installation cible introuvable » sans rien poser.
La cible est maintenant **structurelle** : le dossier au-dessus du paquet (qui doit
contenir un `ajean.exe`). Le nom du lanceur posé est lu dans `Lancer-Cresius.cmd`,
pas supposé. Voir l'en-tête de `Appliquer-Patch-Canal.ps1`.
