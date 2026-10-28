# Trouver-Installations.ps1 — LA decouverte a la place du chemin ecrit en dur.
# Ecrit le 2026-10-28, parce que nos versions sont PORTABLES et se posent
# n'importe ou, sur n'importe quelle machine : un chemin cablé ment des que le
# dossier est renomme ou deplace (mesure du jour : 5880 occurrences du nom du
# dossier dans data\ajean.db, 14 de mes propres scripts qui le citent, et le
# pont de l'app tableau de bord qui se taisait parce que son chemin avait change).
#
# Ce que fait ce fichier : il ne definit AUCUN chemin. Il trouve.
# Il se dot-source :  . "$PSScriptRoot\Trouver-Installations.ps1"
#
# Piege applique (vu le 27/10) : [string]::Replace / Ordinal, jamais -replace
# (insensible a la casse + le motif est une regex).
$ErrorActionPreference = 'Continue'

function Trouver-Installations {
  param(
    [string]$Racine = 'C:\ProgramData',
    [string[]]$Motif = 'Ajean*'
  )
  $res = @()
  foreach ($mot in $Motif) {
    foreach ($d in (Get-ChildItem -LiteralPath $Racine -Directory -Filter $mot -EA SilentlyContinue)) {
      $exe = Join-Path $d.FullName 'ajean.exe'
      if (-not (Test-Path -LiteralPath $exe -PathType Leaf)) {
        # Pas de binaire a la racine : ce n'est pas une installation, c'est un
        # reste (dossier de donnees d'une version archivee, etc.). On le signale
        # sans le compter comme installation en service.
        $res += [pscustomobject]@{
          Nom = $d.Name ; Dossier = $d.FullName ; Binaire = $null
          Version = '' ; EstCresius = $false ; VerrouillePar = @()
          SansBinaire = $true
        }
        continue
      }
      $v = ''
      try { $v = [Diagnostics.FileVersionInfo]::GetVersionInfo($exe).ProductVersion } catch { $v = 'ILLISIBLE' }
      $tenus = @()
      foreach ($p in (Get-CimInstance Win32_Process -EA SilentlyContinue)) {
        if (-not $p.ExecutablePath) { continue }
        # Le separateur de dossier est OBLIGATOIRE dans la comparaison : sans lui,
        # C:\ProgramData\ajean est un PRECICE de C:\ProgramData\ajean v18.15.3 maj
        # Cresius\… et chaque installation se croit verrouillee par les processus
        # de sa voisine (mesure du 2026-10-28).
        if ($p.ExecutablePath.ToLower().StartsWith($d.FullName.ToLower() + '\')) { $tenus += $p.ProcessId }
      }
      $res += [pscustomobject]@{
        Nom = $d.Name ; Dossier = $d.FullName ; Binaire = $exe
        Version = $v ; EstCresius = ($v -match 'Cresius')
        VerrouillePar = $tenus ; SansBinaire = $false
      }
    }
  }
  return $res
}

# La variante, s'il y en a une : ce qui se distingue par un suffixe apres « maj
# Cresius » (chez nous « RX6900XT », chez un autre « RTX », « portable »…). On ne
# devine pas le materiel : on lit le suffixe du dossier.
function Trouver-Variante {
  param([object[]]$Installations = (Trouver-Installations))
  return @($Installations | Where-Object { -not $_.SansBinaire -and $_.EstCresius -and $_.Nom -match 'maj\s+Cresius\s+\S' })
}

# L'installation principale : une ligne Cresius SANS suffixe.
function Trouver-Principale {
  param([object[]]$Installations = (Trouver-Installations))
  return @($Installations | Where-Object { -not $_.SansBinaire -and $_.EstCresius -and $_.Nom -match 'maj\s+Cresius$' })
}

# Le nom que la routine de nommage impose pour une version donnee :
# « Ajean v<version> maj Cresius ». Le nom du dossier ne sert plus qu'a l'humain ;
# aucun outil ne doit le supposer.
# Constat d'usage (dossiers reels de ce poste) : la version du binaire s'ecrit
# « 0.18.15.3 Cresius » mais le dossier s'appelle « Ajean v18.15.3 maj Cresius » :
# le « 0. » initial tombe. On le retire ici pour que le nom attendu ressemble a
# ce que l'humain a deja sous les yeux, sinon chaque controle hurlerait a l'ecart.
function Nom-Attendu {
  param([string]$Version, [string]$Suffixe = '')
  $v = $Version -replace '\s+Cresius$', ''
  $v = $v -replace '^0\.', ''
  $base = 'Ajean v' + $v + ' maj Cresius'
  if ($Suffixe) { $base += ' ' + $Suffixe }
  return $base
}

# Le paquet du canal, quel que soit son millésime : on cherche un MOTIF, pas un
# nom. (Un nom en dur ment des que la revision change — c'etait deja le cas.)
function Trouver-Paquet-Canal {
  param([string]$Dossier)
  if (-not $Dossier -or -not (Test-Path -LiteralPath $Dossier)) { return $null }
  $p = @(Get-ChildItem -LiteralPath $Dossier -Directory -Filter 'patch-canal-*' -EA SilentlyContinue |
         Where-Object { Test-Path -LiteralPath (Join-Path $_.FullName 'ajean.exe') } |
         Sort-Object Name -Descending)
  if (-not $p.Count) { return $null }
  return $p[0].FullName
}

# Les outils que le poseur du paquet installe dans data\scripts. LUS, jamais recopies :
# une liste d'outils ecrite a la main dans un controleur ment des qu'une piece est
# ajoutee ou change de nom (mesure du 2026-10-28 : la sonde annoncait « 7 / 9 » a vie
# et le verificateur ignorait les deux outils de la veille).
# La lecture se fait par PROFONDEUR de parentheses, pas par regex gloutonne : le
# tableau du poseur contient des commentaires qui portent des parentheses, et
# « (?s)…\) » s'arretait a la premiere d'entre elles.
function Outils-du-Poseur {
  param([string]$Paquet)
  if (-not $Paquet) { return @() }
  $poseur = Join-Path $Paquet 'Appliquer-Patch-Canal.ps1'
  if (-not (Test-Path -LiteralPath $poseur -PathType Leaf)) { return @() }
  $lignes = @([IO.File]::ReadAllLines($poseur, [Text.Encoding]::UTF8))
  $debut = -1
  for ($i = 0; $i -lt $lignes.Count; $i++) {
    if ($lignes[$i] -match '^\s*\$OutilsInstalles\s*=\s*@\(') { $debut = $i ; break }
  }
  if ($debut -lt 0) { return @() }
  $bloc = @()
  $profondeur = 0
  for ($i = $debut; $i -lt $lignes.Count; $i++) {
    $brut = $lignes[$i]
    $bloc += $brut
    if ($brut.TrimStart().StartsWith('#')) { continue }   # les parentheses d'un commentaire ne comptent pas
    foreach ($ch in $brut.ToCharArray()) {
      if ($ch -eq '(') { $profondeur++ }
      elseif ($ch -eq ')') { $profondeur-- }
    }
    if ($profondeur -le 0) { break }
  }
  $corps = ($bloc -join "`n")
  # On ne garde que les litteraux entre apostrophes des lignes qui ne sont pas des
  # commentaires : les commentaires citent parfois un nom de fichier.
  $noms = @()
  foreach ($lg in ($bloc -split "`n")) {
    if ($lg.TrimStart().StartsWith('#')) { continue }
    foreach ($x in [regex]::Matches($lg, "'([^']+)'")) { $noms += $x.Groups[1].Value }
  }
  return @($noms | Where-Object { $_ } | Sort-Object -Unique)
}

if ($MyInvocation.InvocationName -ne '.') {
  # Execute directement (pas dot-source) : on imprime l'etat, c'est deja une
  # reponse pour l'humain qui cherche ou il en est.
  $tout = @(Trouver-Installations)
  Write-Output '== INSTALLATIONS TROUVEES (par binaire, pas par nom de dossier) =='
  foreach ($i in $tout) {
    if ($i.SansBinaire) { Write-Output ('  ' + $i.Nom.PadRight(42) + '  (aucun ajean.exe a la racine)'); continue }
    $verrou = if (@($i.VerrouillePar).Count) { 'EN SERVICE PID ' + (@($i.VerrouillePar) -join ',') } else { 'arretée' }
    # Le « nom attendu » n'a de sens que pour une ligne Cresius : c'est la routine
    # de nommage du projet, pas une regle generale.
    $ecart = '(ligne hors projet Cresius)'
    if ($i.EstCresius) {
      $attendu = Nom-Attendu -Version $i.Version
      $ecart = if ($attendu -eq $i.Nom) { 'nom coherent' } else { 'le dossier s appelle autre chose -> ' + $attendu }
    }
    Write-Output ('  ' + $i.Version.PadRight(22) + ' ' + $i.Nom)
    Write-Output ('  ' + ''.PadRight(22) + ' ' + $verrou.PadRight(26) + ' ' + $ecart)
  }
  # Un() OBLIGATOIRE ici : une fonction PowerShell DEROULE le tableau a la sortie,
  # donc un seul resultat revient comme un objet simple et sa propriete Count est
  # vide (mesure du 2026-10-28 : «variante detectee = » sans nombre).
  Write-Output ('principale detectee = ' + @(Trouver-Principale -Installations $tout).Count)
  Write-Output ('variante detectee   = ' + @(Trouver-Variante -Installations $tout).Count)
}
