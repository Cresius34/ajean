# Repond a une question precise : « pourquoi la carte « Pont Ajean ↔ Ajean »
# n'apparait-elle pas ? » Ecrit le 2026-10-28, a partir d'une capture d'ecran.
#
# CE QUE LA CAPTURE MONTRE : la version annoncee est 0.18.16.1, la carte est
# absente, et une adresse est saisie sur le port 18080. Les octets repondent :
#
#   1. la carte est DANS le binaire. Compte des marques dans les trois binaires
#      fabriques (ajean-18155, ajean-18156, ajean-18161) : bridge-details x2,
#      bridgeToggle x2, renderBridge x4, ajean-pont x1, pareils partout. Donc
#      « ma version ne l'a pas » est faux : elle l'a.
#   2. l'interface est EMBARQUEE dans le binaire (web_server.go : //go:embed
#      ui/index.html …). Le HTML servi vient du binaire qui tourne.
#   3. le service worker (ui/sw.js) ne met RIEN en cache, volontairement. Donc
#      aucun vieux HTML resservi : vider le cache ne peut rien changer.
#   4. 18080 est le port du MOTEUR Strata, pas celui de l'interface. Mesure sur
#      ce poste : http://127.0.0.1:18080/ rend le panneau du moteur (16 960
#      octets, « Strata on GitHub ») SANS bridge-details, et son /api/bridge
#      repond 404 {"error":{"message":"not found"}}. L'interface, elle, est sur
#      18090 (parallel_config.go : parallelUIPortDef = "18090") et son HTML sert
#      bien <details id="bridge-details">.
#
# La regle d'affichage, lue dans internal/ajean/ui/src/js/13b-bridge.js :
# la carte <details id="bridge-details" hidden> ne se montre QUE si /api/bridge
# repond ok:true. 404 = le service interroge n'a pas cette API (moteur, ou
# binaire anterieur au pont) ; 403 = version portable sans la surface.
#
# LECTURE SEULE. Sortie != 0 si la carte ne peut pas apparaitre.
param(
  [string[]]$Ports = @('18090', '18080', '18094', '18084', '18095', '18085'),
  [string]$Hote = '127.0.0.1'
)
$ErrorActionPreference = 'Continue'

# Les marques cherchees dans le HTML servi. Elles viennent de la source de
# l'interface, pas d'un nom invente : si l'interface change, ces marques
# changent avec elle.
$Marques = @('bridge-details', 'bridgeToggle', 'renderBridge')

Write-Output '== LA CARTE « Pont Ajean <-> Ajean » PEUT-ELLE APPARAITRE ? =='
Write-Output ('hote interroge : ' + $Hote)
$trouve = $false
foreach ($p in $Ports) {
  $url = 'http://' + $Hote + ':' + $p + '/'
  Write-Output ''
  Write-Output ('--- port ' + $p + ' : ' + $url)
  $html = ''
  try {
    # -UseBasicParsing : sans lui, Invoke-WebRequest peut echouer sur un parse
    # HTML (piege mesure le 2026-10-28 avec /api/network).
    $w = Invoke-WebRequest -Uri $url -TimeoutSec 6 -UseBasicParsing
    $html = [string]$w.Content
  } catch {
    Write-Output ('    rien ne repond (' + $_.Exception.Message + ')')
    continue
  }
  $nes = @($Marques | Where-Object { $html -like ('*' + $_ + '*') })
  Write-Output ('    page de ' + $html.Length + ' octets · marques de la carte trouvees : ' + $nes.Count + ' / ' + $Marques.Count)
  if ($html -match 'Strata on GitHub') {
    Write-Output '    CE PORT EST CELUI DU MOTEUR Strata (panneau du moteur, pas l interface AJEAN).'
    Write-Output '    La carte n y existera JAMAIS : elle vit dans l interface, port 18090.'
  }
  # L'API du pont, sur le MEME port : c'est elle qui decide de l'affichage.
  try {
    $b = Invoke-RestMethod -Uri ('http://' + $Hote + ':' + $p + '/api/bridge') -TimeoutSec 6
    Write-Output ('    /api/bridge -> ok=' + $b.ok + '  enabled=' + $b.enabled + '  listening=' + $b.listening + '  port_pont=' + $b.port)
  } catch {
    $code = ''
    try { $code = [string][int]$_.Exception.Response.StatusCode } catch {}
    Write-Output ('    /api/bridge -> ECHEC ' + $(if ($code) { ('HTTP ' + $code) } else { $_.Exception.Message }))
    if ($code -eq '404') {
      Write-Output '    404 = ce service n a pas l API du pont : c est le MOTEUR, ou un binaire'
      Write-Output '    anterieur au pont. La regle de 13b-bridge.js cache alors la carte.'
    } elseif ($code -eq '403') {
      Write-Output '    403 = version portable sans cette surface.'
    }
  }
  if ($nes.Count -eq $Marques.Count) {
    Write-Output '    => l interface servie ici CONTIENT la carte. Si elle ne s affiche pas,'
    Write-Output '       la cause est /api/bridge au-dessus (le JS ne la montre que si ok:true).'
    $trouve = $true
  }
}

Write-Output ''
Write-Output '== BILAN =='
if ($trouve) {
  Write-Output '  La carte est servie. Ce n est ni un cache, ni une version trop vieille.'
  Write-Output '  Si elle reste invisible a l ecran : verifier /api/bridge sur le MEME port'
  Write-Output '  (le bloc au-dessus), puis regarder la console du navigateur.'
  exit 0
}
Write-Output '  Aucune interface avec la carte n a ete trouvee sur ces ports.'
Write-Output '  Verifier dans l ordre : (1) l instance tourne ; (2) le port de l interface est'
Write-Output '  18090, pas 18080 (le moteur) — AJEAN_UI_PORT peut le dcaler, le lire dans'
Write-Output '  .\Diagnostic-Installation.ps1 ; (3) le binaire qui tourne est bien celui qui'
Write-Output '  est sur le disque (compare les versions : .\Trouver-Installations.ps1).'
exit 1
