# Declare (ou retire) le voisin du canal, sans rien d'autre.
#
# Utile quand le pont est DEJA ouvert et qu'il ne manque que l'adresse d'en face.
# Ouvrir-Canal.ps1 fait tout d'un coup ; ici on ne touche ni au pare-feu, ni au
# pont, ni au binaire.
#
# La cle n'est jamais ecrite dans un fichier, jamais affichee : elle est demandee
# au clavier avec Read-Host -AsSecureString, puis convertie en clair seulement
# pour le corps de la requete POST /api/peers, qui l'envoie a L'INSTANCE LOCALE
# (127.0.0.1:18090). C'est elle qui la range dans sa base — la seule place ou une
# cle doit vivre.
#
# Usage :
#   .\cabler-canal.ps1 -Voisin 192.168.1.99            # nom deduit, cle demandee
#   .\cabler-canal.ps1 -Voisin 192.168.1.99 -Nom poste-salon -Port 18190
#   .\cabler-canal.ps1 -Retirer poste-salon
#   .\cabler-canal.ps1 -Liste
#
# Attention au sens : la cle demandee est celle de la machine APPELEE (celle qui
# prete son cerveau), pas celle d'ici. Deux machines qui se pretent reciproquement
# ont donc DEUX declations, chacune avec la cle de l'autre.

param(
  [string]$Voisin = '',
  [string]$Nom   = '',
  [string]$Cle   = '',
  [int]$Port     = 18190,
  [string]$UI    = 'http://127.0.0.1:18090',
  [string]$Retirer = '',
  [switch]$Liste
)

$ErrorActionPreference = 'Stop'
$RulePrefix = 'AJEAN canal (port '   # meme nom que dans Ouvrir-Canal.ps1

function Note($m) { Write-Output $m }

function CleMasquee([string]$Libelle) {
  # Read-Host -AsSecureString ne laisse rien passer a l'ecran ni dans l'historique.
  $s = Read-Host -Prompt $Libelle -AsSecureString
  $pt = [Runtime.InteropServices.Marshal]::SecureStringToGlobalAllocUnicode($s)
  try { return [Runtime.InteropServices.Marshal]::PtrToStringUni($pt) }
  finally { [Runtime.InteropServices.Marshal]::ZeroFreeGlobalAllocUnicode($pt) }
}

if ($Liste -or -not $Retirer -and -not $Voisin) {
  try {
    $p = Invoke-RestMethod -Uri ($UI + '/api/peers') -TimeoutSec 8
    Note ("voisins declares : " + (@($p.peers).Count))
    foreach ($v in $p.peers) {
      Note ("   " + $v.name + " -> " + $v.url + " · cle=" + $v.has_key + " · modele « " + $v.model + " » · canal " + $v.thread + " messages")
    }
    if (@($p.peers).Count -eq 0) { Note "   aucun. C'est normal sur une machine qui ne fait que preter." }
  } catch { Note ("(!) /api/peers : " + $_.Exception.Message) ; exit 1 }
  exit 0
}

if ($Retirer) {
  $corps = @{ remove = $Retirer } | ConvertTo-Json -Compress
  try {
    $r = Invoke-RestMethod -Uri ($UI + '/api/peers') -Method Post -Body $corps -ContentType 'application/json' -TimeoutSec 15
    Note ("voisin retire : " + $r.removed + " (son souvenir du canal part avec lui)")
  } catch { Note ("(!) retrait de « $Retirer » : " + $_.Exception.Message) ; exit 1 }
  exit 0
}

# --- declaration ---
# L'URL est recollée comme le fait le binaire : sans schema, http:// ; sans /v1,
# /v1. Une URL mal saisie donne un voisin qui echoue a chaque demande, avec un
# diagnostic qui ressemble a un pare-feu. Mieux vaut la construire ici.
$hote = $Voisin.Trim()
if ($hote -match '^https?://') { $base = $hote } else { $base = "http://$hote" }
$base = $base.TrimEnd('/')
if ($base -notmatch ':\d+') { $base = ($base -replace ('(//[^:/]+)'), ('$1:' + $Port)) }
if ($base -notmatch '/v1$') { $base = $base + '/v1' }
if (-not $Nom) {
  # Un nom deduit, pas un numero : « 192.168.1.99 » se lit mal dans un log et
  # change si le DHCP bouge.
  $Nom = 'poste-' + (([uri]$base).Host -replace '\.', '-')
}

$etat = $null
try { $etat = Invoke-RestMethod -Uri ($UI + '/api/peers') -TimeoutSec 8 } catch {
  Note ("(!) $UI/api/peers ne repond pas : " + $_.Exception.Message)
  Note "    L'instance de ce dossier doit tourner (Lancer-Cresius.cmd)."
  exit 1
}
$deja = @($etat.peers | Where-Object { $_.name -eq $Nom })
if ($deja.Count) {
  Note "   « $Nom » est deja declare sur $($deja[0].url). Il sera remplace (memoire du canal perdue)."
}

if (-not $Cle) {
  Note ""
  Note "   La cle demandee est celle de la machine qui PRETE (celle qu'on appelle : $base)."
  Note "   Elle se lit la-bas : interface → Réglages → clé API (ou « ajean set-api-key »)."
  $Cle = CleMasquee ("Cle API de $Nom (saisie masquee)")
}
if (-not $Cle) { Note "   (!) cle vide : le pont d'en face refusera chaque demande (401). Abandon." ; exit 1 }

$corps = @{ add = $Nom ; url = $base ; key = $Cle } | ConvertTo-Json -Compress
try {
  $a = Invoke-RestMethod -Uri ($UI + '/api/peers') -Method Post -Body $corps -ContentType 'application/json' -TimeoutSec 30
  Note ("voisin declare : " + $a.name + " -> " + $a.url)
} catch {
  Note ("(!) POST /api/peers : " + $_.Exception.Message)
  exit 1
}
$Cle = $null   # plus besoin de la cle en clair en memoire

try {
  $p = Invoke-RestMethod -Uri ($UI + '/api/peers') -TimeoutSec 10
  foreach ($v in $p.peers) { Note ("   " + $v.name + " -> " + $v.url + " · cle=" + $v.has_key) }
} catch { }

Note ""
Note "Pour essayer tout de suite (le canal se joue en CLI, pas par l'API web) :"
Note "   .\ajean.exe peer status $Nom"
Note "   .\ajean.exe peer ask $Nom `"Dis le nom du modele que tu es.`""
Note "Le pont d'ici doit etre allume pour que l'autre puisse rappeler : Ouvrir-Canal.ps1 -Verifier."
