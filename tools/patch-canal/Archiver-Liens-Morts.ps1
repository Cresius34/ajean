# Archiver-Liens-Morts.ps1 — range les outils de data\scripts selon ce qu'ils visent.
#
# Pourquoi archiver et non corriger : ces outils appartiennent a une campagne JOUEE
# (construire la 18.4, basculer la 18.11.1, jouer le patch 18155). Corriger leur
# chemin les ferait mentir autrement — un outil de la campagne 18.4 qui vise 18.15.3
# n'est plus l'outil de la campagne 18.4. Et les laisser en place a un prix mesure :
# `Fabriquer-Paquet-18155.ps1` embarque le binaire 18155 en silence, donc un paquet
# fabrique avec lui livre l'ancienne revision sans le dire. C'est le mecanisme
# exact du « le patch ne remplace pas les fichiers ».
#
# L'archive est la regle de rangement que Verifier-Chemin-En-Dur.ps1 applique deja :
# il ne scanne pas data\scripts\archive\, sinon il crie 40 fois et on l'eteint.
#
# Rien n'est supprime. Deux passages, dans cet ordre, pour etre IDEMPOTENT :
#   A. REINTEGRATION : un fichier d'archive qui n'est pas une piece de campagne et
#      qui cite un dossier vivant revient a sa place. Le premier tirage a deplace
#      Verifier-Chemin-En-Dur.ps1 (le garde-fou pose par le patch) parce que son
#      COMMENTAIRE cite un dossier mort — l'archiver cassait l'outil qui sert
#      justement a compter les liens en dur.
#   B. ARCHIVAGE, deux raisons, dans cet ordre :
#      B1. le NOM porte un millésime qui n'est pas celui du paquet en place : c'est
#          une piece de campagne (Generer-Lanceurs-18155, Jouer-Patch-18155-Cresius,
#          Maj-Variante-6900XT-18151…). Ces-la citent parfois un dossier VIVANT,
#          donc la seule raison « chemins morts » ne les attrapait pas.
#      B2. il ne cite AUCUN dossier vivant.
#
# Le jugement porte sur le NOM DE DOSSIER d'installation, pas sur le chemin entier.
# Premiere version : elle comparait la capture complete, donc
# « Ajean v18.15.3 maj Cresius\data\workspace\…» etait juge MORT alors que le
# dossier est vivant — six outils encore utiles sont partis a l'archive.
$ErrorActionPreference = 'Stop'

$Racine = 'C:\ProgramData'
$Inst = $null
$m = $PSScriptRoot
for ($n = 0; $n -lt 8 -and $m; $n++) {
  if (Test-Path -LiteralPath (Join-Path $m 'ajean.exe')) { $Inst = $m ; break }
  $p = Split-Path -Parent $m
  if (-not $p -or $p -eq $m) { break }
  $m = $p
}
if (-not $Inst) { throw "installation introuvable au-dessus de $PSScriptRoot (ajean.exe attendu a sa racine)" }
$Scripts = Join-Path $Inst 'data\scripts'
$Archive = Join-Path $Scripts 'archive'
New-Item -ItemType Directory -Force -Path $Archive | Out-Null

# Dossiers vivants : trouves par la presence du binaire, jamais par un nom ecrit ici.
$vivants = @(Get-ChildItem $Racine -Directory -Filter 'Ajean*' -EA SilentlyContinue |
             Where-Object { Test-Path -LiteralPath (Join-Path $_.FullName 'ajean.exe') } |
             ForEach-Object { $_.Name })
Write-Output ('installation traitee : ' + $Inst)
Write-Output ('dossiers vivants     : ' + ($vivants -join ' | '))

# Le millésime du paquet CANAL EN PLACE, lu dans le nom du dossier du paquet.
$PaquetCourant = ''
foreach ($d in $vivants) {
  foreach ($q in (Get-ChildItem -LiteralPath (Join-Path $Racine $d) -Directory -Filter 'patch-canal-*' -EA SilentlyContinue)) {
    $chiffres = ([regex]::Match($q.Name, 'patch-canal-(\d+)')).Groups[1].Value
    if ($chiffres -and $chiffres -gt $PaquetCourant) { $PaquetCourant = $chiffres }
  }
}
Write-Output ('paquet canal en place : ' + $(if ($PaquetCourant) { $PaquetCourant } else { 'AUCUN' }))

