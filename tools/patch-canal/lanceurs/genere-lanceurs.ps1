# genere-lanceurs.ps1 — remet le VERROU de version des lanceurs du paquet au
# millésime du binaire LIVRÉ dans ce paquet.
#
# Ce qui était cablé et ne l'est plus :
#   - l'ancienne version copiait une LISTE de versions de départ admises
#     ($Depar = 18.15.3, .4, .5, .6). A chaque révision il fallait l'allonger ;
#     sinon le générateur refusait de tourner. Ici : on remplace LE verrou trouvé,
#     quel qu'il soit, et on affiche ancien -> nouveau.
#   - l'ancienne version écrivait « 0.18.16.1 Cresius » en dur. Ici : la version
#     est LUE dans le binaire à côté (ajean.exe du paquet), jamais supposée.
#
# Les lanceurs eux-mêmes ne nomment AUCUN dossier : ils travaillent depuis
# $PSScriptRoot et verrouillent par la version du binaire. Le nom du dossier est
# donc libre — c'est ce que demande la règle « versions portables ».
#
# Encodage (règle mesurée, ne pas « nettoyer ») : BOM UTF-8 conservé tel quel sur
# les .ps1 ; un .ps1 accentué sans BOM est illisible en PowerShell 5.1.
param(
  [string]$VersionLue = ''   # vide = lue dans le binaire du paquet
)
$ErrorActionPreference = 'Stop'

if (-not $VersionLue) {
  # Le paquet : un dossier au-dessus de lanceurs\.
  $Paquet = Split-Path $PSScriptRoot -Parent
  $exe = Join-Path $Paquet 'ajean.exe'
  if (-not (Test-Path -LiteralPath $exe)) {
    throw "ajean.exe introuvable dans $Paquet — le binaire doit être en place avant de générer les lanceurs"
  }
  $VersionLue = [Diagnostics.FileVersionInfo]::GetVersionInfo($exe).ProductVersion
  if (-not $VersionLue) { throw 'version illisible dans le binaire — ne pas poser de verrou' }
}
Write-Output ('verrou visé : ' + $VersionLue + '  (lu dans le binaire, pas supposé)')

foreach ($nom in (Get-ChildItem -LiteralPath $PSScriptRoot -Filter 'Lancer-*.ps1' -File | ForEach-Object { $_.Name })) {
  $chemin = Join-Path $PSScriptRoot $nom

  # Secours écrit à côté : sans lui, une écriture manquée laisserait le paquet sans lanceur.
  $secours = Join-Path $env:TEMP ('ajean-lanceur-secours-' + $nom)
  Copy-Item -LiteralPath $chemin -Destination $secours -Force

  $octets = [IO.File]::ReadAllBytes($chemin)
  $bom = ($octets.Length -ge 3 -and $octets[0] -eq 0xEF -and $octets[1] -eq 0xBB -and $octets[2] -eq 0xBF)
  $t = [Text.Encoding]::UTF8.GetString($octets)
  if ($bom) { $t = $t.Substring(1) }

  $m = [regex]::Match($t, '\$expectedVersion\s*=\s*''([^'']*)''')
  if (-not $m.Success) { throw "aucun verrou de version dans $nom — refus, un lanceur sans verrou démarre n'importe quoi" }
  $ancien = $m.Groups[1].Value

  $t2 = $t -replace ('(\$expectedVersion\s*=\s*''' + [regex]::Escape($ancien) + ''')'),
                     ('$expectedVersion = ''' + $VersionLue + '''')

  # Relecture contrôlée : écrire sans relire, c'est annoncer un succès qu'on n'a pas eu.
  $verif = [regex]::Match($t2, '\$expectedVersion\s*=\s*''([^'']*)''').Groups[1].Value
  if ($verif -ne $VersionLue) { throw "verrou non appliqué dans $nom (obtenu : '$verif')" }

  [IO.File]::WriteAllText($chemin, $t2, (New-Object Text.UTF8Encoding($bom)))
  Write-Output ('   ' + $nom.PadRight(32) + ' ' + $ancien + '  ->  ' + $VersionLue + '   BOM=' + $bom)
}
