[CmdletBinding()]
param([string]$Source = (Join-Path $env:ProgramData 'ajean\presets'))
$ErrorActionPreference = 'Stop'
$destination = Join-Path $PSScriptRoot 'data\presets'
if (-not (Test-Path -LiteralPath $Source)) { throw "Dossier presets introuvable : $Source" }
New-Item -ItemType Directory -Path $destination -Force | Out-Null
$copied = 0
foreach ($file in Get-ChildItem -LiteralPath $Source -Filter '*.env' -File) {
    [IO.File]::Copy($file.FullName, (Join-Path $destination $file.Name), $false)
    $copied++
}
Write-Host "Presets copies : $copied"
Write-Host "Destination : $destination"
