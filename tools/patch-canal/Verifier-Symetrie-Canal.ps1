# Verifie la SYMETRIE du canal Ajean↔Ajean, dans les deux sens, et dit sur
# QUELLE MACHINE la moitié manque. Ecrit le 2026-10-28.
#
# POURQUOI CE SCRIPT EXISTE : un canal peut etre ouvert des deux cotes et ne
# toujours pas fonctionner. Mesure du 2026-10-28 chez la voisine : son pont
# repond 200 a ma cle (donc son pont est ouvert, sa cle est bonne), et
# pourtant elle ne peut rien me demander. Parce que l'outil `peer` n'est pas
# annonce au modele quand AUCUN voisin n'est declare chez elle
# (peerConfigured() dans llm_client.go / parallel_peer.go). Le pont prete,
# l'appelant ne part pas. Un controle qui ne regarde que « le pont est-il
# ouvert ? » annonce DONC une reussite qui n'existe pas.
#
# Les quatre conditions, par machine :
#   1. le pont ecoute            -> /api/bridge : enabled + listening
#   2. un voisin est declare     -> /api/peers  : peers[] non vide
#   3. la cle du voisin est rangee -> peers[].has_key
#   4. le pont du voisin accepte cette cle -> GET <voisin>/v1/models = 200
#
# Condition 2 est celle qu'on ne peut pas verifier chez le voisin : son pont ne
# sert QUE /v1/* (mesure : /api/peers sur son pont = 404 « surface OpenAI
# uniquement »), et son interface n'est pas ouverte sur le reseau (mesure : son
# 18090 est ferme depuis le LAN). Donc la moitie manquante se regle SUR SA
# MACHINE, et ce script doit etre joue la-bas aussi.
#
# LECTURE SEULE. Ne demande aucune cle : il lit celle qui est deja rangee en
# base, sans jamais l'afficher. Sortie != 0 si la symetrie n'est pas faite.
param(
  [string]$Voisin = '',      # IP du voisin ; vide = on prend celui qui est declare
  [int]$PortPont  = 18190,
  [string]$UI     = 'http://127.0.0.1:18090',
  [string]$Cle    = '',      # cle du voisin, pour confirmer l'appel jusqu'au 200
  [switch]$Demander          # la demander au clavier, sans trace (Read-Host -AsSecureString)
)
$ErrorActionPreference = 'Continue'

function CleMasquee([string]$Libelle) {
  $s = Read-Host -Prompt $Libelle -AsSecureString
  $pt = [Runtime.InteropServices.Marshal]::SecureStringToGlobalAllocUnicode($s)
  try { return [Runtime.InteropServices.Marshal]::PtrToStringUni($pt) }
  finally { [Runtime.InteropServices.Marshal]::ZeroFreeGlobalAllocUnicode($pt) }
}

function Emp([string]$s) {
  if (-not $s) { return '(vide)' }
  try {
    $sha = [Security.Cryptography.SHA256]::Create()
    $h = $sha.ComputeHash([Text.Encoding]::UTF8.GetBytes($s))
    return ('empreinte ' + ([BitConverter]::ToString($h[0..5])).Replace('-',''))
  } catch { return '(empreinte indisponible)' }
}

Write-Output '== SYMETRIE DU CANAL AJEAN <-> AJEAN =='
Write-Output ('interface locale : ' + $UI)

# ---- 1. ce que cette machine prete ---------------------------------------
$bridge = $null
try { $bridge = Invoke-RestMethod -Uri ($UI + '/api/bridge') -TimeoutSec 8 }
catch {
  Write-Output ('(!) /api/bridge ne repond PAS sur ' + $UI + ' : ' + $_.Exception.Message)
  Write-Output ''
  Write-Output '   TROIS CAUSES, dans l ordre ou les verifier :'
  Write-Output '     a) le port saisi est celui du MOTEUR (18080), pas celui de l interface'
  Write-Output '        (18090). Sur 18080 on obtient le panneau du moteur Strata : il n a'
  Write-Output '        ni la carte « Pont Ajean ↔ Ajean » ni /api/bridge (mesure : 404).'
  Write-Output '     b) l interface est sur un autre port (AJEAN_UI_PORT) — le dire, ne pas deviner.'
  Write-Output '     c) l instance ne tourne pas.'
  exit 2
}
Write-Output ('  pont    : enabled=' + $bridge.enabled + '  listening=' + $bridge.listening +
              '  port=' + $bridge.port + '  cle=' + $bridge.has_key)
