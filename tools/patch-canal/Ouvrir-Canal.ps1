# Ouvre le canal Ajean-Ajean sur CETTE machine : pare-feu, pont, voisin, preuve.
#
# A lancer sur chaque machine du canal (la principale et celle de la RX 6900 XT),
# une fois le patch 0.18.15.5 Cresius applique dans le dossier qui contient ce
# script. Il ne fait que ce que le binaire sait deja faire ; il ne patche rien.
#
# Ce qu'il fait, dans l'ordre :
#   1. verifie que le binaire du dossier annonce bien 0.18.15.5 Cresius (le pont
#      n'existe pas avant 0.18.15.2 : /api/bridge repond 404 sur un 0.18.15.1) ;
#   2. verifie la cle API machine — sans elle le pont refuse de demarrer ;
#   3. pose une regle de pare-feu ENTRANTE sur le port du pont ;
#   4. allume le pont (POST /api/bridge) — a chaud, sans redemarrer le moteur ;
#   5. prouve que le pont sert reelement : /v1/models puis une completion, avec
#      et SANS l'en-tete du canal (la seconde est la preuve de la revision 3) ;
#   6. declare le voisin (POST /api/peers) et le sonde.
#
# Deux pieges que ce script evite, et pourquoi ils existent :
#   - le binaire SUPPRIME sa propre regle en coupant le pont : stopParallelBridge
#     appelle firewallClose(), qui efface « AJEAN moteur (port 18190) ». Une regle
#     portee ce nom disparaitrait a chaque coupure. Ici la regle s'appelle
#     « AJEAN canal (port ...) » : elle lui survit.
#   - la cle n'est JAMAIS ecrite dans un fichier de ce dossier : elle passe en
#     parametre ou par la variable AJEAN_CLE_CANAL, et finit dans la base de
#     l'installation (la seule place ou elle doit vivre).
#
# Usage (sur la machine qui doit preter ET parler) :
#   .\Ouvrir-Canal.ps1 -VoisinNom ajean-voisin -VoisinUrl http://192.168.1.98:18190/v1
#   .\Ouvrir-Canal.ps1 -Statut            # lit l'etat, n'ecrit rien
#   .\Ouvrir-Canal.ps1 -Verifier          # pare-feu + pont + /v1, sans voisin
#   cle : -Cle sk-... ou $env:AJEAN_CLE_CANAL (la cle de la MACHINE VOISINE)

param(
  [string]$VoisinNom = '',
  [string]$VoisinUrl = '',
  [string]$Cle       = '',
  [int]$Port         = 18190,
  [string]$UI        = 'http://127.0.0.1:18090',
  [switch]$Statut,
  [switch]$Verifier,
  [switch]$SansPareFeu
)

$ErrorActionPreference = 'Stop'
# Le pont existe depuis 0.18.15.2 ; la révision 3 y ajoute la carte d'identité
# envoyée au modèle prêté. On accepte tout ce qui a le pont, mais ce qui est
# sous la révision 5 est signalé : de ce côté-là le modèle d'en face ne sait pas
# qu'il est prêté et invente (« je tourne sur une infrastructure cloud »).
#
# COMPARAISON NUMÉRIQUE, plus de liste de versions recopiée. FAUTIF MESURÉ le
# 2026-10-28 : `$Accepte` s'arrêtait à 0.18.15.5, donc ce script REFUSAIT le
# binaire que le paquet du même nom livrait (0.18.16.1 Cresius) — il disait
# « le pont n'existe qu'à partir de 0.18.15.2, lancer d'abord le patch » alors
# que le patch était posé, devant lui. Une liste en dur rattrape toujours le
# paquet par surprise le lendemain. On compare des nombres.
$VeutRev = 5                 # révision du canal attendue (carte d'identité + auteur)
$PontDepuis = '0.18.15.2'    # première version où /api/bridge existe
# La carte d'identité du modèle prêté est arrivée avec la révision 5 du canal,
# c'est-à-dire 0.18.15.5. Le millésime Cresius est 0.18.<amont>.<rev> : un amont
# plus haut (0.18.16.1) passe donc aussi la borne, et [Version] le compare
# composant par composant — 0.18.16.1 > 0.18.15.5. Une seule borne basse suffit ;
# c'est justement ce que la liste en dur ne savait pas faire.
$CarteDepuis = '0.18.15.5'
$RulePrefix = 'AJEAN canal (port '   # != « AJEAN moteur (port ...) », que le binaire efface

function Note($m) { Write-Output $m }
function NomRegle($p) { return ($RulePrefix + $p + ')') }

