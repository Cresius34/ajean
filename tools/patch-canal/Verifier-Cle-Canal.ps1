# Verifie si les DEUX machines du canal partagent la MEME cle, et le dit.
# Ecrit le 2026-10-28, apres la phrase de l'utilisateur : « me semble que vos
# clefs sont les memes ». Il avait raison, et rien dans l'outillage ne le
# mesurait : le pont prete, le pair est declare, la cle est « posee » — tout est
# vert, et pourtant les deux ponts s'ouvrent avec le meme secret.
#
# CE QUE CA VEUT DIRE, concretement :
#   - une seule cle pour deux cerveaux : celui qui la possede peut appeler l'un
#     et l'autre. La rotation d'un cote casse l'autre, et la revocation
#     separee est impossible ;
#   - la cle n'est pas seulement en base : le canal a une memoire (le fil des
#     derniers echanges est garde en base des deux cotes), donc une cle colle
#     DANS UN MESSAGE DU CANAL y survit. Mesure du jour : la cle de l'une des
#     machines a ete ecrite en clair dans un message exchange — elle est donc
#     lisible dans l'historique des DEUX postes ;
#   - deux machines qui ont la meme cle ne peuvent pas se distinguer l'une de
#     l'autre dans les journaux : un appel recu signifie « la cle du canal », pas
#     « la machine voisine ».
#
# Comment on mesure sans jamais exposer la cle : on compare des EMPREINTES
# SHA-256 (6 premiers octets), et on teste une cle CONTRE LE PONT DU VOISIN par
# un simple GET /v1/models. Ma cle ouvre-t-elle le pont d'en face ? Si oui, les
# secrets sont identiques. La cle n'est jamais affichee, jamais ecrite.
#
# LECTURE SEULE. Sortie != 0 si les cotes partagent le meme secret.
param(
  [string]$Voisin = '',        # IP du voisin ; vide = on prend ceux qui sont declares
  [int]$PortPont  = 18190,
  [string]$UI     = 'http://127.0.0.1:18090',
  [switch]$Demander            # demander la cle du voisin au clavier (sans trace)
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
    return (([BitConverter]::ToString($h[0..5])).Replace('-',''))
  } catch { return '(indisponible)' }
}

Write-Output '== LES DEUX PONTS DU CANAL ONT-ILS LE MEME SECRET ? =='
Write-Output ('interface locale : ' + $UI)

# 1. Ma cle a moi : ou est-elle ? Le moteur ne la rend pas (voulu : la cle ne
#    sort pas de la base). On la demande donc a l'humain, au clavier.
$maCle = ''
if ($Demander) { $maCle = CleMasquee 'cle API de CETTE machine (saisie masquee)' }
Write-Output ('empreinte de ma cle : ' + $(if ($maCle) { (Emp $maCle) } else { 'NON DEMANDEE (-Demander)' }))
Write-Output "  la cle n'est jamais affichee en clair : seule son empreinte sort ici."

# 2. Les ponts des voisins declares.
$peers = @()
try { $p = Invoke-RestMethod -Uri ($UI + '/api/peers') -TimeoutSec 8 ; $peers = @($p.peers) }
catch { Write-Output ('(!) /api/peers : ' + $_.Exception.Message) }
$cibles = @()
if ($Voisin) { $cibles += ('http://' + $Voisin + ':' + $PortPont + '/v1') }
foreach ($v in $peers) { if ($v.url) { $cibles += $v.url } }
$cibles = @($cibles | Select-Object -Unique)
Write-Output ('ponts voisins a tester : ' + $cibles.Count)

