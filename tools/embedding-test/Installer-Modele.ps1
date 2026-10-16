[CmdletBinding()]
param()
$ErrorActionPreference = 'Stop'
$modelDirectory = Join-Path $PSScriptRoot 'data\models'
$modelPath = Join-Path $modelDirectory 'embeddinggemma-2-Q8_0.gguf'
$expectedHash = '2188ac1deca4b77dffefd603c2776a9d76d9d74ec01841392982ebb840b09135'
$modelUrl = 'https://huggingface.co/ggml-org/embeddinggemma-2-GGUF/resolve/bfcd298762cc34d0357ece5ebdd31791a3a374d8/embeddinggemma-2-Q8_0.gguf'
New-Item -ItemType Directory -Path $modelDirectory -Force | Out-Null
if (Test-Path -LiteralPath $modelPath) {
    if ((Get-FileHash -LiteralPath $modelPath -Algorithm SHA256).Hash.ToLowerInvariant() -ne $expectedHash) {
        throw "Un modele different est deja present dans $modelPath. Il est conserve ; utilise un nouveau dossier de test."
    }
    Write-Host 'EmbeddingGemma 2 Q8_0 est deja present et verifie.'
    return
}
Write-Host 'Telechargement du modele texte EmbeddingGemma 2 Q8_0 (310 Mo).'
Write-Host 'Source : ggml-org/embeddinggemma-2-GGUF ; licence Apache 2.0.'
Write-Host 'Le modele, les donnees et la configuration de ton Ajean habituel restent dans leur dossier.'
$partialPath = Join-Path $modelDirectory ('embeddinggemma-2-' + [Guid]::NewGuid().ToString('N') + '.part')
try {
    [Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12
    $ProgressPreference = 'SilentlyContinue'
    Invoke-WebRequest -Uri $modelUrl -OutFile $partialPath -UseBasicParsing -TimeoutSec 900
    if ((Get-FileHash -LiteralPath $partialPath -Algorithm SHA256).Hash.ToLowerInvariant() -ne $expectedHash) {
        throw 'Empreinte SHA256 du modele incorrecte : le fichier ne sera pas installe.'
    }
    [IO.File]::Move($partialPath, $modelPath)
    Write-Host "Modele installe et verifie : $modelPath"
} finally {
    if (Test-Path -LiteralPath $partialPath) { Remove-Item -LiteralPath $partialPath }
}