# ---------- 0. ce dossier ----------
$Racine = Split-Path -Parent $PSScriptRoot
$exe = Join-Path $Racine 'ajean.exe'
Note "== dossier : $Racine"
if (-not (Test-Path -LiteralPath $exe)) {
  Note "   (!) pas d'ajean.exe a cote de ce dossier-canal : ce script doit vivre DANS l'installation a ouvrir."
  exit 1
}
$disque = [Diagnostics.FileVersionInfo]::GetVersionInfo((Resolve-Path -LiteralPath $exe).Path).ProductVersion
Note "   binaire du dossier : $disque"
# La version lue porte un suffixe (« 0.18.16.1 Cresius ») : on n'en compare que la
# partie numérique. [version] compare les composants un par un, donc 0.18.16.1 est
# bien au-dessus de 0.18.15.5 — ce qu'une liste en dur ratait.
function NumVersion([string]$s) {
  $m = [regex]::Match($s, '(\d+)\.(\d+)\.(\d+)(?:\.(\d+))?')
  if (-not $m.Success) { return $null }
  $r = '{0}.{1}.{2}.{3}' -f [int]$m.Groups[1].Value, [int]$m.Groups[2].Value,
                          [int]$m.Groups[3].Value, $(if ($m.Groups[4].Success) { [int]$m.Groups[4].Value } else { 0 })
  try { return [Version]$r } catch { return $null }
}
$nuDisque = NumVersion $disque
if (-not $nuDisque) {
  Note "   REFUSE : version illegible (« $disque ») — impossible de savoir si le pont y est."
  exit 1
}
if ($nuDisque -lt (NumVersion $PontDepuis)) {
  Note ("   REFUSE : le pont n'existe qu'a partir de " + $PontDepuis + " (sur un 0.18.15.1, /api/bridge repond 404). Lancer d'abord Appliquer-Patch-Canal.ps1.")
  exit 1
}
if ($nuDisque -lt (NumVersion $CarteDepuis)) {
  Note "   (!) ce binaire est en $disque, sous $CarteDepuis : le modele prete par ce pont ne recevra AUCUNE"
  Note "       carte d'identite. Il repondra a « as-tu un terminal ? » par une invention (mesure du"
  Note "       2026-10-27 : « je tourne sur une infrastructure cloud »). Appliquer le patch du paquet."
}

# ---------- 1. etat lu par l'instance en cours ----------
$etatPont = $null
try { $etatPont = Invoke-RestMethod -Uri ($UI + '/api/bridge') -TimeoutSec 6 } catch {
  Note "   REFUSE : $UI/api/bridge ne repond pas (" + $_.Exception.Message + ")."
  Note "          Lancer Lancer-Cresius.cmd, puis relancer ce script. Une reponse 404 = binaire non patche."
  exit 1
}
Note "   pont tel qu'il est : demande=$($etatPont.requested) actif=$($etatPont.enabled) ecoute=$($etatPont.listening) port=$($etatPont.port) cle=$($etatPont.has_key)"
Note "   prete : $($etatPont.serving) · preset « $($etatPont.preset) » · modele : $($etatPont.model)"

if ($Statut) {
  try {
    $p = Invoke-RestMethod -Uri ($UI + '/api/peers') -TimeoutSec 6
    Note "   voisins : " + (@($p.peers).Count)
    foreach ($v in $p.peers) { Note ("      " + $v.name + " -> " + $v.url + " · cle=" + $v.has_key + " · canal " + $v.thread + " messages") }
  } catch { Note "   (!) /api/peers : " + $_.Exception.Message }
  exit 0
}

# ---------- 2. la cle ----------
if (-not $Cle) { $Cle = [string]$env:AJEAN_CLE_CANAL }
if (-not $etatPont.has_key) {
  Note "   REFUSE : cette machine n'a pas de cle API machine. Le pont repond 503 sans elle (antKeyOK n'admet personne)."
  Note "          La definir dans l'interface (page Reglages, section cle API), PUIS relancer ce script."
  Note "          Ne pas passer par « ajean set-api-key » sur une instance en service : la commande"
  Note "          attend une reponse sur stdin et redemarre le moteur."
  exit 1
}
if (-not $Cle -and -not $Verifier) {
  Note "   (!) cle du voisin non fournie : le pont sera ouvert, mais le voisin ne pourra pas etre ajoute."
  Note "       -Cle sk-... ou `$env:AJEAN_CLE_CANAL (c'est la cle de la MACHINE VOISINE, pas la tienne)."
}

