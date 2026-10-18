[CmdletBinding()]
param([string]$OutputDirectory = '')
$ErrorActionPreference = 'Stop'
$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
if ($OutputDirectory -eq '') { $OutputDirectory = Join-Path $repoRoot 'dist\Ajean-Memoire-Test' }
$targetDirectory = [IO.Path]::GetFullPath($OutputDirectory)
New-Item -ItemType Directory -Path $targetDirectory -Force | Out-Null
$savedGoOS = $env:GOOS; $savedGoArch = $env:GOARCH; $savedCgo = $env:CGO_ENABLED
Push-Location $repoRoot
try {
    & go generate ./internal/ajean
    if ($LASTEXITCODE -ne 0) { throw 'Assemblage de l interface echoue.' }
    $env:GOOS = 'windows'; $env:GOARCH = 'amd64'; $env:CGO_ENABLED = '0'
    & go build -trimpath -ldflags '-s -w' -o (Join-Path $targetDirectory 'ajean-memoire-test.exe') ./cmd/ajean-memory-test
    if ($LASTEXITCODE -ne 0) { throw 'Compilation du prototype echouee.' }
    foreach ($name in @('Lancer-Test','Installer-Modele','Copier-Memoire','Copier-Presets','Installer-Multimodal')) {
        foreach ($extension in @('.cmd','.ps1')) {
            Copy-Item -LiteralPath (Join-Path $PSScriptRoot ($name + $extension)) -Destination $targetDirectory -Force
        }
    }
    Copy-Item -LiteralPath (Join-Path $PSScriptRoot 'embedding-requirements.txt') -Destination $targetDirectory -Force
    Copy-Item -LiteralPath (Join-Path $repoRoot 'docs\embedding-memory-test.md') -Destination (Join-Path $targetDirectory 'README.md') -Force
    $archive = Join-Path $targetDirectory 'llama-cpu.zip'
    $downloadUrl = 'https://github.com/ggml-org/llama.cpp/releases/download/b11669/llama-b11669-bin-win-cpu-x64.zip'
    $expected = '37b3efcf962847ca869745fd4ccf7f5e7e8dacfeff9fa3523b4ca8cdc66d632b'
    [Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12
    $ProgressPreference = 'SilentlyContinue'
    try {
        Invoke-WebRequest -Uri $downloadUrl -OutFile $archive -UseBasicParsing -TimeoutSec 300
        if ((Get-FileHash -LiteralPath $archive -Algorithm SHA256).Hash.ToLowerInvariant() -ne $expected) { throw 'Empreinte du moteur CPU incorrecte.' }
        Expand-Archive -LiteralPath $archive -DestinationPath (Join-Path $targetDirectory 'encoder') -Force
    } finally { if (Test-Path -LiteralPath $archive) { Remove-Item -LiteralPath $archive } }
    $licenceDirectory = Join-Path $targetDirectory 'licences'
    New-Item -ItemType Directory -Path $licenceDirectory -Force | Out-Null
    Copy-Item -LiteralPath (Join-Path $repoRoot 'LICENSE') -Destination (Join-Path $licenceDirectory 'Ajean-MIT.txt') -Force
    Invoke-WebRequest -Uri 'https://raw.githubusercontent.com/ggml-org/llama.cpp/620d2225c/LICENSE' -OutFile (Join-Path $licenceDirectory 'llama.cpp-MIT.txt') -UseBasicParsing -TimeoutSec 30
    Write-Host "Prototype construit : $targetDirectory"
    Write-Host 'Le modele GGUF est telecharge par Lancer-Test au premier lancement.'
} finally {
    $env:GOOS = $savedGoOS; $env:GOARCH = $savedGoArch; $env:CGO_ENABLED = $savedCgo
    Pop-Location
}
