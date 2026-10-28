# Fabriquer-Paquet-Canal.ps1 — fabrique le paquet du patch canal, SANS nom de
# version ni de dossier en dur.
#
# Pourquoi ce script remplace Fabriquer-Paquet-18155.ps1 : celui-ci portait
# « 18155 » dans son nom, son dossier de sortie, le nom du zip et le binaire
# qu'il embarque. A chaque changement de millésime il fallait le recopier, et un
# paquet fabrique avec l'ancien embarque l'ancien binaire en silence.
#
# Ce qui est lu, pas suppose :
#   - le MILLESIME vient du binaire de l'atelier (sa version interne), pas d'un
#     chiffre dans le nom du script ;
#   - les OUTILS du paquet viennent de tools\patch-canal du depot (la source
#     gelée, commitée) — plus jamais d'un dossier build\ non suivi ;
#   - l'installation de DEPOT est trouvee par la presence du binaire a sa racine.
#
# Usage : .\Fabriquer-Paquet-Canal.ps1            (fabrique, verifie, pose le zip)
#         .\Fabriquer-Paquet-Canal.ps1 -Securise  (refuse si le depot est sale)
param(
  [switch]$Securise,
  [switch]$SansCopie
)
$ErrorActionPreference = 'Stop'

# tools\patch-canal -> depot GIT. On remonte depuis SA position : le script suit
# l'installation ou qu'elle soit installee.
#   patch-canal -> tools -> depot (atelier-rXXXX, qui EST le depot git)
# Attention : le depot git et l'atelier sont le MEME dossier. Croire qu'il y a un
# cran de plus au-dessus envoie le build dans data\workspace\build (mesure du
# 2026-10-28 : « aucun binaire d'atelier » alors qu'ils etaient la).
$Outils   = $PSScriptRoot
$Depot    = Split-Path (Split-Path $Outils -Parent) -Parent
$Build    = Join-Path $Depot 'build'
if (-not (Test-Path -LiteralPath $Build)) { throw "dossier de construction absent : $Build" }

# Le depot git est CHERCHE, pas suppose. Mesure du 2026-10-28 : le controle de
# propreté passait a vide, parce que le « depot » calculé a ce moment-la etait
# remonte jusqu'a l'INSTALLATION (C:\ProgramData\Ajean v18.15.3 maj Cresius), qui
# n'est pas un depot git — `git status` y repond « not a git repository », et
# comme la sortie est vide, le controle se croyait reussi. Un depot qui n'existe
# pas n'est pas un depot propre.
$DepotGit = $null
$m = $Outils
for ($n = 0; $n -lt 8 -and $m; $n++) {
  if (Test-Path -LiteralPath (Join-Path $m '.git')) { $DepotGit = $m ; break }
  $parent = Split-Path -Parent $m
  if (-not $parent -or $parent -eq $m) { break }
  $m = $parent
}
if (-not $DepotGit) {
  throw ("aucun depot git au-dessus de $Outils : le controle de propreté de l'outillage " +
         "ne peut pas se faire. Ne pas fabriquer un paquet dont la source n'est pas gelée.")
}
Write-Output ('depot git  : ' + $DepotGit)

# Le depot doit etre propre : un outil corrige a la main et non commité se perd au
# paquet suivant. C'est exactement le mecanisme du « le patch ne remplace pas les
# fichiers » (mesure du 2026-10-28 : trois revisions perdues comme ca).
$statut = & git -C $DepotGit status --porcelain -- tools/patch-canal 2>$null
if ($LASTEXITCODE -ne 0) {
  throw "git status a echoue dans $DepotGit (code $LASTEXITCODE) : le controle de propreté n'a pas eu lieu."
}
if ($statut) {
  if ($Securise) { throw "outillage modifie non commité — commiter avant de fabriquer :" + "`n" + ($statut -join "`n") }
  Write-Output 'ATTENTION : outillage modifie non commité. Il est embarque (fichiers courants),'
  Write-Output '            mais il se perdra si le depot est reinitialise. Penser a commiter.'
  Write-Output ($statut -join "`n")
  Write-Output ''
} else {
  Write-Output 'proprete   : outillage du depot sans modification non commitée'
}