Write-Output ('  modele prete : ' + $bridge.model)
if ($bridge.urls) {
  Write-Output '  adresses annoncees par CETTE machine (a donner au voisin, les recopier) :'
  foreach ($u in $bridge.urls) { Write-Output ('     ' + $u) }
}
# Les interrupteurs du tour d'agent : sans les deux, le pont ne prete que le modele nu.
if ($bridge.PSObject.Properties.Name -contains 'agent_on') {
  Write-Output ('  tour d agent : agent_on=' + $bridge.agent_on +
                '  autorisation_pont=' + $bridge.agent_requested +
                '  mode_agent_machine=' + $bridge.agent_mode_local)
} else {
  Write-Output '  tour d agent : /api/bridge n expose pas agent_on -> binaire anterieur a la revision 4'
}

# ---- 2. ce que cette machine appelle -------------------------------------
$peers = @()
try {
  $p = Invoke-RestMethod -Uri ($UI + '/api/peers') -TimeoutSec 8
  $peers = @($p.peers)
} catch { Write-Output ('(!) /api/peers : ' + $_.Exception.Message) }
Write-Output ('  voisins declares ici : ' + $peers.Count)
foreach ($v in $peers) {
  Write-Output ('     ' + $v.name + ' -> ' + $v.url + '  cle=' + $v.has_key +
                '  modele vu « ' + $v.model + ' »')
}
if (-not $peers.Count) {
  Write-Output '     AUCUN. C est la moitie qui manque le plus souvent : le pont prete,'
  Write-Output '     mais l outil « peer » n est pas annonce au modele sans voisin declare.'
}

# ---- 3. la moitie du voisin ----------------------------------------------
# La cible : soit celle passee a -Voisin, soit celle qui est declaree ici.
$cibles = @()
if ($Voisin) { $cibles += ('http://' + $Voisin + ':' + $PortPont + '/v1') }
foreach ($v in $peers) { if ($v.url) { $cibles += $v.url } }
$cibles = @($cibles | Select-Object -Unique)

