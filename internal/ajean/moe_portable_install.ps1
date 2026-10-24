[CmdletBinding()]
param(
    [Parameter(Mandatory=$true)][string]$MoeHome,
    [ValidateSet('hip','cuda')][string]$Backend='hip',
    [ValidateSet('12','13')][string]$Toolkit='13',
    [ValidateSet('auto','rx9070','rdna3','rdna2')][string]$HipVariant='auto',
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
# 0.2.4 publishes one engine package per AMD card family, as a .tar.gz (the flat
# strata-windows-x64-hip.zip only exists up to release 0.1.45). Pick the package
# that carries code for the card in this PC, unless the caller chose it.
function Get-HipKey([string]$want){
    if($want -ne 'auto'){return 'hip-'+$want}
    $arch=''
    foreach($c in @(Get-CimInstance Win32_VideoController -ErrorAction SilentlyContinue | Where-Object {$_.PNPDeviceID -match 'VEN_1002'})){
        $id=$c.PNPDeviceID.ToUpperInvariant(); $name=$c.Name.ToUpperInvariant()
        if($id -match 'DEV_(7550|7551|7590)' -or $name -match 'R9700'){$arch='gfx1201';break}
        if($id -match 'DEV_(744C|7448|745E|747E|7480|7549|7557)' -or $name -match 'RX 7[0-9]{3}'){$arch='gfx1100'}
        if(($arch -eq '' -and $id -match 'DEV_(73BF|73AF|73A5)') -or $name -match 'RX 6[0-9]{3}'){if($arch -eq ''){$arch='gfx1030'}}
    }
    switch($arch){
        'gfx1201'{'hip-rx9070'}
        'gfx1200'{'hip-rx9070'}
        'gfx1151'{'hip-rdna3'}
        'gfx1102'{'hip-rdna3'}
        'gfx1101'{'hip-rdna3'}
        'gfx1100'{'hip-rdna3'}
        'gfx1031'{'hip-rdna2'}
        'gfx1030'{'hip-rdna2'}
        default  {'hip-rdna3'}
    }
}
$engineKey=if($Backend -eq 'hip'){Get-HipKey $HipVariant}else{'cuda'+$Toolkit}
if(-not $manifest.engines.$engineKey){throw ('paquet moteur '+$engineKey+' absent du manifeste epingle')}
$engine=$manifest.engines.$engineKey
$engineDir=if($engine.dir){$engine.dir}else{'engine'}
Write-Host ('Paquet moteur : '+$engine.name+' ('+($engine.archs -join ', ')+')')
$cache=Join-Path $MoeHome ('dl\strata-'+$manifest.version)
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
$engineArchive=Get-Verified $engine
$suffix=if($Backend -eq 'hip'){'hip'}else{'cuda'+$Toolkit}
$source=Join-Path $MoeHome ('strata-'+$manifest.engine_version+'-'+$suffix)
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
            if($top -notin @('setup.py','requirements.txt','CMakeLists.txt','LICENSE','VERSION','serve','tools','include','src','cmake','third_party','strata_control','strata_cli')){continue}
            $dest=Join-Path $source $rel
            New-Item -ItemType Directory -Path ([IO.Path]::GetDirectoryName($dest)) -Force | Out-Null
            [IO.Compression.ZipFileExtensions]::ExtractToFile($entry,$dest,$true)
        }
    }finally{$zip.Dispose()}
    [IO.File]::WriteAllText($marker,$manifest.source.sha256,[Text.Encoding]::ASCII)
    Remove-Item -LiteralPath $staging
}
if([IO.File]::ReadAllText($marker).Trim() -ne $manifest.source.sha256){throw 'Sources d une autre version conservees.'}
# The engine itself: unpacked here, from the pinned .tar.gz, before setup runs.
# setup.py then finds an installed ready-made engine (BUILD.json source=prebuilt)
# for this card and never downloads or compiles one.
$enginePath=Join-Path $source $engineDir
$engineMarker=Join-Path $source '.ajean-engine-sha256'
if(-not(Test-Path -LiteralPath $engineMarker) -or [IO.File]::ReadAllText($engineMarker).Trim() -ne $engine.sha256){
    $tmp=Join-Path $source '.ajean-engine-unpack'
    if(Test-Path -LiteralPath $tmp){Remove-Item -LiteralPath $tmp -Recurse -Force}
    New-Item -ItemType Directory -Path $tmp -Force | Out-Null
    & tar.exe -xzf $engineArchive -C $tmp
    if($LASTEXITCODE -ne 0){throw ('extraction du moteur echouee, code '+$LASTEXITCODE)}
    if(-not(Test-Path -LiteralPath (Join-Path $tmp 'BUILD.json'))){throw 'archive moteur sans BUILD.json'}
    if(Test-Path -LiteralPath $enginePath){Remove-Item -LiteralPath $enginePath -Recurse -Force}
    New-Item -ItemType Directory -Path $enginePath -Force | Out-Null
    foreach($item in Get-ChildItem -LiteralPath $tmp -Force){
        Move-Item -LiteralPath $item.FullName -Destination (Join-Path $enginePath $item.Name) -Force
    }
    Remove-Item -LiteralPath $tmp -Recurse -Force
    $buildPath=Join-Path $enginePath 'BUILD.json'
    $build=Get-Content -LiteralPath $buildPath -Raw | ConvertFrom-Json
    if($build.version -ne $manifest.engine_version){throw ('moteur '+$build.version+', manifeste '+$manifest.engine_version)}
    if($build.backend -ne $engine.backend){throw 'paquet moteur au mauvais backend'}
    $build.source='prebuilt'
    [IO.File]::WriteAllText($buildPath,($build | ConvertTo-Json -Depth 6))
    [IO.File]::WriteAllText($engineMarker,$engine.sha256,[Text.Encoding]::ASCII)
}
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
$visionMode=$(if($Vision){'gpu'}else{'none'})
$arguments=@((Join-Path $source 'ajean_setup.py'),'--ajean-manifest',$manifestPath,'--setup','--yes','--no-start','--no-browser','--backend',$Backend,'--family',$Family,'--model',$Quant,'--context',[string]$Context,'--kv','int8','--vision',$visionMode,'--vram-reserve-mib',[string]$ReserveMiB,'--host','127.0.0.1','--port','18080','--data-dir',$data,'--models-dir',(Join-Path $data 'models'),'--prebuilt',($cache+[IO.Path]::DirectorySeparatorChar))
if($Backend -eq 'cuda'){$arguments+=@('--cuda',$Toolkit)}
# setup.py of 0.2.x resolves its own package: the pinned source tree is on the path
# so `from strata_control import ...` finds it whether setup is run as a script or imported.
Push-Location $source
try{
    & $python @arguments
    if($LASTEXITCODE -ne 0){throw ('Preparation MoE echouee, code '+$LASTEXITCODE)}
}finally{Pop-Location}
Write-Host 'MoE prepare dans le dossier portable. Aucun service, PATH ou SDK systeme installe.'
