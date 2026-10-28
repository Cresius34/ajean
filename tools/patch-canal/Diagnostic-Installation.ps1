# Diagnostic-Installation.ps1 — LECTURE SEULE, sans elevation. La sonde canonique.
#
# Elle repond aux questions qui reviennent : quel dossier tourne, quelle version
# est CHARGEE EN MEMOIRE (pas seulement sur disque), EmbeddingGemma 2 est-il
# l'encodeur, le moteur Strata est-il en service, le pont Ajean-Ajean existe-t-il
# dans ce binaire, le paquet est-il lisible, et l'antimalware n'a-t-il pas mange
# un de nos fichiers.
#
# Deux regles que cette sonde s'applique a elle-meme, parce qu'elle est copieuse
# chez le voisin :
#   - AUCUN chemin de dossier : les installations sont trouvees par la presence du
#     binaire a leur racine (Trouver-Installations.ps1, dot-source) ;
#   - AUCUN sha ni milesime en dur : le paquet est cherche par MOTIF et son
#     empreinte est comparee au manifeste PAQUET.txt qu'il contient. Un sha recopie
#     ici ment des le build suivant, et c'est ce qui faisait annoncer « paquet
#     absent » a tort.
$ErrorActionPreference = 'Continue'

# La sonde se sert des decouvreurs gelés a cote d'elle.
. (Join-Path $PSScriptRoot 'Trouver-Installations.ps1')

function Version-Reelle($chemin) {
  if (-not $chemin) { return '?' }
  try {
    if (-not (Test-Path -LiteralPath $chemin -PathType Leaf)) { return '?' }
    return [Diagnostics.FileVersionInfo]::GetVersionInfo($chemin).ProductVersion
  } catch { return '?' }
}

Write-Output '== 1. INSTALLATIONS TROUVEES (par binaire, jamais par nom de dossier) =='
$installations = @(Trouver-Installations)
foreach ($i in $installations) {
  if ($i.SansBinaire) {
    Write-Output ('   ' + $i.Nom + ' -> AUCUN ajean.exe a la racine (reste d''une version archivee ?)')
    continue
  }
  $attendu = Nom-Attendu -Version $i.Version
  $nom = 'ligne hors projet Cresius : pas de routine de nommage applicable'
  if ($i.EstCresius) {
    $nom = 'nom coherent'
    if ($attendu -ne $i.Nom) { $nom = 'le dossier s appelle autre chose (attendu : ' + $attendu + ')' }
  }
  Write-Output ('   ' + $i.Nom + ' -> ' + $i.Version + '  ' + $nom)
}
Write-Output ('   principale = ' + @(Trouver-Principale -Installations $installations).Count +
              '   variante = ' + @(Trouver-Variante -Installations $installations).Count)

Write-Output ''
Write-Output '== 2. CE QUI TOURNE — le processus qui tient chaque port, version CHARGEE EN MEMOIRE =='
foreach ($p in 18090, 18190, 18080, 18081) {
  $c = @(Get-NetTCPConnection -LocalPort $p -State Listen -EA SilentlyContinue)
  if (-not $c.Count) { Write-Output ('   ' + $p + ' : rien ne ecoute') ; continue }
  foreach ($id in ($c | ForEach-Object { $_.OwningProcess } | Select-Object -Unique)) {
    $w = Get-CimInstance Win32_Process -Filter ('ProcessId=' + $id) -EA SilentlyContinue
    if (-not $w) { Write-Output ('   ' + $p + ' : PID ' + $id + ' (processus invisible sans elevation)') ; continue }
    Write-Output ('   ' + $p + ' : ' + (Version-Reelle $w.ExecutablePath) + '  <-  ' + $w.ExecutablePath)
  }
}
Write-Output '   (la version du fichier ne dit pas la version en memoire : un binaire remplace'
Write-Output '    sans redemarrage tourne toujours avec son ancienne version)'

