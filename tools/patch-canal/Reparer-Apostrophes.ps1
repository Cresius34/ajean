# Remplace l'apostrophe typographique (U+2019) par deux apostrophes droites dans
# une chaine PowerShell, et signale celles qui restent dans du CODE (hors
# commentaire). U+2018 (guillemet simple ouvrant) est retire.
#
# Pourquoi : PowerShell 5.1 traite '' comme un DELIMITEUR de chaine, comme '.
# « l'aveugle » ecrit avec l'apostrophe typographique ferme la chaine, et le
# fichier devient inutilisable avec une erreur qui pointe ailleurs. Mesure du
# 2026-10-28 : la fabrique de paquet refusait de se parser a cause d'un seul ''.
param([string[]]$Fichiers = @())
$ErrorActionPreference = 'Stop'
$AposTypo = [string][char]0x2019
$GuilOuv  = [string][char]0x2018

if (-not $Fichiers.Count) {
  $Fichiers = @(Get-ChildItem -LiteralPath $PSScriptRoot -Recurse -File -Filter '*.ps1' |
                ForEach-Object { $_.FullName })
}
foreach ($f in $Fichiers) {
  $octets = [IO.File]::ReadAllBytes($f)
  $bom = ($octets.Length -ge 3 -and $octets[0] -eq 0xEF -and $octets[1] -eq 0xBB -and $octets[2] -eq 0xBF)
  $t = [Text.Encoding]::UTF8.GetString($octets)
  if ($bom) { $t = $t.Substring(1) }
  $avant = ([regex]::Matches($t, $AposTypo)).Count + ([regex]::Matches($t, $GuilOuv)).Count
  if (-not $avant) { continue }
  # '' -> '' : dans une chaine delimitee par des apostrophes droites, '' rend une
  # apostrophe. Dans du texte libre, deux apostrophes se lisent. Aucun cas ne
  # casse le parseur.
  $t2 = $t.Replace($AposTypo, "''").Replace($GuilOuv, '')
  [IO.File]::WriteAllText($f, $t2, (New-Object Text.UTF8Encoding($bom)))
  Write-Output ((Split-Path $f -Leaf).PadRight(38) + ' remplaces=' + $avant + '  BOM=' + $bom)
}
