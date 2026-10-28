# Diagnostic-Bekka.ps1 — etat de l'installation, LECTURE SEULE.
#
# Ecrit le 2026-10-28 pour le point demande par l'utilisateur : verifier chez la
# voisine (1) EmbeddingGemma 2, (2) le moteur Strata. Ce script ne doit RIEN
# modifier : ni arret de service, ni ecriture, ni telechargement. Il rend du JSON
# pour que le poste appelant puisse le lire sans deviner.
#
# Pourquoi ce script existe : le voisin a repondu « NON VERIFIE » a chaque question,
# a juste titre — son tour d'agent est eteint, il n'a donc aucun outil et ne peut
# pas lire son propre disque. Sans ce script (ou sans ouvrir son tour d'agent),
# l'autre poste ne peut que supposer. Mesure de l'exterieur faite ici : seuls les
# ports 18090/18080/18081 ne repondent pas depuis le reseau (filtres), le pont
# (18190) est le seul point d'entree — il ne renseigne pas sur le moteur.
#
# Premiere mesure utile, sans rien toucher : QUEL dossier tourne. Le modele prete
# par son pont est un gguf dont le CHEMIN contient « Ajean v18.11.1 maj Cresius
# RX6900XT ». Si le dossier qui tourne chez elle est v18.11.1 et que le patch a ete
# depose dans un dossier v18.15.1, le patch n'a jamais concerne l'instance en
# service — ce qui rend exactement le « le patch ne s'applique pas ».
$ErrorActionPreference = 'Continue'

function Sha-Court($chemin) {
  # L'antivirus peut refuser la LECTURE d'un binaire qu'il a classe (mesure du
  # 2026-10-28 ici : Get-FileHash rend « le fichier contient un virus »). Le
  # signaler vaut mieux qu'un faux « absent » ou qu'un hash invente.
  if (-not (Test-Path -LiteralPath $chemin -PathType Leaf)) { return 'ABSENT' }
  try { return (Get-FileHash -LiteralPath $chemin -Algorithm SHA256 -ErrorAction Stop).Hash }
  catch { return 'LECTURE_REFUSEE:' + $_.Exception.Message.Trim() }
}

function Ver($chemin) {
  if (-not (Test-Path -LiteralPath $chemin -PathType Leaf)) { return $null }
  $v = [Diagnostics.FileVersionInfo]::GetVersionInfo($chemin)
  $i = Get-Item -LiteralPath $chemin
  return [ordered]@{ produit = $v.ProductVersion ; fichier = $v.FileVersion ;
                     taille = $i.Length ; modifie = $i.LastWriteTime.ToString('s') }
}

$racines = @(Get-ChildItem -LiteralPath 'C:\ProgramData' -Directory -Filter 'Ajean*' -EA SilentlyContinue |
             Sort-Object Name | ForEach-Object { $_.FullName })

