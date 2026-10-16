[CmdletBinding()]
param([string]$Source = (Join-Path $env:ProgramData 'ajean\presets'))
$ErrorActionPreference = 'Stop'
$app = Join-Path $PSScriptRoot 'ajean-memoire-test.exe'
if (-not (Test-Path -LiteralPath $app)) { throw 'Extrais tout le ZIP avant de lancer le test.' }
& $app import-presets $Source
if ($LASTEXITCODE -ne 0) { throw "Import des presets interrompu (code $LASTEXITCODE)." }
Write-Host 'Recharge la page du TEST pour voir les presets importes.'
Write-Host 'Les presets, moteurs, DLL et modeles ont ete copies dans data. Les chemins sont relatifs au dossier portable.'
Write-Host 'Pour charger Swift localement dans le TEST, arrete d abord son moteur dans Ajean habituel afin de liberer la VRAM.'