Write-Output ''
Write-Output '== CE QUE CETTE MACHINE PEUT APPELER =='
if (-not $cibles.Count) {
  Write-Output '  RIEN : aucun voisin declare, donc aucun appel partant possible ici.'
}
# Trois verdicts, pas deux. Premiere version du script : elle ne comptait « peut
# appeler » que sur un 200 obtenu avec une cle relue dans /api/peers — or le moteur
# ne RENDS PAS la cle (mesure du 2026-10-28 : peers[].has_key = True, peers[].key
# absent, volontairement). Le script annoncait donc « ne peut pas appeler » sur une
# machine qui le peut. Un pont qui repond 401 avec une cle rangee chez nous EST
# pret : c'est l'appel reel qui la utilisera.
$pretAppels = 0   # declare + cle rangee + pont du voisin present
$okAppels = 0     # 200 effectivement obtenu
foreach ($c in $cibles) {
  $base = ($c -replace '/v1/?$','')
  $url  = ($base.TrimEnd('/') + '/v1/models')
  $voisinIci = $null
  foreach ($v in $peers) { if ($v.url -and (($v.url -replace '/v1/?$','') -eq $base)) { $voisinIci = $v } }
  $cleRangee = [bool]($voisinIci -and $voisinIci.has_key)
  Write-Output ('  cible ' + $url)
  Write-Output ('       declare ici : ' + [bool]$voisinIci + '   cle rangee en base : ' + $cleRangee)
  $joint = $false
  try {
    $null = Invoke-RestMethod -Uri $url -TimeoutSec 8
    Write-Output '       SANS CLE : ACCEPTe. Le pont est ouvert a tout le monde, a corriger.'
    $joint = $true
  } catch [System.Net.WebException] {
    $rep = $_.Exception.Response
    if ($rep -and [int]$rep.StatusCode -eq 401) {
      Write-Output '       SANS CLE : 401 → pont present et verrouille (bon etat).'
      $joint = $true
    } else {
      Write-Output ('       INJOIGNABLE : ' + $_.Exception.Message)
    }
  } catch { Write-Output ('       ERREUR : ' + $_.Exception.Message) }

  if ($joint -and $cleRangee) { $pretAppels++ }

  # Confirmation jusqu'au 200, si la cle nous est fournie a la main.
  $cle = $Cle
  if (-not $cle -and $Demander) { $cle = CleMasquee ('cle du voisin pour ' + $base) }
  if ($joint -and $cle) {
    try {
      $m = Invoke-RestMethod -Uri $url -Headers @{ Authorization = ('Bearer ' + $cle) } -TimeoutSec 8
      $nomModele = if ($m.data -and $m.data.Count) { $m.data[0].id } else { '?' }
      Write-Output ('       AVEC la cle fournie : 200 → APPEL CONFIRME, modele « ' + $nomModele + ' »')
      Write-Output ('       ' + (Emp $cle) + ' — comparee a celle du voisin, jamais affichee')
      $okAppels++
    } catch {
      Write-Output ('       AVEC la cle fournie : ECHEC (' + $_.Exception.Message + ') → cle mauvaise ou obsolete')
    }
  } elseif ($joint -and $cleRangee) {
    Write-Output '       Le 200 n est pas confirme ici : le moteur ne rend pas la cle rangee (voulu).'
    Write-Output '       Pour confirmer : rejouer avec -Cle … ou -Demander.'
  }
}

Write-Output ''
Write-Output '== CE QUE LE VOISIN PEUT APPELER ICI (il doit le verifier chez lui) =='
Write-Output '  Son pont ne sert QUE /v1/* : /api/peers chez lui = 404 (mesure). Son interface'
Write-Output '  n est pas ouverte sur le reseau. On ne peut donc PAS lire ses voisins depuis ici.'
Write-Output '  A faire LA-BAS :  .\Verifier-Symetrie-Canal.ps1'
Write-Output '  (le meme script, joue sur sa machine).'

Write-Output ''
Write-Output '== BILAN =='
$pretAPret = ($bridge.enabled -and $bridge.listening -and $bridge.has_key)
Write-Output ('  cette machine prete        : ' + $(if ($pretAPret) { 'OUI' } else { 'NON' }))
Write-Output ('  pret a appeler (declare + cle rangee + pont du voisin present) : ' +
              $(if ($pretAppels) { ('OUI (' + $pretAppels + ')') } else { 'NON' }))
Write-Output ('  appel effectivement confirme par un 200 : ' +
              $(if ($okAppels) { ('OUI (' + $okAppels + ')') } else { 'NON — non tente (la cle rangee ne sort pas de la base)' }))
$note = 'incomplete'
if ($pretAPret -and $pretAppels) { $note = 'COMPLETE (les deux moities sont en place ici)' }
Write-Output ('  symetrie, vue de cette machine : ' + $note)
if (-not $pretAPret) {
  Write-Output ''
  Write-Output '  RESTE ICI : ouvrir le pont ->  .\Ouvrir-Canal.ps1'
}
if (-not $pretAppels) {
  Write-Output ''
  Write-Output '  RESTE ICI : declarer le voisin ->  .\cabler-canal.ps1 -Voisin <IP-du-voisin>'
  Write-Output '  La cle demandee est celle de la machine APPELEE, pas celle d ici.'
}
if ($pretAPret -and -not $okAppels) {
  Write-Output ''
  Write-Output '  Pour lever le doute sur la cle : rejouer avec -Demander (saisie au clavier,'
  Write-Output '  sans trace) et obtenir le 200 du /v1/models du voisin.'
}
if ($pretAPret -and $pretAppels) { exit 0 }
exit 1
