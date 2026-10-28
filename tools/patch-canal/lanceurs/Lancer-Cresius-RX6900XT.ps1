[CmdletBinding()]
param()
# Lanceur de l'installation autonome "Ajean v18.15.1 maj Cresius RX6900XT" (binaire 0.18.16.1 Cresius, base amont 0.18.16, paquet moteur hip-rdna2).
# Refuse de demarrer si le port 18090 est occupe par une autre instance (n'en arrete aucune).
$ErrorActionPreference = 'Stop'
$app = Join-Path $PSScriptRoot 'ajean.exe'
if (-not (Test-Path -LiteralPath $app)) { throw "Executable introuvable : $app" }
$expectedVersion = '0.18.16.1 Cresius'
$diskVersion = [Diagnostics.FileVersionInfo]::GetVersionInfo($app).ProductVersion
if ($diskVersion -ne $expectedVersion) {
    throw "Cet executable annonce $diskVersion, le dossier attend $expectedVersion."
}
$chatUrl = 'http://127.0.0.1:18090/'
$state = $null
try { $state = Invoke-RestMethod -Uri 'http://127.0.0.1:18090/api/embeddings' -TimeoutSec 2 } catch { }
if ($null -ne $state) {
    $expectedHome = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot 'data'))
    if (-not $state.parallel -or $state.home -ne $expectedHome) {
        throw "Le port 18090 est occupe par une autre instance ($($state.home)). Aucune ne sera arretee : quittez-la depuis son icone pres de l'horloge."
    }
    if ($state.executable -ne [IO.Path]::GetFullPath($app)) {
        throw "Une autre copie occupe 18090 ($($state.executable)). Quittez-la depuis son icone pres de l'horloge, puis relancez ce dossier."
    }
    Write-Host "Instance deja en service sur $chatUrl"
    Start-Process $chatUrl
    return
}
Write-Host "Demarrage de $app"
Write-Host "Interface : $chatUrl   (le moteur MoE se demarre depuis l'interface)"
& $app app
if ($LASTEXITCODE -ne 0) { throw "Ajean a quitte avec le code $LASTEXITCODE." }
