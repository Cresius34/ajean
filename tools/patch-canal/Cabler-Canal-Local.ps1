# Cabler-Canal-Local.ps1 — l'AUTRE moitié du canal Ajean↔Ajean.
#
# Pourquoi ce script existe : le canal ne se branche pas tout seul. Il faut une
# declaration SUR CHAQUE machine, et chacune declare la machine qu'elle APPELLE,
# avec la cle de la machine QUI PRETE. Mesure du 2026-10-27 : ce poste appelle la
# machine voisine (souvenir commun de 12 messages), mais la voisine ne peut pas
# rappeler — chez elle aucun voisin n'est declare, donc l'outil « peer » n'est
# meme pas annonce au modele (llm_client.go : parallelBuild() && peerConfigured()).
#
# Ce que fait ce script, sur LA machine ou on le joue :
#   1. il lit SA PROPRE cle API depuis son interface locale (127.0.0.1:18090) :
#      jamais demandee a la main, jamais ecrite sur disque, jamais affichee ;
#   2. il declare le voisin pointe sur le PONT de l'autre machine (port 18190,
#      JAMAIS 18080 — le port du moteur ne sort pas sur le LAN en version
#      portable, et c'est l'erreur de saisie la plus frequente) ;
#   3. il relit /api/peers pour verifier que la cle est bien posee ;
#   4. avec -Test, il joue « peer status <nom> » en se servant de l'installation
#      TROUVEE par le binaire (aucun chemin de dossier ecrit ici).
#
# Usage, sur la machine ou l'on veut pouvoir appeler l'autre :
#   .\Cabler-Canal-Local.ps1 -Voisin 192.168.1.71 -Nom cresius
#   .\Cabler-Canal-Local.ps1 -Voisin 192.168.1.71 -Nom cresius -Test
#   .\Cabler-Canal-Local.ps1 -Liste
#   .\Cabler-Canal-Local.ps1 -Retirer cresius
#
# La cle utilisee est celLE DE CETTE MACHINE : c'est elle que le pont de CETTE
# machine doit accepter quand l'autre l'appelle. Mesure du 2026-10-28 : les deux
# postes ont la MEME cle (200 avec la mienne, 401 sans cle, 401 avec une cle
# volontairement fausse). La cle n'est donc pas ce qui empechait le retour :
# c'est l'absence de declaration du cote appele.
#
# Regle d'ecriture de ce dossier : pas d'apostrophe dans une chaine delimitee par
# des apostrophes (PowerShell 5.1 casse le parseur, et l'erreur pointe ailleurs).

param(
  [string]$Voisin = '',
  [string]$Nom    = '',
  [int]$PortPont  = 18190,
  [string]$UI     = 'http://127.0.0.1:18090',
  [string]$Retirer = '',
  [switch]$Liste,
  [switch]$Test
)
$ErrorActionPreference = 'Stop'

function Note($m) { Write-Output $m }

function Empreinte([string]$s) {
  (Get-FileHash -InputStream ([IO.MemoryStream]::new([Text.Encoding]::UTF8.GetBytes($s))) -Algorithm SHA256).Hash.Substring(0, 12)
}

# --- etat des lieux -------------------------------------------------------
if ($Liste -or (-not $Retirer -and -not $Voisin)) {
  try {
    $p = Invoke-RestMethod -Uri ($UI + '/api/peers') -TimeoutSec 8
    Note ('voisins declares ici : ' + @($p.peers).Count)
    foreach ($v in $p.peers) {
      Note ('   ' + $v.name + ' -> ' + $v.url + '  cle=' + $v.has_key +
            '  souvenir=' + $v.thread + ' message(s)')
    }
    if (@($p.peers).Count -eq 0) {
      Note '   AUCUN. Cette machine peut preter son cerveau mais ne peut pas appeler : l outil peer ne lui est meme pas annonce.'
      Note ('   Pour cabler : .' + (Split-Path $MyInvocation.MyCommand.Path -Leaf) +
            ' -Voisin <IP-de-l autre-machine> -Nom <nom>')
    }
  } catch {
    Note ('(!) ' + $UI + '/api/peers ne repond pas : ' + $_.Exception.Message)
    Note '    L instance de ce poste doit tourner (Lancer-Cresius.cmd).'
    exit 1
  }
  exit 0
}

# --- retrait --------------------------------------------------------------
if ($Retirer) {
  $corps = @{ remove = $Retirer } | ConvertTo-Json -Compress
  try {
    $r = Invoke-RestMethod -Uri ($UI + '/api/peers') -Method Post -Body $corps -ContentType 'application/json' -TimeoutSec 8
    Note ('voisin retire : ' + $r.removed + '  (son souvenir du canal part avec lui)')
  } catch { Note ('(!) retrait de « ' + $Retirer + ' » : ' + $_.Exception.Message) ; exit 1 }
  exit 0
}

# --- URL du voisin : recollée comme le fait le binaire -------------------
# Sans schema -> http:// ; sans port -> le port du pont ; sans /v1 -> /v1. Une
# URL mal saisie produit un voisin qui echoue a chaque demande, avec un
# diagnostic qui ressemble a un pare-feu.
$hote = $Voisin.Trim()
if ($hote -match '^https?://') { $base = $hote } else { $base = 'http://' + $hote }
$base = $base.TrimEnd('/')
if ($base -notmatch ':\d+') { $base = $base -replace '(//[^:/]+)', ('$1:' + $PortPont) }
if ($base -notmatch '/v1$') { $base = $base + '/v1' }