$installations = @()
foreach ($r in $racines) {
  $exe = Join-Path $r 'ajean.exe'
  $cmd = Join-Path $r 'Lancer-Cresius.cmd'
  $nomAppele = $null
  if (Test-Path -LiteralPath $cmd -PathType Leaf) {
    $m = [regex]::Match([IO.File]::ReadAllText($cmd), '-File\s+"?%~dp0([^"\s]+)')
    if ($m.Success) { $nomAppele = $m.Groups[1].Value }
  }
  $outils = @('cabler-canal.ps1','Ouvrir-Canal.ps1','Ouvrir-PareFeu-Canal.cmd',
              'Verifier-Outils-Canal.ps1','Verifier-Collisions-Casse.ps1','Encodages.ps1')
  $poses = @() ; $manquants = @()
  foreach ($o in $outils) {
    if (Test-Path -LiteralPath (Join-Path $r ('data\scripts\' + $o))) { $poses += $o } else { $manquants += $o }
  }
  $installations += [ordered]@{
    dossier   = $r
    exe       = Ver $exe
    sha_exe   = Sha-Court $exe
    cmd_appelle = $nomAppele
    lanceur_appelable = $(if ($nomAppele) { Test-Path -LiteralPath (Join-Path $r $nomAppele) } else { $false })
    verrous_lanceurs = @(Get-ChildItem -LiteralPath $r -Filter 'Lancer-Cresius*.ps1' -File -EA SilentlyContinue |
        ForEach-Object { $t = [IO.File]::ReadAllText($_.FullName)
                         $mv = [regex]::Match($t, 'expectedVersion\s*=\s*''([^'']+)''')
                         [ordered]@{ fichier = $_.Name ; verrou = $mv.Groups[1].Value } })
    paquet_18161 = (Test-Path -LiteralPath (Join-Path $r 'patch-canal-18161\ajean.exe'))
    sha_paquet  = $(if (Test-Path -LiteralPath (Join-Path $r 'patch-canal-18161\ajean.exe')) { Sha-Court (Join-Path $r 'patch-canal-18161\ajean.exe') } else { $null })
    outils_poses   = $poses.Count
    outils_absents = $manquants
    journaux_patch = @(Get-ChildItem -LiteralPath (Join-Path $r 'data\scripts') -Filter 'Patch-canal-*.log' -File -EA SilentlyContinue |
                       Sort-Object LastWriteTime -Descending | ForEach-Object { $_.Name + ' ' + $_.LastWriteTime.ToString('s') })
    archives = @(Get-ChildItem -LiteralPath $r -Directory -Filter 'archive-*' -EA SilentlyContinue | ForEach-Object { $_.Name })
  }
}

# --- moteur MoE Strata : ce qui est installe, pas ce qui tourne ---
$moteurs = @(Get-ChildItem -LiteralPath 'C:\ProgramData' -Directory -Filter 'Ajean*' -EA SilentlyContinue |
  ForEach-Object { $m = Join-Path $_.FullName 'data\moe'
                   if (Test-Path -LiteralPath $m) {
                     Get-ChildItem -LiteralPath $m -Directory -EA SilentlyContinue | ForEach-Object {
                       [ordered]@{ dossier = $_.FullName ; installation = $_.Parent.Parent.Name } } } })

# --- ce que les journaux disent du moteur et de Gyro-S ---
$logs = @()
foreach ($r in $racines) {
  $ld = Join-Path $r 'data\logs'
  if (-not (Test-Path -LiteralPath $ld)) { continue }
  foreach ($f in (Get-ChildItem -LiteralPath $ld -File -EA SilentlyContinue |
                  Where-Object { $_.Name -match 'strata|gyro|moe|engine' } |
                  Sort-Object LastWriteTime -Descending | Select-Object -First 6)) {
    $queue = @()
    try { $queue = @(Get-Content -LiteralPath $f.FullName -Tail 12 -Encoding UTF8 -EA SilentlyContinue) } catch {}
    $logs += [ordered]@{ fichier = $f.FullName ; modifie = $f.LastWriteTime.ToString('s') ;
                         taille = $f.Length ; dernieres_lignes = $queue }
  }
}

# --- EmbeddingGemma 2 : le modele est-il la, et quelle dimension s'affiche ? ---
$modeles = @()
foreach ($r in $racines) {
  $md = Join-Path $r 'data\models'
  if (-not (Test-Path -LiteralPath $md)) { continue }
  foreach ($f in (Get-ChildItem -LiteralPath $md -Recurse -File -EA SilentlyContinue |
                  Where-Object { $_.Name -match 'gemma|embed' })) {
    $modeles += [ordered]@{ fichier = $f.FullName ; taille = $f.Length ; modifie = $f.LastWriteTime.ToString('s') }
  }
}

# --- ce qui ecoute LOCALEMENT chez ce poste (18080 moteur, 18081 encodeur, 18090 interface) ---
$ecoutes = [ordered]@{}
foreach ($p in @(18080, 18081, 18090, 18190)) {
  $c = @(Get-NetTCPConnection -LocalPort $p -State Listen -EA SilentlyContinue)
  if (-not $c.Count) { $ecoutes[[string]$p] = 'rien n''ecoute' ; continue }
  $ids = $c | ForEach-Object { $_.OwningProcess } | Select-Object -Unique
  $detail = @()
  foreach ($id in $ids) {
    $w = Get-CimInstance Win32_Process -Filter "ProcessId=$id" -EA SilentlyContinue
    $detail += ('PID ' + $id + ' ' + $(if ($w) { $w.Name + ' | ' + $w.ExecutablePath } else { '?' }))
  }
  $ecoutes[[string]$p] = $detail
}

# --- ce que l'interface annonce reellement (encodeur, dimensions) ---
$service = $null
try { $service = Invoke-RestMethod -Uri 'http://127.0.0.1:18090/api/embeddings' -TimeoutSec 8 }
catch { $service = [ordered]@{ erreur = $_.Exception.Message } }

# --- l'antivirus a-t-il frappe ces fichiers ? ---
$av = @()
try {
  $av = @(Get-WinEvent -LogName 'Microsoft-Windows-Windows Defender/Operational' -MaxEvents 200 -EA Stop |
    Where-Object { $_.Id -in 1116,1117,1119 } | Select-Object -First 8 |
    ForEach-Object {
      [ordered]@{ date = $_.TimeCreated.ToString('s') ; id = $_.Id ;
                  nom = [regex]::Match($_.Message, 'Nom\s*:\s*(.+)').Groups[1].Value.Trim() ;
                  chemin = [regex]::Match($_.Message, 'Chemin\s*:\s*([^;]+)').Groups[1].Value.Trim() } })
} catch { $av = @([ordered]@{ erreur = $_.Exception.Message }) }

[ordered]@{
  ok = $true
  poste = [ordered]@{ nom = $env:COMPUTERNAME ; ip = (@(Get-NetIPAddress -AddressFamily IPv4 -EA SilentlyContinue |
              Where-Object { $_.IPAddress -like '192.168.*' -or $_.IPAddress -like '10.*' } |
              ForEach-Object { $_.IPAddress }) -join ',') }
  installations = $installations
  moteurs_installes = $moteurs
  modeles_embeddings = $modeles
  ecoutes_locales = $ecoutes
  service_embeddings = $service
  journaux_moteur = $logs
  antivirus = $av
} | ConvertTo-Json -Depth 6 -Compress