Write-Output ''
Write-Output '== 3. ENCODEUR D EMBEDDINGS — 512 dimensions = EmbeddingGemma 2, 768 = ancien Gemma =='
try {
  $s = Invoke-RestMethod 'http://127.0.0.1:18090/api/embeddings' -TimeoutSec 8
  $dims = $s.config.dimensions
  $verdict = 'encodeur NON RECONNU'
  if ($dims -eq 512) { $verdict = 'EG2 EN PLACE' } elseif ($dims -eq 768) { $verdict = 'ANCIEN GEMMA : le patch n est pas passe, ou le moteur n a pas redemarre' }
  Write-Output ('   pret=' + $s.ready + '  en service=' + $s.running + '  dims=' + $dims + '  fournisseur=' + $s.config.provider)
  Write-Output ('   modele  : ' + $s.config.model)
  Write-Output ('   => ' + $verdict)
} catch {
  Write-Output '   18090 ne repond pas : aucune instance de ce poste ne tourne (lancer Lancer-Cresius.cmd).'
}

Write-Output ''
Write-Output '== 4. MOTEUR MoE — on sonde /health, PAS /v1/models qui rend 401 alors que le moteur tourne =='
try {
  $moteur = Invoke-RestMethod 'http://127.0.0.1:18080/health' -TimeoutSec 8
  Write-Output ('   service=' + $moteur.service + '  charge=' + $moteur.loaded + '  modele=' + $moteur.model +
                '  contexte=' + $moteur.contexte_max)
} catch {
  Write-Output '   18080 : rien ne repond = moteur NON en service.'
  Write-Output '   Le pont Ajean-Ajean repond alors « llama-server injoignable » : ce n est pas un'
  Write-Output '   probleme de cle ni de reseau, c est le modele local qui n est pas demarre.'
}

Write-Output ''
Write-Output '== 5. PONT AJEAN-AJEAN — la carte de l interface n apparait QUE si /api/bridge repond 200 =='
try {
  $b = Invoke-RestMethod 'http://127.0.0.1:18090/api/bridge' -TimeoutSec 8
  Write-Output ('   CARTE ATTENDUE : actif=' + $b.enabled + '  ecoute=' + $b.listening + '  port=' + $b.port +
                '  cle=' + $b.has_key)
  Write-Output ('   ce que le pont prete : ' + $b.serving + '  modele=' + $b.model)
  Write-Output ('   ports : pont=' + $b.port + '  moteur regle=' + $b.engine_port +
                '  moteur proxifie par le pont=' + $b.engine_serving)
  # /api/bridge ne rend PAS les voisins (lu dans parallelBridgeStatus,
  # parallel_bridge.go:676-702) : ils viennent de /api/peers. Annoncer « 0 voisin »
  # sur une cle absente aurait fait cabler en double.
  try {
    $voisins = Invoke-RestMethod 'http://127.0.0.1:18090/api/peers' -TimeoutSec 8
    Write-Output ('   voisin(s) declare(s) ici : ' + @($voisins.peers).Count)
    foreach ($v in $voisins.peers) {
      Write-Output ('      ' + $v.name + ' -> ' + $v.url + '  cle=' + $v.has_key +
                    '  souvenir=' + $v.thread + ' message(s)')
    }
    if (-not @($voisins.peers).Count) {
      Write-Output '      AUCUN : cette machine peut preter son cerveau mais ne peut pas appeler.'
      Write-Output '      Cabler : .\Cabler-Canal-Local.ps1 -Voisin <IP de l autre> -Nom <nom> -Test'
    }
  } catch { Write-Output ('   /api/peers : ' + $_.Exception.Message) }
  Write-Output ('   tour d agent autorise : ' + $b.agent_on + '  (autorisation du pont) / ' +
                $b.agent_mode_local + '  (mode agent de cette machine)')
  if (-not $b.agent_on) {
    Write-Output '   => ce pont ne prete que le MODELE NU. Un tour d agent a distance demande les deux'
    Write-Output '      interrupteurs allumes des deux cotes (Reglages → Pont, ou « ajean bridge agent on »).'
  }
} catch {
  $code = 'aucune reponse'
  if ($_.Exception.Response) { $code = [string]([int]$_.Exception.Response.StatusCode) }
  Write-Output ('   /api/bridge = ' + $code)
  if ($code -eq '403') {
    Write-Output '   => le binaire EN SERVICE n est pas une version portable Cresius : le pont n existe'
    Write-Output '      pas dedans. Poser le patch.'
  } elseif ($code -eq '404') {
    Write-Output '   => binaire plus ancien que le pont : la carte ne peut PAS apparaitre, quel que soit'
    Write-Output '      le reglage. Poser le patch.'
  }
}

