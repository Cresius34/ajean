# Verifier-Chemin-En-Dur.ps1 — garde-fou contre les chemins d'installation ecrits
# en dur. Ecrit le 2026-10-28.
#
# POURQUOI CE SCRIPT EXISTE : nos versions sont PORTABLES et se transposent d'une
# machine a l'autre, et le dossier d'installation est renomme a chaque changement de
# version. Un chemin ecrit en dur ne casse pas bruyamment : il ment. Mesure du jour :
# 14 de nos scripts citaient le nom du dossier, et le pont de l'app du tableau de bord
# se taisait parce que son chemin avait change. Pire : je câblais « Ajean v18.15.1
# maj Cresius RX6900XT » chez la voisine, qui tourne en realite dans
# « Ajean v18.11.1 maj Cresius RX6900XT » (lu dans son /v1/models) — trois allers
# retours perdus sur le canal a cause de ca.
#
# Ce qu'il accepte quand meme : un chemin en dur est LECTURE SEULE et justifie s'il
# designe un dossier MORT (repli historique, archive). Ces cas-la doivent etre
# signales avec un mot de justification, sinon le garde-fou devient du bruit et on
# l'eteindra. Le mot accepté : DECABLAGE-MORT (voir --lignes tolerees plus bas).
#
# LECTURE SEULE. Sortie != 0 s'il reste un chemin en dur non justifie.
param(
  [string[]]$Dossiers = @(),
  [string]$Motif = 'ProgramData\\Ajean\s+v\d',
  [switch]$Tout
)
$ErrorActionPreference = 'Continue'

if (-not $Dossiers -or -not $Dossiers.Count) {
  # On remonte depuis SA PROPRE position, jamais depuis un nom de dossier suppose.
  # outillage : <install>\data\workspace\atelier-r18\tools\patch-canal
  $depot = Split-Path (Split-Path $PSScriptRoot -Parent) -Parent          # …\atelier-r18
  $data  = Split-Path (Split-Path (Split-Path $depot -Parent) -Parent) -Parent  # …\data
  $Dossiers = @(
    (Join-Path $depot 'tools\patch-canal'),   # l'outillage gelé dans le dépôt
    (Join-Path $data  'data\scripts')         # les outils vivants de l'installation
  ) | Where-Object { Test-Path -LiteralPath $_ }
  if (-not $Dossiers.Count) {
    Write-Output 'DOSSIERS INTROUVABLES depuis ' + $PSScriptRoot + ' — passer -Dossiers.'
    exit 2
  }
}

Write-Output '== CHEMINS D INSTALLATION CRITS EN DUR =='
$total = 0
$justifies = 0
# Regle de rangement (pas un silence) : un script qui ne sert plus qu'a un dossier
# MORT doit etre deplace dans data\scripts\archive. Le garde-fou ne scanne pas un
# chemin contenant \archive\ — sinon il crie 40 fois sur des scripts deja joues et
# on l'eteint, ce qui est exactement ce qu'on veut eviter.
foreach ($d in $Dossiers) {
  foreach ($f in (Get-ChildItem -LiteralPath $d -Recurse -File -EA SilentlyContinue |
                  Where-Object { $_.Extension -in '.ps1', '.cmd', '.py' -and $_.FullName -notmatch '\\(build|node_modules|\.git|archive)\\' })) {
    $lignes = [IO.File]::ReadAllLines($f.FullName)
    $frappe = @()
    for ($i = 0; $i -lt $lignes.Count; $i++) {
      if ($lignes[$i] -match $Motif) {
        # Une ligne qui porte la trace de la justification est toleree explicitement.
        $justifiee = ($lignes[$i] -match 'DECABLAGE-MORT')
        if ($justifiee) { $justifies++ } else { $frappe += ($i + 1) }
      }
    }
    if (-not $frappe.Count -and -not $justifies) { continue }
    $total += $frappe.Count
    if ($frappe.Count) {
      Write-Output ('  ' + $f.FullName)
      foreach ($n in $frappe) {
        $t = $lignes[$n - 1].Trim()
        if ($t.Length -gt 120) { $t = $t.Substring(0, 120) + '…' }
        Write-Output ('      L' + $n + ' : ' + $t)
      }
    }
  }
}
Write-Output ''
Write-Output ('chemins en dur NON justifies : ' + $total)
Write-Output ('lignes tolerees (DECABLAGE-MORT) : ' + $justifies)
if ($total -gt 0) {
  Write-Output ''
  Write-Output 'CORRECTIF : dot-sourcer Trouver-Installations.ps1 et appeler Trouver-Principale /'
  Write-Output 'Trouver-Variante / Trouver-Paquet-Canal. Un outil ne doit jamais SUPPOSER le nom'
  Write-Output 'du dossier : il le TROUVE par la presence du binaire et sa version.'
  exit 1
}
Write-Output 'RIEN A REDIRE.'
exit 0
