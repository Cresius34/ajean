[CmdletBinding()]
param()
$ErrorActionPreference = 'Stop'
$app = Join-Path $PSScriptRoot 'ajean-memoire-test.exe'
if (-not (Test-Path -LiteralPath $app)) { throw 'Extrais tout le ZIP avant de lancer le test.' }
$expectedVersion = '0.18.1-embedding-test.5'
$diskVersion = [Diagnostics.FileVersionInfo]::GetVersionInfo($app).ProductVersion
if ($diskVersion -ne $expectedVersion) {
    throw "Fichiers de versions differentes : le lanceur attend $expectedVersion mais cet executable annonce $diskVersion. Extrais de nouveau le paquet R5 complet dans le dossier du TEST."
}
$chatUrl = 'http://127.0.0.1:18090/?memory-test=' + $expectedVersion
Write-Host "AJEAN Memoire TEST R5 : $app"
$state = $null
try { $state = Invoke-RestMethod -Uri 'http://127.0.0.1:18090/api/embeddings' -TimeoutSec 2 } catch { }
if ($null -ne $state) {
    $expectedHome = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot 'data'))
    if (-not $state.parallel -or $state.home -ne $expectedHome) {
        throw 'Le port 18090 est deja utilise par une autre instance. Aucun processus ne sera arrete.'
    }
    if ($state.version -ne $expectedVersion -or $state.memory_ui_revision -ne 5) {
        throw "L ancienne version $($state.version) tourne encore sur 18090. Quitte AJEAN Memoire TEST depuis son icone pres de l horloge, puis relance Lancer-Test.cmd. Fermer l onglet ne ferme pas le programme."
    }
    if ($state.executable -ne [IO.Path]::GetFullPath($app)) {
        throw 'Une autre copie du TEST occupe le port 18090. Quitte cette copie depuis son icone pres de l horloge, puis relance ce dossier.'
    }
    Start-Process $chatUrl
    return
}
& (Join-Path $PSScriptRoot 'Installer-Modele.ps1')
Write-Host "Lancement de $expectedVersion sur $chatUrl"
Write-Host 'Le menu doit afficher TEST memoire R5. Reglages : Parametres > General et Presets > Modele > Memoire.'
Write-Host 'Pour quitter : icone AJEAN Memoire TEST pres de l horloge > Quitter le TEST.'
& $app app
if ($LASTEXITCODE -ne 0) { throw "Ajean TEST a quitte avec le code $LASTEXITCODE." }