# Le binaire de l'atelier : le plus recent des ajean-<millesime>.exe.
$Exe = Get-ChildItem -LiteralPath $Build -Filter 'ajean-*.exe' -File -EA SilentlyContinue |
       Sort-Object LastWriteTime -Descending | Select-Object -First 1
if (-not $Exe) { throw "aucun binaire d'atelier (build\ajean-*.exe) dans $Build" }

# Le millesime est LU dans le binaire, pas dans son nom.
$t = [IO.File]::ReadAllBytes($Exe.FullName)
$marque = [regex]::Match([Text.Encoding]::ASCII.GetString($t), '0\.1[0-9]\.[0-9]+\.[0-9]+ Cresius')
if (-not $marque.Success) { throw 'millesime introuvable dans le binaire — pause, ne pas livrer a l''aveugle' }
$Millesime = $marque.Value
$Nu = ($Millesime -split ' ')[0]              # 0.18.16.1
# Le « 0. » initial tombe, comme dans la routine de nommage des dossiers : nos
# paquets s'appellent patch-canal-18161, pas patch-canal-018161.
$Chiffres = (($Nu -replace '^0\.', '') -replace '\.', '')
Write-Output ('binaire    : ' + $Exe.Name + '  ' + $Exe.Length + ' o')
Write-Output ('millesime  : ' + $Millesime + '   -> dossier patch-canal-' + $Chiffres)

$Sha = (Get-FileHash -LiteralPath $Exe.FullName -Algorithm SHA256).Hash
Write-Output ('sha256     : ' + $Sha)

# Installation qui RECOIT le zip : celle qui heberge l'atelier. Trouvee par le
# binaire, JAMAIS par un nom de dossier ecrit ici. Ce n'est PAS le depot git : les
# deux s'appelaient « depot » dans le meme script, et le mot designait deux choses
# differentes selon l'endroit — c'est ainsi que le controle de propreté a pu se
# croire reussi sur un dossier qui n'est pas un depot.
. (Join-Path $Outils 'Trouver-Installations.ps1')
$Inst = (Trouver-Principale).Dossier
if (-not $Inst) { throw 'installation principale introuvable (ajean.exe + « maj Cresius » sans suffixe)' }
Write-Output ('zip pose a : ' + $Inst)

