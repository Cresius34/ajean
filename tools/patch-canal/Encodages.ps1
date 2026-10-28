# Encodages.ps1 - etat, puis reparation ciblee des encodages du paquet.
#
# Deux pieges mesurés le 2026-10-27, tous deux reels :
#   1. `Get-ChildItem -LiteralPath X -Include *.ps1` IGNORE le -Include (il faut
#      -Path). Un poseur de BOM ecrit avec -Include a traite TOUS les fichiers du
#      paquet, y compris ajean.exe : trois octets ajoutes en tete, binaire mort.
#   2. Windows PowerShell 5.1 ne devine pas l'encodage d'un .ps1. Sans BOM, il lit
#      en CP1252 : un tiret demi-cadrin (E2 80 94) devient « a€" » — le 0x94 est
#      une VRAIE guillemet double en CP1252, donc la chaine se termine en plein
#      milieu et le fichier ne se parse plus. Le BOM est obligatoire des qu'un
#      .ps1 contient un caractere non ASCII.
#   3. L'inverse pour un .cmd : cmd.exe rend le BOM en « ? » et la premiere ligne
#      devient « ?@echo off », qui n'est plus une commande. Un .cmd doit rester
#      ASCII pur, sans BOM.
#
# Regles appliquees :
#   .ps1 avec non-ASCII  -> BOM
#   .ps1 sans non-ASCII  -> pas de BOM (reproduit l'octet du fichier en service)
#   .cmd                 -> ASCII pur, sans BOM
#   .patch .txt .md      -> sans BOM (un BOM gene `git apply`)
#   .exe .json .7z       -> jamais touches
param([string]$Racine = $PSScriptRoot, [switch]$Reparer)
$ErrorActionPreference = 'Stop'
$Interdits = @('.exe', '.json', '.7z', '.syso')

$fichiers = @(Get-ChildItem -Path $Racine -Recurse -File |
  Where-Object { $Interdits -notcontains $_.Extension.ToLower() })
Write-Output ("fichiers examines : " + $fichiers.Count + " (exclus : " + ($Interdits -join ' ') + ")")

foreach ($it in $fichiers) {
  $f = $it.FullName
  $ext = $it.Extension.ToLower()
  $b = [IO.File]::ReadAllBytes($f)
  $bom = ($b.Length -ge 3 -and $b[0] -eq 0xEF -and $b[1] -eq 0xBB -and $b[2] -eq 0xBF)
  $t = [Text.Encoding]::UTF8.GetString($b)
  if ($bom) { $t = $t.Substring(1) }
  $exot = @($t.ToCharArray() | Where-Object {
    ([int]$_ -gt 126) -or ([int]$_ -lt 32 -and $_ -ne "`r" -and $_ -ne "`n" -and $_ -ne "`t") })
  $nExot = $exot.Count

  $veutBom = $null
  switch ($ext) {
    '.cmd'  { $veutBom = $false }
    # Un BOM de trop ne casse rien ; le retirer change un octet qui marche et
    # fait diverger le paquet du fichier en service. On ne le pose que quand il
    # est necessaire, on ne l'enleve jamais.
    '.ps1'  { $veutBom = ($bom -or ($nExot -gt 0)) }
    default { $veutBom = $false }   # .patch .txt .md : pas de BOM
  }

  $cmdSale = ($ext -eq '.cmd' -and $nExot -gt 0)
  $ok = ($bom -eq $veutBom) -and -not $cmdSale
  $ligne = ("{0} | BOM={1} attendu {2} | non-ASCII={3}" -f $it.Name, $bom, $veutBom, $nExot)
  if ($ok) { Write-Output ("CONFORME  " + $ligne) ; continue }
  Write-Output ("A CORRIGER " + $ligne)
  if (-not $Reparer) { continue }

  $contenu = $t
  if ($ext -eq '.cmd') {
    # cmd.exe ne sait pas afficher un accent : on le remplace, on ne met pas de BOM.
    $contenu = [regex]::Replace($t, '[^\x20-\x7E\r\n]', '?')
  }
  [IO.File]::WriteAllText($f, $contenu, (New-Object Text.UTF8Encoding($veutBom)))
  $apres = [IO.File]::ReadAllBytes($f)
  $bomApres = ($apres.Length -ge 3 -and $apres[0] -eq 0xEF -and $apres[1] -eq 0xBB -and $apres[2] -eq 0xBF)
  if ($bomApres -ne $veutBom) { throw ("reparation non verifiee sur " + $it.Name) }
  if ($ext -eq '.cmd') {
    $reste = @([Text.Encoding]::GetEncoding(28591).GetString($apres).ToCharArray() | Where-Object { [int]$_ -gt 126 })
    if ($reste.Count -gt 0) { throw ("non-ASCII reste dans " + $it.Name) }
  }
  Write-Output ("   -> BOM=" + $veutBom + " verifie" + $(if ($ext -eq '.cmd') { ", ASCII pur verifie" } else { '' }))
}