# Le nom d'un dossier d'installation dans un chemin : jusqu'au premier separateur.
# Les citations en commentaire sont comptees aussi, volontairement : dans le doute
# on NE DEPLACE PAS.
$rxNom = [regex]('Ajean v18\.[0-9][0-9.]* maj Cresius[^\\' + [char]39 + [char]34 + [char]96 + ']{0,30}')

function Noms-Cites([string]$t) {
  $bruts = @(foreach ($x in $rxNom.Matches($t)) {
      $v = $x.Value.Trim().TrimEnd([char]39, [char]34, [char]96, ',')
      # Le suffixe materiel fait partie du nom : « … maj Cresius RX6900XT » est un
      # dossier DIFFERENT de « … maj Cresius ». On le garde.
      if ($v) { $v }
    })
  @($bruts | Select-Object -Unique)
}

function Cite-Vivant([string]$t) {
  $n = @(Noms-Cites $t)
  foreach ($v in $vivants) {
    foreach ($c in $n) {
      if ($c -eq $v) { return $true }
      if ($c.StartsWith($v) -and ($c.Length -eq $v.Length -or $c[$v.Length] -eq ' ')) { return $true }
    }
  }
  return $false
}

function Millesime-du-nom([string]$nomFichier) {
  $g = [regex]::Matches($nomFichier, '(?<!\d)(18\d{3})(?!\d)') | ForEach-Object { $_.Groups[1].Value } | Sort-Object -Unique
  if (-not $g.Count) { return '' }
  if ($PaquetCourant -and @($g | Where-Object { $_ -eq $PaquetCourant }).Count) { return '' }
  return ($g -join '/')
}

# INTANGIBLES : un outil installe par le patch canal en cours ne s'archive pas, quel
# que soit ce que son texte contient. La liste est LUE dans le poseur du paquet (son
# tableau $OutilsInstalles, via Outils-du-Poseur), pas ecrite ici : elle suit le
# paquet. Le premier tirage de ce script a archive Verifier-Chemin-En-Dur.ps1, le
# garde-fou lui-meme, parce que son commentaire cite un dossier mort.
# Les decouvreurs d'installations et la lecture du tableau d'outils du poseur
# viennent de Trouver-Installations.ps1. Les DEUX copies sont essayees : celle de
# l'installation (data\scripts) et celle, gelée, qui vit a cote de ce script dans
# le depot. Un dot-source sur un fichier absent avec ErrorActionPreference=Stop
# interrompt tout le script : on teste avant d'appeler.
$srcTrouver = @((Join-Path $Scripts 'Trouver-Installations.ps1'),
                (Join-Path $PSScriptRoot 'Trouver-Installations.ps1')) |
              Where-Object { Test-Path -LiteralPath $_ -PathType Leaf } | Select-Object -First 1
if (-not $srcTrouver) { throw 'Trouver-Installations.ps1 introuvable : les decouvreurs sont obligatoires' }
. $srcTrouver
Write-Output ('decouvreurs charges depuis : ' + $srcTrouver)
$intangibles = @()
foreach ($d in $vivants) {
  $dos = Join-Path $Racine $d
  foreach ($q in (Get-ChildItem -LiteralPath $dos -Directory -Filter 'patch-canal-*' -EA SilentlyContinue)) {
    $intangibles += @(Outils-du-Poseur -Paquet $q.FullName)
    $intangibles += @(Get-ChildItem -LiteralPath $q.FullName -File | ForEach-Object { $_.Name })
  }
}
$intangibles = @($intangibles | Where-Object { $_ } | Sort-Object -Unique)
Write-Output ('outils du patch en place, proteges : ' + $intangibles.Count)