Write-Output ''
Write-Output '== 6. PAQUET DU CANAL — cherche par MOTIF, empreinte comparee a SON manifeste =='
foreach ($i in $installations) {
  if ($i.SansBinaire) { continue }
  $paquet = Trouver-Paquet-Canal -Dossier $i.Dossier
  if (-not $paquet) { Write-Output ('   ' + $i.Nom + ' : aucun dossier patch-canal-* avec un binaire a la racine') ; continue }
  $exe = Join-Path $paquet 'ajean.exe'
  $nomPaquet = Split-Path $paquet -Leaf
  $lu = ''
  try { $lu = (Get-FileHash -LiteralPath $exe -Algorithm SHA256).Hash } catch {
    # Pas de parentheses englobantes ici : le « ) » du nom de paquet tombait dans
    # la chaine et le parseur cherchait la parenthese fermante ailleurs.
    $msg = '   ' + $i.Nom + ' [paquet ' + $nomPaquet + '] : LECTURE REFUSEE. Le fichier est la mais bloque'
    Write-Output $msg
    Write-Output '      par l antimalware (une mise en quarantaine bloque la LECTURE). Ce n est pas un'
    Write-Output '      fichier absent : voir la section 8.'
    continue
  }
  $attenduPaquet = ''
  $man = Join-Path $paquet 'PAQUET.txt'
  if (Test-Path -LiteralPath $man) {
    foreach ($ligne in (Get-Content -LiteralPath $man)) {
      $m = [regex]::Match($ligne, 'sha256\s*:\s*([0-9A-Fa-f]{64})')
      if ($m.Success) { $attenduPaquet = $m.Groups[1].Value.ToUpper() }
    }
  }
  $verdict = 'manifeste absent : rien a comparer'
  if ($attenduPaquet) { $verdict = $(if ($attenduPaquet -eq $lu) { 'CONFORME a son manifeste' } else { 'DIVERGENT de son manifeste — paquet modifie en route' }) }
  Write-Output ('   ' + $i.Nom + ' (' + $nomPaquet + ') : ' + (Version-Reelle $exe) + '  ' +
                (Get-Item -LiteralPath $exe).Length + ' o  sha=' + $lu.Substring(0,16) + '  ' + $verdict)
}

Write-Output ''
Write-Output '== 7. OUTILS DU CANAL dans data\scripts =='
$attendus = @('cabler-canal.ps1', 'Ouvrir-Canal.ps1', 'Ouvrir-PareFeu-Canal.cmd',
              'Verifier-Outils-Canal.ps1', 'Verifier-Collisions-Casse.ps1', 'Encodages.ps1',
              'Trouver-Installations.ps1', 'Cabler-Canal-Local.ps1', 'Diagnostic-Installation.ps1')
foreach ($i in $installations) {
  if ($i.SansBinaire) { continue }
  $dos = Join-Path $i.Dossier 'data\scripts'
  if (-not (Test-Path -LiteralPath $dos)) { Write-Output ('   ' + $i.Nom + ' : pas de data\scripts') ; continue }
  $absents = @($attendus | Where-Object { -not (Test-Path -LiteralPath (Join-Path $dos $_)) })
  Write-Output ('   ' + $i.Nom + ' -> ' + ($attendus.Count - $absents.Count) + ' / ' + $attendus.Count +
                $(if ($absents.Count) { '   absents : ' + ($absents -join ', ') } else { '' }))
}

Write-Output ''
Write-Output '== 8. DETECTIONS ANTIVIRUS sur nos fichiers (une quarantaine bloque la LECTURE) =='
try {
  $dets = @(Get-MpThreatDetection -EA SilentlyContinue | Where-Object { $_.Resources -match 'ajean' } |
            Sort-Object InitialDetectionTime -Descending | Select-Object -First 5)
  if (-not $dets.Count) { Write-Output '   aucune detection ne concerne un fichier ajean' }
  foreach ($d in $dets) {
    $res = @($d.Resources) -join ' ; '
    if ($res.Length -gt 150) { $res = $res.Substring(0, 150) }
    Write-Output ('   ' + $d.InitialDetectionTime + '  ' + $res)
  }
  Write-Output '   Si un de nos binaires est cite : le poser de nouveau apres exclusion, sinon le'
  Write-Output '   poseur ne peut pas le hacher et le patch echoue sans dire pourquoi.'
} catch {
  Write-Output ('   journal antimalware inaccessible sans elevation : ' + $_.Exception.Message)
}
Write-Output ''
Write-Output 'FIN — aucune ecriture, aucun arret, aucun telechargement.'