$Sortie = Join-Path $Build ('patch-canal-' + $Chiffres)
# Le dossier de sortie est UN ARTEFACT : il est vide et refait, jamais repris.
# Raison mesuree : un fichier oublie la (un vieux `diffs\`, un lanceur d'une
# autre revision) se retrouve embarque dans le zip suivant sans qu'on le voie.
# Garde-fou : on ne supprime que sous $Build — le paquet deploye a la racine d'une
# installation n'est jamais touche ici.
$BuildRes = (Resolve-Path -LiteralPath $Build).Path.TrimEnd('\')
if ($Sortie.StartsWith($BuildRes + '\', [StringComparison]::OrdinalIgnoreCase)) {
  if (Test-Path -LiteralPath $Sortie) {
    Remove-Item -LiteralPath $Sortie -Recurse -Force
    Write-Output 'sortie     : dossier de build repris a zero (rien d''herite)'
  }
} else {
  throw "dossier de sortie hors du dossier de build — refus : $Sortie"
}
New-Item -ItemType Directory -Path $Sortie -Force | Out-Null

# 1. Le binaire, copie BINAIRE (jamais de recodage texte : mesure 19 946 496 ->
#    29 027 591 octets quand un .exe etait traite comme du texte).
Copy-Item -LiteralPath $Exe.FullName -Destination (Join-Path $Sortie 'ajean.exe') -Force

# 2. Les outils : tout ce qui est gele dans le depot, a l'exception des README
#    (ils restent dans le depot) et du fabricant lui-meme.
#    Garde-fou contre les archives : un .zip oublie dans le dossier des outils
#    partait dans le paquet suivant (mesure du 2026-10-28 : paquet de 16 080 876 o
#    au lieu de 8 051 967, avec un vieux patch-canal-018161.zip a l'interieur).
#    On ne copie que des extensions de code et de texte.
$Exclus = @('README.md', 'README-DIAGNOSTIC.md', 'Fabriquer-Paquet-Canal.ps1')
$ExtensionsAdmises = @('.ps1', '.cmd', '.py', '.md', '.txt', '.json')
$poses = @()
foreach ($f in (Get-ChildItem -LiteralPath $Outils -File | Where-Object { $Exclus -notcontains $_.Name })) {
  if ($ExtensionsAdmises -notcontains $f.Extension.ToLower()) {
    throw "fichier non admis dans la source des outils ($Outils) : " + $f.Name +
          " ($($f.Extension)) — une archive ne doit pas vivre a cote des outils"
  }
  Copy-Item -LiteralPath $f.FullName -Destination (Join-Path $Sortie $f.Name) -Force
  $poses += $f.Name
}

# 3. Les lanceurs : ils viennent de la SOURCE GELÉE du dépôt (tools\patch-canal\
#    lanceurs), jamais d'une installation en service. Mesure du 2026-10-28 :
#    l'ancienne fabrique prenait les lanceurs des dossiers vivants par des chemins
#    absolus — sur une autre machine ces dossiers n'existent pas, et le paquet
#    embarquait le verrou de version d'un dossier qu'on ne connaissait pas.
#    Le générateur remis à neuf lit la version dans le binaire du paquet.
$LanceursSrc = Join-Path $Outils 'lanceurs'
if (-not (Test-Path -LiteralPath $LanceursSrc)) {
  throw "source des lanceurs absente ($LanceursSrc) — les gel dans le dépôt, ne pas les régénérer à partir d'un poste"
}
$Lanceurs = Join-Path $Sortie 'lanceurs'
New-Item -ItemType Directory -Path $Lanceurs -Force | Out-Null
foreach ($f in (Get-ChildItem -LiteralPath $LanceursSrc -File)) {
  Copy-Item -LiteralPath $f.FullName -Destination (Join-Path $Lanceurs $f.Name) -Force
}
# Le verrou est appliqué APRÈS la copie : ainsi la source gelée garde sa propre
# version et le paquet part avec la sienne.
& (Join-Path $Lanceurs 'genere-lanceurs.ps1') -VersionLue $Millesime
if ($LASTEXITCODE) { throw "genere-lanceurs a echoue" }
# Relecture : un paquet dont le lanceur verrouille une autre version refuse de
# démarrer chez le destinataire, et l'erreur ressemble à un problème d'installation.
foreach ($f in (Get-ChildItem -LiteralPath $Lanceurs -Filter 'Lancer-*.ps1' -File)) {
  $verrou = [regex]::Match([IO.File]::ReadAllText($f.FullName), "\`$expectedVersion\s*=\s*'([^']*)'").Groups[1].Value
  if ($verrou -ne $Millesime) { throw ('lanceur ' + $f.Name + ' verrouille ' + $verrou + ', attendu ' + $Millesime) }
  Write-Output ('   lanceur : ' + $f.Name + '  verrou=' + $verrou)
}

# 2b. Le manifeste du paquet. Sans lui, le poseur ne peut comparer l'empreinte du
#     binaire à rien : l'empreinte ne doit pas vivre dans le script du poseur (ce
#     serait une seconde vérité à maintenir à chaque build).
$Entrees = @(Get-ChildItem -LiteralPath $Sortie -Recurse -File |
             ForEach-Object { $_.FullName.Substring($Sortie.Length + 1).Replace('\','/') })
$manifeste = @(
  '# PAQUET DU PATCH CANAL — ecrit par Fabriquer-Paquet-Canal.ps1'
  ('fabrique  : ' + (Get-Date -Format 'yyyy-MM-dd HH:mm:ss'))
  ('version   : ' + $Millesime)
  ('octets    : ' + $Exe.Length)
  ('sha256    : ' + $Sha)
  ('source    : ' + $DepotGit)
  # Le commit et l'etiquette viennent du DEPOT GIT. $Depot (calcul de chemin) et
  # $DepotGit (decouverte par .git) coincident ici, mais ne pas les melanger dans
  # le manifeste : un paquet doit dire de quel commit il sort, et ce commit doit
  # pouvoir etre rejoue.
  ('commit    : ' + (& git -C $DepotGit rev-parse --short HEAD 2>$null))
  ('etiquette : ' + (& git -C $DepotGit describe --tags --abbrev=0 2>$null))
  ''
  '# Contenu (' + $Entrees.Count + ' fichiers au moment du manifeste)'
) + ($Entrees | ForEach-Object { '  ' + $_ })
[IO.File]::WriteAllLines((Join-Path $Sortie 'PAQUET.txt'), $manifeste, (New-Object Text.UTF8Encoding($false)))
Write-Output ('manifeste  : PAQUET.txt (' + $manifeste.Count + ' lignes)')

# 4. Le zip, a la racine de chaque installation (la ou l'humain le voit).
# Trouver-Principale / Trouver-Variante rendent un OBJET (Nom, Dossier, Version,
# VerrouillePar…), pas un chemin. Se servir de l'objet comme d'une chaine produit
# « @{Nom=…} » et la copie echoue avec « Lecteur introuvable » (mesure du
# 2026-10-28). On prend .Dossier.
# Le zip porte le NOM DU DOSSIER en prefixe. C'est ce que l'humain attend : il
# dezippe a la racine de l'installation et obtient patch-canal-18161\. Or
# [IO.Compression.ZipFile]::CreateFromDirectory ne met QUE le contenu, sans le
# dossier — dézippé tel quel, il aurait écrasé l'ajean.exe de l'installation (mesuré
# du 2026-10-28 : « Le fichier …\ajean.exe existe déjà »). Les entrées sont donc
# écrites une par une avec le préfixe.
$Zip = Join-Path $Build ('patch-canal-' + $Chiffres + '.zip')
if (Test-Path -LiteralPath $Zip) { Remove-Item -LiteralPath $Zip -Force }
Add-Type -AssemblyName System.IO.Compression.FileSystem
$Prefixe = (Split-Path $Sortie -Leaf) + '/'
$zipArchive = [System.IO.Compression.ZipFile]::Open($Zip, 'Create')
try {
  foreach ($f in (Get-ChildItem -LiteralPath $Sortie -Recurse -File)) {
    $relative = $f.FullName.Substring($Sortie.Length + 1).Replace('\', '/')
    $null = [System.IO.Compression.ZipFileExtensions]::CreateEntryFromFile(
      $zipArchive, $f.FullName, ($Prefixe + $relative), ([System.IO.Compression.CompressionLevel]::Optimal))
  }
} finally { $zipArchive.Dispose() }
$zi = Get-Item -LiteralPath $Zip
$zSha = (Get-FileHash -LiteralPath $Zip -Algorithm SHA256).Hash
$zu = [System.IO.Compression.ZipFile]::OpenRead($Zip)
try {
  $entrees = @($zu.Entries)
  # Controle du prefixe : un paquet dont les entrées ne sont pas rangées sous le
  # dossier du patch se dezippe DIRECTEMENT sur l'installation et ecrase son
  # ajean.exe avant meme que le poseur tourne.
  $malRangees = @($entrees | Where-Object { $_.FullName -notlike ($Prefixe + '*') })
  if ($malRangees.Count) {
    throw ($malRangees.Count + ' entree(s) du zip hors du dossier ' + $Prefixe + ' — ' +
           (($malRangees | Select-Object -First 3 | ForEach-Object { $_.FullName }) -join ', '))
  }
} finally { $zu.Dispose() }

Write-Output ''
Write-Output ('zip        : ' + $zi.Name + '  ' + $zi.Length + ' o  ' + $entrees + ' entrees')
Write-Output ('sha256 zip : ' + $zSha)

if (-not $SansCopie) {
  foreach ($objet in @((Trouver-Principale), (Trouver-Variante))) {
    if (-not $objet) { continue }
    $d = $objet.Dossier
    if (-not $d) { continue }
    Copy-Item -LiteralPath $Zip -Destination (Join-Path $d $zi.Name) -Force
    Write-Output ('   pose      : ' + $d + '\' + $zi.Name)
  }
}
Write-Output ''
Write-Output 'outils du paquet : ' ($poses -join ', ')
