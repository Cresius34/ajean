# Renommer-Installation.ps1 — aligne le NOM du dossier sur la VERSION du binaire.
# Ecrit le 2026-10-28, apres le decablage des chemins en dur.
#
# Regle de securite, dans cet ordre :
#   1) on lit la version dans le BINAIRE, jamais dans le nom du dossier ;
#   2) on refuse si un processus tient le dossier (un exe en cours ne se deplace pas) ;
#   3) on renomme, puis on RELIT tout : le nom doit etre la, le binaire identique.
# Un renommage sur le meme volume ne copie rien : c'est une entree de repertoire.
#
# CE SCRIPT NE RENOMME PAS la principale s'il est joue depuis l'interieur : l'instance
# qui m'héberge tient ce dossier. Le refus est voulu et affiche les PID en cause.
param([switch]$Seccher)
$ErrorActionPreference = 'Stop'

. (Join-Path $PSScriptRoot 'Trouver-Installations.ps1')

function Version-Binaire($dossier) {
  $exe = Join-Path $dossier 'ajean.exe'
  if (-not (Test-Path -LiteralPath $exe -PathType Leaf)) { return $null }
  try { return [Diagnostics.FileVersionInfo]::GetVersionInfo($exe).ProductVersion } catch { return 'ILLISIBLE' }
}

$Seccher = [switch]$Seccher
$dossiers = @(
  @(Trouver-Principale | ForEach-Object { $_.Dossier })
  @(Trouver-Variante   | ForEach-Object { $_.Dossier })
) | Where-Object { $_ }

if (-not $dossiers.Count) { Write-Output 'aucune installation trouvee (aucun ajean.exe sous C:\ProgramData\Ajean*)' ; exit 1 }

foreach ($d in $dossiers) {
  Write-Output ('===== ' + $d)
  $v = Version-Binaire $d
  if (-not $v) { Write-Output '  PAS DE BINAIRE a la racine : on ne renomme pas (rien a aligner).' ; continue }

  # Le suffixe du dossier (RX6900XT, RTX, portable…) est conserve : il dit sur
  # quelle machine on est, et c'est l'humain qui l'a choisi.
  $nomActuel = Split-Path $d -Leaf
  $suffixe = ''
  $m = [regex]::Match($nomActuel, 'maj\s+Cresius\s+(.+)$')
  if ($m.Success) { $suffixe = $m.Groups[1].Value }
  $attendu = Nom-Attendu -Version $v -Suffixe $suffixe
  $parent = Split-Path $d -Parent
  $nouveau = Join-Path $parent $attendu

  Write-Output ('  version du binaire = ' + $v)
  Write-Output ('  nom du dossier     = ' + $nomActuel)
  Write-Output ('  nom aligne         = ' + $attendu)
  if ($nomActuel -eq $attendu) { Write-Output '  DEJA ALIGNE : rien a faire.' ; continue }

  # Verrous : tout processus dont l'executeur est SOUS ce dossier.
  $tenus = @(Get-CimInstance Win32_Process -EA SilentlyContinue |
             Where-Object { $_.ExecutablePath -and $_.ExecutablePath.ToLower().StartsWith($d.ToLower() + '\') })
  if ($tenus.Count) {
    Write-Output ('  REFUS : ' + $tenus.Count + ' processus tiennent ce dossier :')
    foreach ($p in $tenus) {
      Write-Output ('    PID ' + $p.ProcessId + '  ' + (Split-Path $p.ExecutablePath -Leaf) + '  ' + $p.ExecutablePath)
    }
    Write-Output '  => arreter l''instance (ou la jouer depuis l''exterieur) puis relancer ce script.'
    continue
  }
  if (Test-Path -LiteralPath $nouveau) {
    Write-Output ('  REFUS : le nom cible existe deja -> ' + $attendu)
    continue
  }

  $shaAvant = (Get-FileHash -LiteralPath (Join-Path $d 'ajean.exe') -Algorithm SHA256).Hash

  # On n'ANNONCE le renommage qu'apres l'avoir VERIFIE. Mesure du 2026-10-28 : le
  # premier jet affichait « RENOMME » alors que Rename-Item avait echoue (erreur
  # non terminative, le script continuait). Une annonce heureuse sur un echec est
  # le pire mensonge d'un outil de pose.
  try {
    Rename-Item -LiteralPath $d -NewName $attendu -ErrorAction Stop
  } catch {
    Write-Output ('  ECHEC du renommage : ' + $_.Exception.Message)
    Write-Output '  Aucune donnee bougee. Causes possibles : un fichier du dossier est ouvert'
    Write-Output '  (le renommage exige de pouvoir SUPPRIMER l entree du repertoire parent),'
    Write-Output '  ou le jeton sans elivation. Voir le README : jouer ce script depuis une'
    Write-Output '  console ELEVEE, instance arretee.'
    continue
  }
  if (-not (Test-Path -LiteralPath $nouveau)) {
    Write-Output ('  ECHEC : le renommage ne laisse rien a ' + $attendu)
    continue
  }
  Write-Output ('  RENOMME -> ' + $attendu)
  $shaApres = (Get-FileHash -LiteralPath (Join-Path $nouveau 'ajean.exe') -Algorithm SHA256).Hash
  Write-Output ('  binaire inchange  = ' + ($shaAvant -eq $shaApres) + '  (' + $shaApres.Substring(0,16) + ')')
  Write-Output ('  version relue     = ' + (Version-Binaire $nouveau))
}

Write-Output ''
Write-Output '===== CE QUE LA DECOUVERTE VOIT DESORMAIS (sans aucun chemin cable) ====='
foreach ($i in @(Trouver-Installations)) {
  if ($i.SansBinaire) { Write-Output ('  ' + $i.Nom + '  (aucun binaire)') ; continue }
  $attendu = Nom-Attendu -Version $i.Version
  $ecart = if ($attendu -eq $i.Nom) { 'nom coherent' } else { 'nom pas aligne -> ' + $attendu }
  Write-Output ('  ' + $i.Version.PadRight(22) + ' ' + $i.Nom.PadRight(44) + ' ' + $ecart)
}
