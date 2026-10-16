[CmdletBinding()]
param()
$ErrorActionPreference = 'Stop'
$modelDir = Join-Path $PSScriptRoot 'data\models'
New-Item -ItemType Directory -Path $modelDir -Force | Out-Null
$modelFile = Join-Path $modelDir 'embeddinggemma-225m-q8_0.gguf'
if (Test-Path -LiteralPath $modelFile) {
    Write-Host 'Modele deja present : ' $modelFile
    return
}
Write-Host 'Telechargement de EmbeddingGemma 2 Q8_0 (225M, 768d) ...'
$url = 'https://huggingface.co/quantized-llm/EmbeddingGemma-225M-Q8_0/resolve/main/embeddinggemma-225m-q8_0.gguf'
[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12
$ProgressPreference = 'SilentlyContinue'
Invoke-WebRequest -Uri $url -OutFile $modelFile -UseBasicParsing -TimeoutSec 300
$size = (Get-Item $modelFile).Length
Write-Host "Modele installe : $size octets"
Write-Host $modelFile
