[CmdletBinding()]
param()
$ErrorActionPreference = 'Stop'
$runtimeDirectory = Join-Path $PSScriptRoot 'data\embedding-python'
$modelDirectory = Join-Path $PSScriptRoot 'data\models\embeddinggemma-2-full'
$python = Join-Path $runtimeDirectory 'python.exe'
$manifest = Join-Path $runtimeDirectory 'ajean-portable-runtime.txt'
$staging = Join-Path $runtimeDirectory '.ajean-runtime-staging'
[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12
$ProgressPreference = 'SilentlyContinue'
if ((Test-Path -LiteralPath $python) -and -not (Test-Path -LiteralPath $manifest) -and -not (Test-Path -LiteralPath $staging)) {
    throw 'Runtime Python existant non gere par cet installeur : il est conserve. Choisis-le directement dans le preset.'
}
if (-not (Test-Path -LiteralPath $manifest) -or -not (Test-Path -LiteralPath $python)) {
    if ((Test-Path -LiteralPath $runtimeDirectory) -and (Get-ChildItem -LiteralPath $runtimeDirectory -Force | Select-Object -First 1) -and -not (Test-Path -LiteralPath $staging)) { throw 'Le dossier Python non vide est conserve.' }
    New-Item -ItemType Directory -Path $runtimeDirectory -Force | Out-Null
    [IO.File]::WriteAllText($staging, 'Ajean portable runtime staging', [Text.Encoding]::ASCII)
    $archive = Join-Path $runtimeDirectory 'python-embed.zip'
    Invoke-WebRequest -UseBasicParsing -Uri 'https://www.python.org/ftp/python/3.12.10/python-3.12.10-embed-amd64.zip' -OutFile $archive -TimeoutSec 300
    $expectedPython = '4acbed6dd1c744b0376e3b1cf57ce906f9dc9e95e68824584c8099a63025a3c3'
    if ((Get-FileHash -LiteralPath $archive -Algorithm SHA256).Hash.ToLowerInvariant() -ne $expectedPython) { throw 'Empreinte Python incorrecte.' }
    Expand-Archive -LiteralPath $archive -DestinationPath $runtimeDirectory -Force
    Remove-Item -LiteralPath $archive
    [IO.File]::WriteAllText((Join-Path $runtimeDirectory 'python312._pth'), "python312.zip`r`n.`r`nLib\site-packages`r`nimport site`r`n", [Text.Encoding]::ASCII)
    [IO.File]::WriteAllText($manifest, 'Ajean memory portable Python 3.12.10', [Text.Encoding]::ASCII)
    Remove-Item -LiteralPath $staging
}
$bootstrap = Join-Path $runtimeDirectory 'get-pip.py'
if (-not (Test-Path -LiteralPath (Join-Path $runtimeDirectory 'Lib\site-packages\pip'))) {
    Invoke-WebRequest -UseBasicParsing -Uri 'https://bootstrap.pypa.io/get-pip.py' -OutFile $bootstrap -TimeoutSec 300
    if ((Get-FileHash -LiteralPath $bootstrap -Algorithm SHA256).Hash.ToLowerInvariant() -ne 'fb24e693bab954209a063d90953621412ccad4a500905a726286e038f508ddf6') { throw 'Empreinte du bootstrap pip incorrecte : fichier conserve, installation arretee.' }
    & $python $bootstrap --no-warn-script-location
    if ($LASTEXITCODE -ne 0) { throw 'Initialisation pip echouee.' }
}
Write-Host 'Installation locale des dependances CPU (plusieurs Go libres recommandes). Aucun PATH ou Python systeme modifie.'
& $python -m pip install --disable-pip-version-check 'torch==2.14.1' 'torchvision==0.29.1' --index-url 'https://download.pytorch.org/whl/cpu' --no-warn-script-location
if ($LASTEXITCODE -ne 0) { throw 'Installation PyTorch CPU echouee.' }
& $python -m pip install --disable-pip-version-check -r (Join-Path $PSScriptRoot 'embedding-requirements.txt') --no-warn-script-location
if ($LASTEXITCODE -ne 0) { throw 'Installation des dependances multimodales echouee.' }
$modelMarker = Join-Path $modelDirectory '.ajean-model-revision'
$revision = '914f7f89142e33e77833254d9c9b90c3cef7303b'
if ((Test-Path -LiteralPath $modelDirectory) -and (Get-ChildItem -LiteralPath $modelDirectory -Force | Select-Object -First 1)) {
    if (-not (Test-Path -LiteralPath $modelMarker) -or ([IO.File]::ReadAllText($modelMarker).Trim() -ne $revision)) { throw 'Dossier modele existant non gere par cet installeur : il est conserve.' }
}
New-Item -ItemType Directory -Path $modelDirectory -Force | Out-Null
[IO.File]::WriteAllText($modelMarker, $revision, [Text.Encoding]::ASCII)
$env:HF_HOME = Join-Path $PSScriptRoot 'data\cache\huggingface'
$downloadScript = Join-Path $runtimeDirectory 'download-embeddinggemma2.py'
$downloadCode = @'
import sys
from huggingface_hub import snapshot_download
snapshot_download('google/embeddinggemma-2', revision='914f7f89142e33e77833254d9c9b90c3cef7303b', local_dir=sys.argv[1])
'@
[IO.File]::WriteAllText($downloadScript, $downloadCode, (New-Object Text.UTF8Encoding($false)))
& $python $downloadScript $modelDirectory
if ($LASTEXITCODE -ne 0) { throw 'Telechargement du modele complet echoue.' }
Write-Host 'Encodeur multimodal CPU pret. Dans le preset, choisis Complet puis les modalites utiles.'
Write-Host 'GPU : choisir un runtime Python portable avec PyTorch compatible avec ton GPU. Cet installeur fournit la variante CPU.'
