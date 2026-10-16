[CmdletBinding()]
param([switch]$Force)
$ErrorActionPreference = 'Stop'
$pyDir = Join-Path $PSScriptRoot 'embedding-python'
$pyExe = Join-Path $pyDir 'python.exe'
if ((Test-Path -LiteralPath $pyExe) -and -not $Force) {
    Write-Host 'Python portable deja installe : ' $pyDir
    return
}
Write-Host 'Installation de Python portable + dependances multimodales ...'
# Telecharger python-embed
$ver = '3.12.8'
$arch = 'x64'
$zip = Join-Path $env:TEMP "python-$ver-embed-$arch.zip"
$url = "https://www.python.org/ftp/python/$ver/python-$ver-embed-$arch.zip"
[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12
$ProgressPreference = 'SilentlyContinue'
if (-not (Test-Path $zip)) { Invoke-WebRequest -Uri $url -OutFile $zip -UseBasicParsing -TimeoutSec 120 }
if (Test-Path $pyDir) { Remove-Item -Recurse -Force $pyDir }
Expand-Archive -LiteralPath $zip -DestinationPath $pyDir -Force
# Activer site-packages
$pth = Join-Path $pyDir 'python312._pth'
if (Test-Path $pth) {
    $content = Get-Content $pth -Raw
    $content = $content -replace '#import site', 'import site'
    Set-Content $pth $content
}
# pip
$ProgressPreference = 'SilentlyContinue'
& $pyExe -m ensurepip --default-pip 2>&1 | Out-Null
# dependances
$reqFile = Join-Path $PSScriptRoot 'embedding-requirements.txt'
Write-Host 'Installation des paquets (torch, sentence-transformers, ...) ...'
& $pyExe -m pip install --quiet --no-warn-script-location -r $reqFile
if ($LASTEXITCODE -ne 0) { throw 'pip install a echoue.' }
Write-Host 'Python portable pret : ' $pyExe