$partage = 0
foreach ($c in $cibles) {
  $url = (($c -replace '/v1/?$','').TrimEnd('/')) + '/v1/models'
  Write-Output ''
  Write-Output ('--- ' + $url)
  # a. Le pont est-il la, et verrouille ?
  try {
    $null = Invoke-RestMethod -Uri $url -TimeoutSec 8
    Write-Output '   SANS CLE : ACCEPTe. Le pont est ouvert a tout le monde : a corriger.'
  } catch [System.Net.WebException] {
    $rep = $_.Exception.Response
    if (-not ($rep -and [int]$rep.StatusCode -eq 401)) {
      Write-Output ('   INJOIGNABLE : ' + $_.Exception.Message)
      Write-Output '   (un pont ferme n est pas une cle differente : on ne peut rien conclure)'
      continue
    }
    Write-Output '   SANS CLE : 401 → pont present et verrouille.'
  } catch { Write-Output ('   ERREUR : ' + $_.Exception.Message) ; continue }

  # b. Est-ce MA cle qui l'ouvre ? C'est la question qui repond a « vos cles sont
  #    les memes ». Si oui : partage, et il faut tourner.
  if ($maCle) {
    try {
      $m = Invoke-RestMethod -Uri $url -Headers @{ Authorization = ('Bearer ' + $maCle) } -TimeoutSec 8
      $nom = if ($m.data -and $m.data.Count) { $m.data[0].id } else { '?' }
      Write-Output ('   AVEC MA CLE : 200 → CE MEME SECRET OUVRIRAIT LES DEUX PONTS. partage a tourner.')
      $partage++
    } catch [System.Net.WebException] {
      $c2 = ''
      try { $c2 = [string][int]$_.Exception.Response.StatusCode } catch {}
      if ($c2 -eq '401') { Write-Output '   AVEC MA CLE : 401 → les deux cotes ont bien des secrets DIFFERENTS.' }
      else { Write-Output ('   AVEC MA CLE : ' + $(if ($c2) { ('HTTP ' + $c2) } else { $_.Exception.Message })) }
    } catch { Write-Output ('   AVEC MA CLE : ' + $_.Exception.Message) }
  } else {
    Write-Output '   MA CLE N A PAS ETE TESTEE : relancer avec -Demander pour trancher le partage.'
  }
}

Write-Output ''
Write-Output '== BILAN =='
if ($partage) {
  Write-Output ('  MEME SECRET DES DEUX COTES sur ' + $partage + ' pont(s).')
  Write-Output ''
  Write-Output '  POURQUOI CE N EST PAS neutre :'
  Write-Output '    - un seul secret pour deux cerveaux : qui le possede peut appeler l un ET l autre ;'
  Write-Output '    - tourner la cle d un cote casse l autre : il faut re-declarer en face, dans la foulee ;'
  Write-Output '    - le canal garde son fil en base : une cle colle dans un MESSAGE du canal y survit,'
  Write-Output '      donc elle est lisible dans l historique des DEUX postes.'
  Write-Output ''
  Write-Output '  PROCEDURE DE ROTATION, dans cet ordre (sinon le canal se casse en route) :'
  Write-Output '    1. sur la machine A : NOUVELLE cle, par l INTERFACE (page Reglages, section cle API).'
  Write-Output '       NE PAS jouer « ajean set-api-key » sur une instance EN SERVICE : la commande'
  Write-Output '       attend une reponse sur stdin (elle BLOQUE en script) et redemarre le moteur'
  Write-Output '       en cours. En CLI, seulement instance arretee.'
  Write-Output '       (la cle machine est en base, pas dans un fichier : elle ne se lit donc pas'
  Write-Output '        par un fichier du paquet, et ne doit jamais y entrer.)'
  Write-Output '    2. sur la machine B : ajean peer rm <nom-de-A>, puis'
  Write-Output '       ajean peer add <nom-de-A> http://<IP-A>:18190/v1 <NOUVELLE-cle-de-A>'
  Write-Output '       (la cle en TROISIEME position. « --key » etait avalé COMME la cle avant le'
  Write-Output '        correctif du 2026-10-28 : la cle rangee valait litteralement « --key » et le'
  Write-Output '        pont repondait 401 a vie. Desormais accepte, mais la forme positionnelle'
  Write-Output '        reste l usage : « ajean peer add <nom> <url> [cle] [modele] ».)'
  Write-Output '    3. verifier dans les deux sens : .\Verifier-Symetrie-Canal.ps1'
  Write-Output '    4. ne plus jamais ecrire une cle dans un MESSAGE du canal : le fil est garde en'
  Write-Output '       base des deux cotes (20 derniers messages), la cle y survit et repart dans les'
  Write-Output '       prompts suivants. Nettoyage : ajean peer forget <nom>.'
  exit 1
}
Write-Output '  Aucun partage constate entre ce poste et les ponts teste.'
if (-not $maCle) { Write-Output '  (non prouve : ma cle n a pas ete testee — relancer avec -Demander)' }
exit 0
