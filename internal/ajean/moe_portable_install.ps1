[CmdletBinding()]
param(
    [Parameter(Mandatory=$true)][string]$MoeHome,
    [ValidateSet('hip','cuda')][string]$Backend='hip',
    [ValidateSet('12','13')][string]$Toolkit='13',
    [ValidateSet('swift','qwen')][string]$Family='swift',
    [ValidateSet('Q2_0','IQ2_XS','IQ3_XXS','IQ3_S')][string]$Quant='IQ3_XXS',
    [int]$Context=86016,
    [int]$ReserveMiB=4096,
    [switch]$Vision
)
$ErrorActionPreference='Stop'
[Net.ServicePointManager]::SecurityProtocol=[Net.SecurityProtocolType]::Tls12
$ProgressPreference='SilentlyContinue'
$MoeHome=[IO.Path]::GetFullPath($MoeHome)
$manifestPath=Join-Path $MoeHome 'windows-manifest.json'
$manifest=Get-Content -LiteralPath $manifestPath -Raw | ConvertFrom-Json
$engineKey=if($Backend -eq 'hip'){'hip'}else{'cuda'+$Toolkit}
$engine=$manifest.engines.$engineKey
$cache=Join-Path $MoeHome 'dl\strata-0.1.44.1'
New-Item -ItemType Directory -Path $cache -Force | Out-Null
function Get-Verified($item){
    $path=Join-Path $cache $item.name
    if(Test-Path -LiteralPath $path){
        if((Get-FileHash -LiteralPath $path -Algorithm SHA256).Hash.ToLowerInvariant() -eq $item.sha256){return $path}
        throw "Archive existante incorrecte conservee : $path"
    }
    $part=$path+'.part'
    Write-Host ('Telechargement : '+$item.name)
    Invoke-WebRequest -UseBasicParsing -Uri $item.url -OutFile $part -TimeoutSec 3600
    if((Get-FileHash -LiteralPath $part -Algorithm SHA256).Hash.ToLowerInvariant() -ne $item.sha256){throw ('SHA-256 incorrect : '+$item.name)}
    Move-Item -LiteralPath $part -Destination $path
    return $path
}
$sourceZip=Get-Verified $manifest.source
$engineZip=Get-Verified $engine
$suffix=if($Backend -eq 'hip'){'hip'}else{'cuda'+$Toolkit}
$source=Join-Path $MoeHome ('strata-'+$manifest.version+'-'+$suffix)
$marker=Join-Path $source '.ajean-source-sha256'
$staging=Join-Path $source '.ajean-installing'
if(-not(Test-Path -LiteralPath $marker)){
    if((Test-Path -LiteralPath $source) -and (Get-ChildItem -LiteralPath $source -Force | Select-Object -First 1) -and -not(Test-Path -LiteralPath $staging)){throw 'Dossier source non gere conserve. Choisir un dossier TEST propre.'}
    New-Item -ItemType Directory -Path $source -Force | Out-Null
    [IO.File]::WriteAllText($staging,'Ajean MoE R6',[Text.Encoding]::ASCII)
    Add-Type -AssemblyName System.IO.Compression.FileSystem
    $zip=[IO.Compression.ZipFile]::OpenRead($sourceZip)
    try{
        foreach($entry in $zip.Entries){
            if(-not $entry.FullName.StartsWith($manifest.source.root)){continue}
            $rel=$entry.FullName.Substring($manifest.source.root.Length)
            if($rel -eq '' -or $rel.EndsWith('/')){continue}
            if($rel -match '(^|/)\.\.(/|$)' -or [IO.Path]::IsPathRooted($rel)){throw 'Chemin archive invalide'}
            $top=($rel -split '/')[0]
            if($top -notin @('setup.py','requirements.txt','CMakeLists.txt','LICENSE','VERSION','serve','tools','include','src','cmake','third_party')){continue}
            $dest=Join-Path $source $rel
            New-Item -ItemType Directory -Path ([IO.Path]::GetDirectoryName($dest)) -Force | Out-Null
            [IO.Compression.ZipFileExtensions]::ExtractToFile($entry,$dest,$true)
        }
    }finally{$zip.Dispose()}
    [IO.File]::WriteAllText($marker,$manifest.source.sha256,[Text.Encoding]::ASCII)
    Remove-Item -LiteralPath $staging
}
if([IO.File]::ReadAllText($marker).Trim() -ne $manifest.source.sha256){throw 'Sources d une autre version conservees.'}
$pythonHome=Join-Path $source 'python'
$python=Join-Path $pythonHome 'python.exe'
$pythonMarker=Join-Path $pythonHome '.ajean-python'
$pythonStaging=Join-Path $pythonHome '.ajean-python-installing'
$env:PIP_CACHE_DIR=Join-Path $MoeHome 'cache\pip'
$env:PIP_CONFIG_FILE='NUL'
foreach($name in @('PIP_TARGET','PIP_PREFIX','PIP_USER','PYTHONHOME','PYTHONPATH')){Remove-Item -LiteralPath ('Env:'+ $name) -ErrorAction SilentlyContinue}
$env:PYTHONNOUSERSITE='1'
$env:PYTHONUNBUFFERED='1'
if(-not(Test-Path -LiteralPath $pythonMarker)){
    if((Test-Path -LiteralPath $pythonHome) -and (Get-ChildItem -LiteralPath $pythonHome -Force | Select-Object -First 1) -and -not(Test-Path -LiteralPath $pythonStaging)){throw 'Python existant non gere conserve.'}
    New-Item -ItemType Directory -Path $pythonHome -Force | Out-Null
    [IO.File]::WriteAllText($pythonStaging,'Ajean portable Python 3.12.10',[Text.Encoding]::ASCII)
    $pythonZip=Get-Verified $manifest.python
    Expand-Archive -LiteralPath $pythonZip -DestinationPath $pythonHome -Force
    [IO.File]::WriteAllText((Join-Path $pythonHome 'python312._pth'),"python312.zip`r`n.`r`n..`r`n..\tools`r`nLib\site-packages`r`nimport site`r`n",[Text.Encoding]::ASCII)
    [IO.File]::WriteAllText($pythonMarker,'Ajean portable Python 3.12.10',[Text.Encoding]::ASCII)
    Remove-Item -LiteralPath $pythonStaging
}
if(-not(Test-Path -LiteralPath (Join-Path $pythonHome 'Lib\site-packages\pip'))){
    $pipScript=Get-Verified $manifest.pip
    & $python $pipScript --no-warn-script-location
    if($LASTEXITCODE -ne 0){throw 'Initialisation pip echouee.'}
}
Copy-Item -LiteralPath (Join-Path $MoeHome 'ajean_setup.py') -Destination (Join-Path $source 'ajean_setup.py') -Force
$data=Join-Path $MoeHome 'data'
$arguments=@((Join-Path $source 'ajean_setup.py'),'--ajean-manifest',$manifestPath,'--setup','--yes','--no-start','--no-browser','--backend',$Backend,'--family',$Family,'--model',$Quant,'--context',[string]$Context,'--kv','int8','--vision',$(if($Vision){'gpu'}else{'none'}),'--vram-reserve-mib',[string]$ReserveMiB,'--host','127.0.0.1','--port','18080','--data-dir',$data,'--models-dir',(Join-Path $data 'models'),'--prebuilt',($cache+[IO.Path]::DirectorySeparatorChar))
if($Backend -eq 'cuda'){$arguments+=@('--cuda',$Toolkit)}
Push-Location $source
try{
    & $python @arguments
    if($LASTEXITCODE -ne 0){throw ('Preparation MoE echouee, code '+$LASTEXITCODE)}
}finally{Pop-Location}
Write-Host 'MoE prepare dans le dossier portable. Aucun service, PATH ou SDK systeme installe.'