if ($base -match ':18080') {
  Note '(!) L URL vise le port 18080 (le moteur). En version portable le moteur ecoute sur'
  Note '    127.0.0.1 seulement : ce voisin ne repondra jamais. Le seul port qui sort sur le'
  Note '    reseau est celui du pont, 18190. Refuse — corriger l URL ou passer -PortPont 18190.'
  exit 2
}

if (-not $Nom) {
  # Un nom deduit, pas un numero nu : « 192.168.1.71 » se lit mal dans un log.
  $Nom = 'poste-' + (([uri]$base).Host -replace '\.', '-')
}

# --- ma propre cle API, lue localement ----------------------------------
# Jamais demandee a la main (une cle tape dans l historique), jamais ecrite dans
# un fichier. GET /api/apikey la rend en clair, mais seulement sur 127.0.0.1 :
# ce port n est pas joignable depuis le LAN ici, c est ce qui contient la fuite.
$cleLocale = ''
try {
  $k = Invoke-RestMethod -Uri ($UI + '/api/apikey') -TimeoutSec 8
  if ($k.set -and $k.key) { $cleLocale = [string]$k.key }
} catch {
  Note ('(!) lecture de ma cle API : ' + $_.Exception.Message)
}
if (-not $cleLocale) {
  Note '(!) MA cle API est introuvable sur ce poste. Le pont local refuse de demarrer sans cle,'
  Note '    et le pont d en face refusera toute demande sans la bonne cle. La cle a declarer chez'
  Note '    le voisin est celLE DE CE POSTE (interface → Reglages → cle API).'
  exit 1
}
Note ('ma cle API    : posee, ' + $cleLocale.Length + ' caracteres, empreinte ' + (Empreinte $cleLocale) + ' (jamais affichee)')

# --- declaration --------------------------------------------------------
$etat = $null
try { $etat = Invoke-RestMethod -Uri ($UI + '/api/peers') -TimeoutSec 8 } catch {
  Note ('(!) ' + $UI + '/api/peers ne repond pas : ' + $_.Exception.Message) ; exit 1
}
$deja = @($etat.peers | Where-Object { $_.name -eq $Nom })
if ($deja.Count) {
  Note ('   « ' + $Nom + ' » est deja declare sur ' + $deja[0].url + '. Il sera remplace (le souvenir du canal part avec lui).')
}

$corps = (@{ add = $Nom; url = $base; key = $cleLocale } | ConvertTo-Json -Compress)
try {
  $r = Invoke-RestMethod -Uri ($UI + '/api/peers') -Method Post -Body $corps -ContentType 'application/json' -TimeoutSec 10
  Note ('   declare     : ' + $r.name + ' -> ' + $r.url)
} catch {
  Note ('(!) declaration refusee : ' + $_.Exception.Message)
  exit 1
}

# --- relecture : une declaration annoncee sans relecture est un mensonge --
$apres = Invoke-RestMethod -Uri ($UI + '/api/peers') -TimeoutSec 8
$moi = @($apres.peers | Where-Object { $_.name -eq $Nom })
if (-not $moi.Count) { Note '   (!) RELIRE : le voisin n apparait pas dans /api/peers apres l envoi.' ; exit 1 }
Note ('   verifie     : ' + $moi[0].url + '  cle=' + $moi[0].has_key)
if (-not $moi[0].has_key) {
  Note '   (!) cle NON posee cote local : l appel partira sans en-tete et le pont d en face repondra 401.'
  exit 1
}

# --- le pont de CE poste est-il pret a etre rappele ? -------------------
try {
  $b = Invoke-RestMethod -Uri ($UI + '/api/bridge') -TimeoutSec 8
  Note ('   mon pont    : actif=' + $b.enabled + '  ecoute=' + $b.listening + '  port=' + $b.port +
        '  cle=' + $b.has_key + '  pret=' + $b.serving)
  if (-not $b.enabled -or -not $b.listening) {
    Note '   (!) mon pont n est pas ouvert : l autre machine ne pourra pas me joindre, meme bien cablee.'
    Note '       Reglages → carte « Pont Ajean ↔ Ajean » → activer. (Carte absente du menu = binaire anterieur au pont.)'
  }
  if ($b.agent_on) {
    Note '   agent       : AUTORISE — un voisin peut jouer un tour d agent sur CETTE machine (shell, disque, memoire).'
  } else {
    Note '   agent       : non autorise — ce pont ne prete que le modele nu.'
  }
} catch { Note ('(!) /api/bridge : ' + $_.Exception.Message) }

# --- test reel, optionnel ----------------------------------------------
if ($Test) {
  . (Join-Path $PSScriptRoot 'Trouver-Installations.ps1')
  $inst = (Trouver-Principale).Dossier
  if (-not $inst) { $inst = (Trouver-Variante).Dossier }
  if (-not $inst) {
    Note '   test : installation introuvable depuis ce script (ajean.exe attendu a sa racine).'
    exit 1
  }
  $exe = Join-Path $inst 'ajean.exe'
  $env:AJEAN_HOME = Join-Path $inst 'data'
  Note ''
  Note ('   test avec   : ' + $exe + '  (' + [Diagnostics.FileVersionInfo]::GetVersionInfo($exe).ProductVersion + ')')
  $out = & $exe peer status $Nom 2>&1
  foreach ($l in $out) { Note ('     ' + $l) }
  Note ''
  Note '   Si la reponse dit « outil absent » ou « cle refusee », voir NOTE-DE-PATCH.md (section canal).'
}