# ---- A. reintegration -------------------------------------------------------
$reintegres = @()
foreach ($f in (Get-ChildItem -LiteralPath $Archive -File -EA SilentlyContinue |
                Where-Object { $_.Extension -in '.ps1', '.cmd', '.py' })) {
  $nom = ($f.Name -replace '--[0-9]+(\.[^.]+)$', '$1')
  if ($intangibles -contains $nom) { continue }
  if (Millesime-du-nom $nom) { continue }             # piece de campagne : elle reste archives
  $t = ''
  try { $t = [IO.File]::ReadAllText($f.FullName, [Text.Encoding]::UTF8) } catch { continue }
  if (-not (Cite-Vivant $t)) { continue }
  $dst = Join-Path $Scripts $nom
  if (Test-Path -LiteralPath $dst) {
    Write-Output ('   (!) ' + $f.Name + ' : sa place est prise par ' + $nom + ' — laisse a l archive')
    continue
  }
  Move-Item -LiteralPath $f.FullName -Destination $dst
  if ((Test-Path -LiteralPath $dst) -and -not (Test-Path -LiteralPath $f.FullName)) {
    $reintegres += ($nom + $(if ($nom -ne $f.Name) { ('  (etait ' + $f.Name + ')') } else { '' }))
  }
}
Write-Output ('REINTEGRES de l archive : ' + $reintegres.Count)
foreach ($n in $reintegres) { Write-Output ('   ' + $n) }

# ---- B. archivage ----------------------------------------------------------
$deplaces = @()
$laisses  = @()
$refuses  = @()
foreach ($f in (Get-ChildItem -LiteralPath $Scripts -File |
                Where-Object { $_.Extension -in '.ps1', '.cmd', '.py' })) {
  if ($intangibles -contains $f.Name) { continue }
  if ($f.Name -eq 'Archiver-Liens-Morts.ps1') { continue }   # ce script lui-meme
  $t = ''
  try { $t = [IO.File]::ReadAllText($f.FullName, [Text.Encoding]::UTF8) } catch { $refuses += $f.Name ; continue }

  $raison = ''
  $mil = Millesime-du-nom $f.Name
  if ($mil) {
    # B1 : le nom nomme une revision qui n'est plus celle en place. La fabrique et
    # le poseur canoniques (Fabriquer-Paquet-Canal.ps1, Appliquer-Patch-Canal.ps1)
    # lisent le millésime dans le binaire : un outil qui le porte dans son NOM est
    # l'outil d'une campagne terminee.
    $raison = 'nom fige sur la revision ' + $mil
  } else {
    $cites = @(Noms-Cites $t)
    if (-not $cites.Count) { continue }             # aucun chemin en dur : rien a juger
    if (-not (Cite-Vivant $t)) { $raison = 'aucun dossier vivant cite' }
  }
  if (-not $raison) { $laisses += $f.Name ; continue }

  $dst = Join-Path $Archive $f.Name
  if (Test-Path -LiteralPath $dst) {
    $sansExt = [IO.Path]::GetFileNameWithoutExtension($f.Name)
    $ext = [IO.Path]::GetExtension($f.Name)
    $n = 2
    while (Test-Path -LiteralPath $dst) { $dst = Join-Path $Archive ($sansExt + '--' + $n + $ext) ; $n++ }
  }
  Move-Item -LiteralPath $f.FullName -Destination $dst
  if ((Test-Path -LiteralPath $dst) -and -not (Test-Path -LiteralPath $f.FullName)) {
    $deplaces += [pscustomobject]@{ Nom = $f.Name ; Vers = Split-Path $dst -Leaf ;
                                    Raison = $raison ; Cites = ((Noms-Cites $t) -join ' | ') }
  }
}

Write-Output ''
Write-Output ('ARCHIVES : ' + $deplaces.Count)
foreach ($d in $deplaces) {
  Write-Output ('   ' + $d.Nom + $(if ($d.Vers -ne $d.Nom) { ('  -> ' + $d.Vers) } else { '' }) +
                '   [' + $d.Raison + ']')
  if ($d.Cites) { Write-Output ('        dossiers cites : ' + $d.Cites) }
}
Write-Output ''
Write-Output ('LAISSES SUR PLACE : ' + $laisses.Count)
foreach ($n in $laisses) { Write-Output ('   ' + $n) }
if ($refuses.Count) {
  Write-Output ''
  Write-Output ('LECTURE REFUSEE, non touches : ' + ($refuses -join ', '))
}
Write-Output ''
Write-Output ('data\scripts : ' + @(Get-ChildItem -LiteralPath $Scripts -File).Count +
              ' actif(s), ' + @(Get-ChildItem -LiteralPath $Archive -File).Count + ' dans archive\')
Write-Output 'Controle a rejouer : .\Verifier-Chemin-En-Dur.ps1 (il ne doit plus crier sur les actifs).'