# ---------- 3. pare-feu ----------
$rule = NomRegle $Port
# Deux pieges mesures sur ce poste :
#  - le mot-cle est remoteip (valeur localsubnet). « remoteaddr » et
#    « remoteaddress » sont tous les deux refuses par netsh ;
#  - le CODE DE SORTIE de netsh ne vaut rien : il rend 0 quand il refuse
#    (« l'operation demandee requiert une elevation ») comme sur un argument
#    invalide. Et « show rule » rend 0 meme quand la regle n'existe pas.
#    La seule lecture fiable est Get-NetFirewallRule, dont l'erreur n'est pas
#    localisee.
function RegleEnPlace($nom) {
  try { $null = Get-NetFirewallRule -DisplayName $nom -ErrorAction Stop ; return $true } catch { return $false }
}
if ($SansPareFeu) {
  Note "   pare-feu : NON touche (-SansPareFeu). Sans regle, une autre machine ne verra jamais ce pont."
} elseif (RegleEnPlace $rule) {
  Note "   pare-feu : regle deja en place « $rule »"
} else {
  $null = netsh advfirewall firewall add rule "name=$rule" dir=in action=allow protocol=TCP "localport=$Port" profile=private,domain remoteip=localsubnet 2>&1
  if (RegleEnPlace $rule) {
    Note "   pare-feu : regle posee « $rule » (prive/domaine, depuis le sous-reseau local seulement)"
  } else {
    Note "   (!) regle NON posee — droits administrateur requis sur cette machine."
    Note "       Le plus simple : double-cliquer Ouvrir-PareFeu-Canal.cmd (il se relance en administrateur)."
    Note "       Ou coller exactement, dans un terminal ADMINISTRATEUR :"
    Note "       netsh advfirewall firewall add rule name=`"$rule`" dir=in action=allow protocol=TCP localport=$Port profile=private,domain remoteip=localsubnet"
    Note "       Sans cette regle, le pont ecoute mais reste invisible depuis l'autre machine."
  }
}

# ---------- 4. allumer le pont ----------
$corps = @{ enabled = $true } | ConvertTo-Json -Compress
try {
  $apres = Invoke-RestMethod -Uri ($UI + '/api/bridge') -Method Post -Body $corps -ContentType 'application/json' -TimeoutSec 20
  Note "   pont demande : demande=$($apres.requested) actif=$($apres.enabled) ecoute=$($apres.listening) port=$($apres.port)"
  if ($apres.error) { Note "   (!) $($apres.error)" }
} catch {
  Note "   (!) POST /api/bridge : " + $_.Exception.Message
  exit 1
}

# Le pont s'allume dans le process qui tient la base ; la veille (4 s) le rattache
# au reglage. On attend qu'il ecoute vraiment, au lieu de croire la demande.
$ecoute = $false
$until = (Get-Date).AddSeconds(30)
while ((Get-Date) -lt $until) {
  Start-Sleep -Seconds 2
  try {
    $e = Invoke-RestMethod -Uri ($UI + '/api/bridge') -TimeoutSec 5
    if ($e.listening -or $e.running) { $ecoute = $true ; break }
  } catch { }
}
if (-not $ecoute) {
  Note "   (!) le pont n'est pas a l'ecoute au bout de 30 s. Ca veut dire : port deja pris, ou"
  Note "       moteur absent. Voir `ajean bridge status` et le log de l'instance."
  exit 1
}
Note "   pont a l'ecoute sur le port $Port"

# ---------- 5. preuve que le pont sert ----------
$ip = ''
foreach ($u in $etatPont.urls) { if ($u -notmatch '127\.0\.0\.1') { $ip = ([uri]$u).Host ; break } }
if (-not $ip) { $ip = '127.0.0.1' }
Note "   adresses annoncees par la machine : " + (($etatPont.urls) -join ' , ')

$cleLocale = $null
try { $cleLocale = (Invoke-RestMethod -Uri ($UI + '/api/apikey') -TimeoutSec 6).key } catch { }
if (-not $cleLocale) {
  Note "   (!) cle locale non relue (/api/apikey) : la preuve /v1 ne pourra pas etre jouee."
} else {
  $H = @{ Authorization = ("Bearer " + $cleLocale) }
  try {
    $m = Invoke-RestMethod -Uri ("http://127.0.0.1:" + $Port + "/v1/models") -Headers $H -TimeoutSec 20
    Note ("   /v1/models via le pont : " + ((@($m.data) | ForEach-Object { $_.id }) -join ', '))
  } catch { Note "   (!) /v1/models via le pont : " + $_.Exception.Message }
  try {
    $q = @{ model = 'ajean' ; messages = @(@{ role = 'user' ; content = 'Reponds par un seul mot : pret' }) ; max_tokens = 8 } | ConvertTo-Json -Depth 4
    $r = Invoke-RestMethod -Uri ("http://127.0.0.1:" + $Port + "/v1/chat/completions") -Method Post -Headers $H -Body $q -ContentType 'application/json' -TimeoutSec 180
    Note ("   /v1/chat/completions via le pont : « " + (@($r.choices)[0].message.content) + " »")
  } catch { Note "   (!) /v1/chat/completions via le pont : " + $_.Exception.Message }

  # La preuve propre a la revision 3 : la MEME route, mais avec l'en-tete du canal
  # (X-Ajean-Canal), c'est-a-dire ce que fait reellement l'outil peer. Le modele
  # prete doit alors dire qu'il n'a aucun outil. Sans la revision 3, il
  # repondait a cote — « je tourne sur une infrastructure cloud » — parce que
  # personne ne lui avait dit qu'il etait prete. On affiche, on ne valide pas :
  # c'est une generation, pas une constante.
  try {
    $H2 = @{ Authorization = ("Bearer " + $cleLocale) ; 'X-Ajean-Canal' = '1' }
    $q2 = @{ model = 'ajean' ; messages = @(@{ role = 'user' ; content = 'As-tu un terminal, un disque ou la memoire ici ? Une phrase.' }) ; max_tokens = 90 } | ConvertTo-Json -Depth 4
    $r2 = Invoke-RestMethod -Uri ("http://127.0.0.1:" + $Port + "/v1/chat/completions") -Method Post -Headers $H2 -Body $q2 -ContentType 'application/json' -TimeoutSec 240
    Note ("   avec l'en-tete du canal, le modele prete dit : « " + (@($r2.choices)[0].message.content) + " »")
    Note "       (attendu : il doit dire qu'il n'a ni outil, ni disque, ni memoire — s'il invente un"
    Note "        cloud ou un dossier, le binaire n'est pas a la revision 3)"
  } catch { Note "   (!) /v1/chat/completions avec l'en-tete du canal : " + $_.Exception.Message }
}

if ($Verifier) { exit 0 }

# ---------- 6. le voisin ----------
if (-not $VoisinNom -or -not $VoisinUrl) {
  Note ""
  Note "   voisin non demande : le pont est ouvert, cette machine peut deja etre appelee."
  Note "   Pour qu'elle APPELLE l'autre : -VoisinNom <nom> -VoisinUrl http://<IP-de-l-autre>:$Port/v1 -Cle <cle de l'autre>"
  exit 0
}
$ajout = @{ add = $VoisinNom ; url = $VoisinUrl ; key = $Cle } | ConvertTo-Json -Compress
try {
  $a = Invoke-RestMethod -Uri ($UI + '/api/peers') -Method Post -Body $ajout -ContentType 'application/json' -TimeoutSec 30
  Note "   voisin declare : $($a.name) -> $($a.url)"
} catch {
  Note "   (!) POST /api/peers : " + $_.Exception.Message
  exit 1
}
try {
  $p = Invoke-RestMethod -Uri ($UI + '/api/peers') -TimeoutSec 10
  foreach ($v in $p.peers) { Note ("      " + $v.name + " -> " + $v.url + " · cle=" + $v.has_key + " · modele " + $v.model) }
} catch { Note "   (!) /api/peers : " + $_.Exception.Message }

# Sonde + un vrai aller-retour : la seule preuve qui vaille, c'est une reponse
# qui vient de l'autre cerveau. L'aller-retour ne passe PAS par l'API web : la
# route n'existe pas (/api/peers ne fait qu'ajouter, retirer, oublier). C'est la
# CLI qui porte le canal : « ajean peer ask <nom> <message> ». Le binaire du
# dossier est appele directement — il resout AJEAN_HOME sur son propre dossier
# (verifie avec `ajean.exe where`), donc il parle a la base de CE dossier.
try {
  $s = Invoke-RestMethod -Uri ($UI + '/api/peers') -TimeoutSec 10
  Note "   canal : " + (@($s.peers).Count) + " voisin(s) declare(s)"
} catch { }
Note "   question au voisin (CLI, 5 min maxi — un gros MoE peut mettre du temps)…"
$rep = & $exe peer ask $VoisinNom 'Dis juste le nom du modele que tu es, en un mot.' 2>&1
$code = $LASTEXITCODE
foreach ($ligne in $rep) { Note ("      " + (("" + $ligne) -replace '[^\x20-\x7E]','?')) }
if ($code -ne 0) {
  Note "   (!) le voisin n'a pas repondu (code $code). Il est declare quand meme. Causes attendues :"
  Note "       son pont est eteint, son pare-feu ne laisse rien entrer, ou la cle n'est pas la sienne."
  Note "       Verifier la-bas : ajean bridge status (doit dire « pare-feu : port autorise »)"
  Note "       et, depuis ici : curl http://<IP-voisine>:$Port/v1/models (avec la cle du voisin)."
  exit 1
}

Note ""
Note "CANAL OUVERT — cette machine prete et parle."
