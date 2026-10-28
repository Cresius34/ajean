# Verifie les .ps1 et .cmd d'un dossier d'outils du canal : BOM, parse, collisions
# de casse, coherence des versions annoncees, INTEGRITE DU BINAIRE, et ce que le
# voisin sert reellement. A lancer APRES toute retouche du paquet, avant de le
# poser sur une machine.
param([string[]]$AdresseVoisin = @(), [string]$CleCanal = '')
# La cle du VOISIN (jamais la sienne) : sans elle le pont repond 401 et la preuve
# de la carte d'identite est impossible. Elle passe par ce parametre ou par
# $env:AJEAN_CLE_CANAL, jamais par un fichier de ce dossier.
# Noms de variables choisis sans jumeau qui ne differe que par la casse : en
# PowerShell $Voisins et $voisins sont la MEME variable, et le parametre se
# retrouvait ecrase par la variable locale (attrape par Verifier-Collisions-Casse).
$ErrorActionPreference = 'Stop'
$dossier = $PSScriptRoot

# La liste des fichiers n'est PAS ecrite ici. Mesure du 2026-10-28 : cette liste
# en dur oubliait les outils ajoutes la veille, donc le verificateur annoncait un
# paquet sain en n'ayant rien dit des deux pieces neuves. On enumere ce qui vit
# reellement dans le dossier, plus ce qui vit dans lanceurs\ quand il existe.
$fichiers = @(Get-ChildItem -LiteralPath $dossier -File |
              Where-Object { $_.Extension -in '.ps1', '.cmd' } |
              ForEach-Object { $_.Name })
$sousLanceurs = Join-Path $dossier 'lanceurs'
if (Test-Path -LiteralPath $sousLanceurs) {
  $fichiers += @(Get-ChildItem -LiteralPath $sousLanceurs -File |
                 Where-Object { $_.Extension -in '.ps1', '.cmd' } |
                 ForEach-Object { 'lanceurs\' + $_.Name })
}
$fichiers = @($fichiers | Sort-Object)

# La version visee n'est PAS ecrite ici non plus. Elle est LUE dans le binaire qui
# accompagne le paquet (a cote du script, ou au dossier au-dessus). Un millésime
# cablé dans le verificateur le rend faux au build suivant : c'est exactement le
# mecanisme du « le patch ne remplace pas les fichiers » — l'outil annoncait une
# coherence qui n'existait plus.
$WantV = ''
foreach ($c in @((Join-Path $dossier 'ajean.exe'),
                 (Join-Path (Split-Path -Parent $dossier) 'ajean.exe'))) {
  if (Test-Path -LiteralPath $c -PathType Leaf) {
    $WantV = [Diagnostics.FileVersionInfo]::GetVersionInfo($c).ProductVersion
    if ($WantV) { $WantV = $WantV.Trim() ; break }
  }
}
if (-not $WantV) {
  Write-Output '(!) AUCUN ajean.exe a cote de ce script ni dans son dossier parent :'
  Write-Output '    la version visee ne peut pas etre lue. Ce verificateur tourne dans'
  Write-Output '    data\scripts d''une installation, ou dans un dossier d''outils sans'
  Write-Output '    binaire : le controle des versions est SAUTE, pas reussi.'
}
Write-Output ("fichiers enumeres : " + $fichiers.Count +
              "   version lue du binaire : " + $(if ($WantV) { $WantV } else { 'INDISPONIBLE' }))
$pb = 0

Write-Output "== presence + parse + BOM =="
foreach ($f in $fichiers) {
  $p = Join-Path $dossier $f
  if (-not (Test-Path -LiteralPath $p)) { Write-Output ("{0} : ABSENT" -f $f) ; $pb++ ; continue }
  $b = [IO.File]::ReadAllBytes($p)
  $bom = ($b.Length -ge 3 -and $b[0] -eq 0xEF -and $b[1] -eq 0xBB -and $b[2] -eq 0xBF)
  $t = [IO.File]::ReadAllText($p, [Text.Encoding]::UTF8)
  $estBatch = ($f -like '*.cmd')

  # Un .cmd n'est PAS du PowerShell. Le passer au parseur produisait 18 fausses
  # erreurs par fichier (mesure du 2026-10-28 : « jeton inattendu off » sur
  # @echo off), donc le bilan criait et on apprenait a ne plus le lire.
  $err = @()
  if (-not $estBatch) {
    $e = $null
    $null = [System.Management.Automation.PSParser]::Tokenize($t, [ref]$e)
    $err = @($e)
  }
  Write-Output ("{0} | octets={1} | BOM={2} | {3}" -f $f, $b.Length, $bom,
                $(if ($estBatch) { 'batch — non parse (cmd, pas ps1)' } else { 'erreurs_parse=' + $err.Count }))
  foreach ($x in $err) { Write-Output ("    ligne {0} : {1}" -f $x.Token.StartLine, $x.Message) ; $pb++ }

  # Le BOM n'est pas un ornement : c'est lui qui fait lire les accents a PowerShell 5.
  # Un fichier SANS accent peut s'en passer (les lanceurs de la variante), encore faut-il
  # ne pas lui imposer celui de son voisin — le generateur les rend tels qu'ils sont.
  # Le test d'accent ne doit pas compter les fins de ligne : \r tombait hors de
  # \x20-\x7E et faisait annoncer « le fichier a des accents » a un fichier 100 % ASCII.
  $sansAccents = ($t -notmatch '[^\x20-\x7E\r\n]')
  $octetsNonAscii = @($b | Where-Object { $_ -ge 128 }).Count
  if ($estBatch) {
    # Regle mesuree : un .cmd sans BOM et sans aucun octet non ASCII. Le BOM sur un
    # .cmd s'affiche en caracteres parasites et la premiere commande est mal lue ;
    # un accent dans un .cmd est detruit par le codepage de la console.
    if ($bom) { Write-Output '    (!) BOM sur un .cmd : la premiere ligne est mal lue' ; $pb++ }
    if ($octetsNonAscii -gt 0) { Write-Output ("    (!) " + $octetsNonAscii + ' octet(s) non ASCII dans un .cmd : le console les detruit') ; $pb++ }
  } else {
    if (-not $bom -and -not $sansAccents) { Write-Output "    (!) BOM absent alors que le fichier a des accents — PowerShell 5.1 le lit en ANSI" ; $pb++ }
    if ($bom -and $sansAccents) { Write-Output "    (info) BOM pose sur un fichier sans accents — heritage, sans consequence" }
  }
}

Write-Output ""
Write-Output "== versions annoncees par les outils =="
# Le controle n'est saute que si le binaire est vraiment absent, et le mot le dit :
# « SAUTE », pas « reussi ». Annoncer une coherence qu'on n'a pas pu lire est la
# faute exacte qui a laisse passer trois revisions perdues.
if (-not $WantV) { Write-Output '  SAUTE — aucune version lue dans un binaire du paquet.' }
foreach ($f in @('Appliquer-Patch-Canal.ps1', 'Ouvrir-Canal.ps1', 'lanceurs\genere-lanceurs.ps1')) {
  $p = Join-Path $dossier $f
  if (-not (Test-Path -LiteralPath $p -PathType Leaf)) {
    Write-Output ("  {0} : ABSENT" -f $f) ; $pb++ ; continue
  }
  $t = [IO.File]::ReadAllText($p, [Text.Encoding]::UTF8)
  $want = ([regex]::Match($t, '\$Want\w*\s*=\s*''([^'']*)''')).Groups[1].Value
  Write-Output ("  {0} : WantV en dur = « {1} »" -f $f, $(if ($want) { $want } else { 'AUCUNE — version lue a l execution, ce qui est le comportement voulu' }))
  # Une version en dur dans un outil est un mensonge differe : elle devient fausse
  # au build suivant et l'outil refuse alors de tourner sans dire pourquoi.
  if ($WantV -and $want -and $want -ne $WantV) { Write-Output "     (!) vise $want alors que le binaire du paquet annonce $WantV" ; $pb++ }
  # Une version citee doit etre soit la cible, soit une entree de la liste admise.
  # (Premiere version du controle : elle signalait la cible elle-meme comme
  # « citee hors de la liste admise » — fausse alerte, mesurée le 2026-10-27.)
  if ($f -ne 'lanceurs\genere-lanceurs.ps1') {
    # Le motif etait '(0\.18\.15\.\d Cresius)' : il ne voyait plus rien des que la
    # ligne Cresius changeait d'etage, donc le controle passait en ne controlant
    # rien. N'importe quelle version Cresius est cherchee.
    $admises = @([regex]::Matches($t, "'(0\.18\.\d+(\.\d+)? Cresius)'") | ForEach-Object { $_.Groups[1].Value } | Select-Object -Unique)
    Write-Output ("     versions citees : " + ($admises -join ' / '))
    $ligneAdmises = ($t -split "`n" | Where-Object { $_ -match '\$Accepte\s*=\s*@' } | Out-String)
    foreach ($v in $admises) {
      if ($v -eq $WantV) { continue }
      if ($ligneAdmises -notmatch [regex]::Escape("'$v'")) { Write-Output "     (!) $v citée sans etre dans la liste admise" ; $pb++ }
    }
  }
}
# Les lanceurs ne sont pas nommes ici : le paquet peut n'en contenir qu'un, ou
# trois (une machine par materiel). On enumere ce qui existe.
$lanceursAVerifier = @($fichiers | Where-Object { $_ -like 'lanceurs\Lancer-*' })
foreach ($f in $lanceursAVerifier) {
  $t = [IO.File]::ReadAllText((Join-Path $dossier $f), [Text.Encoding]::UTF8)
  $lock = ([regex]::Match($t, '\$expectedVersion\s*=\s*''([^'']*)''')).Groups[1].Value
  Write-Output ("  {0} : verrou = « {1} »" -f $f, $lock)
  if (-not $WantV) { Write-Output '     SAUTE — pas de binaire a cote pour comparer' ; continue }
  if ($lock -ne $WantV) { Write-Output "     (!) le lanceur refuserait de demarrer le binaire du patch" ; $pb++ }
}

Write-Output ""
Write-Output "== etapes du script de patch =="
Select-String -LiteralPath (Join-Path $dossier 'Appliquer-Patch-Canal.ps1') -Pattern 'Log "== ' | ForEach-Object { Write-Output ("  " + $_.Line.Trim('"').Trim()) }
Write-Output "== garde-fous du script de patch =="
Select-String -LiteralPath (Join-Path $dossier 'Appliquer-Patch-Canal.ps1') -Pattern 'REFUSE|DEJA A JOUR|continue' | ForEach-Object { Write-Output ("  ligne {0}: {1}" -f $_.LineNumber, $_.Line.Trim()) }

Write-Output ""
Write-Output "== NOM_REGLE : le nom pose ne doit pas etre celui que le binaire efface =="
$oc = [IO.File]::ReadAllText((Join-Path $dossier 'Ouvrir-Canal.ps1'), [Text.Encoding]::UTF8)
Write-Output ("  prefixe pose    : " + ([regex]::Match($oc, '\$RulePrefix = ''([^'']*)''')).Groups[1].Value)
Write-Output ("  prefixe cabler  : " + ([regex]::Match([IO.File]::ReadAllText((Join-Path $dossier 'cabler-canal.ps1'), [Text.Encoding]::UTF8), '\$RulePrefix = ''([^'']*)''')).Groups[1].Value)
Write-Output "  nom efface par le binaire : 'AJEAN moteur (port ...)' (sys_firewall_windows.go:27)"
foreach ($f in @('Ouvrir-Canal.ps1', 'cabler-canal.ps1')) {
  $t = [IO.File]::ReadAllText((Join-Path $dossier $f), [Text.Encoding]::UTF8)
  if ($t -match "RulePrefix = 'AJEAN moteur") { Write-Output ("  (!) " + $f + " : MEME NOM, la regle serait supprimee a chaque coupure du pont") ; $pb++ }
}

Write-Output ""
Write-Output "== syntaxe netsh (remoteip est le seul mot-cle valide) =="
foreach ($f in @('Ouvrir-Canal.ps1', 'Ouvrir-PareFeu-Canal.cmd', 'cabler-canal.ps1')) {
  $t = [IO.File]::ReadAllText((Join-Path $dossier $f), [Text.Encoding]::UTF8)
  $mauvais = [regex]::Matches($t, 'remoteaddr(ess)?\b')
  # Les mentions en commentaire sont legitimes (elles disent ce QUI NE MARCHE PAS) ;
  # on ne refuse que si un mot-cle invalide est utilise dans une commande.
  $lignesCommande = @()
  foreach ($m in $mauvais) {
    $line = ($t.Substring(0, $m.Index) -split "`n")[-1]
    if ($line.TrimStart() -notmatch '^\s*#' -and $line.TrimStart() -notmatch '^\s*rem\b' -and $line -notmatch 'Note "') { $lignesCommande += $line.Trim() }
  }
  $bon = [regex]::Matches($t, 'remoteip\s*=\s*localsubnet').Count
  $utiliseNetsh = ($t -match 'netsh\s+advfirewall')
  Write-Output ("  {0} : remoteip=localsubnet x{1} | commandes avec mot-cle invalide x{2}" -f $f, $bon, $lignesCommande.Count)
  foreach ($l in $lignesCommande) { Write-Output ("     (!) " + $l) ; $pb++ }
  # Un outil qui ne perce pas le pare-feu n'a rien a ecrire en remoteip : l'exiger
  # aurait fait echouer cabler-canal.ps1 pour rien (fausse alerte mesuree le 2026-10-27).
  if ($bon -eq 0 -and $utiliseNetsh) { Write-Output "     (!) netsh appele sans remoteip=localsubnet" ; $pb++ }
}

Write-Output ""
Write-Output "== collisions de casse (le piege du switch ecrase par une variable) =="
foreach ($f in $fichiers) {
  $p = Join-Path $dossier $f
  if (-not (Test-Path -LiteralPath $p)) { continue }
  # Le detecteur est obligatoire : Verifier-Collisions-Casse.ps1 exige -Chemin.
  $sortie = & (Join-Path $dossier 'Verifier-Collisions-Casse.ps1') -Chemin $p 2>&1
  $lignes = @($sortie | ForEach-Object { ("" + $_) })
  foreach ($l in $lignes) {
    # -match ignore la casse : « aucune collision de casse » contenait COLLISION
    # et faisait compter une collision là où le detecteur n'en voyait aucune
    # (fausse alerte mesuree le 2026-10-27). -cmatch est sensible a la casse.
    if ($l -cmatch 'COLLISION') { Write-Output ("  (!) " + $f + " — " + $l.Trim()) ; $pb++ }
  }
  Write-Output ("  {0} : {1}" -f $f, (($lignes | Select-Object -Last 1)))
}

Write-Output ""
Write-Output "== binaire du paquet =="
# Le controle qui manque le plus : le 2026-10-27, un poseur de BOM ecrit a la hate
# a reencode ajean.exe comme un fichier texte (29 027 591 octets au lieu de
# 19 946 496, plus aucune version lisible). Sans ce controle, le paquet partait
# casse et le script d'application l'aurait quand meme pose — il ne verifie que la
# version annoncee, et un exe muet n'en annonce aucune.
$exePaquet = Join-Path $dossier 'ajean.exe'
if (-not (Test-Path -LiteralPath $exePaquet)) {
  Write-Output "  (!) ajean.exe ABSENT du paquet : Appliquer-Patch-Canal.ps1 refusera de tourner" ; $pb++
} else {
  $i = Get-Item -LiteralPath $exePaquet
  $v = [Diagnostics.FileVersionInfo]::GetVersionInfo($i.FullName)
  Write-Output ("  taille={0} o | Produit=« {1} » | Fichier=« {2} »" -f $i.Length, $v.ProductVersion, $v.FileVersion)
  Write-Output ("  sha256=" + (Get-FileHash -LiteralPath $exePaquet -Algorithm SHA256).Hash)
  if ($v.ProductVersion -ne $WantV -or $v.FileVersion -ne $WantV) {
    Write-Output "     (!) le binaire n'annonce pas $WantV des deux cotes : soit il est abime, soit ce n'est pas le bon" ; $pb++
  }
  # Un .exe commence par « MZ ». C'est la trace exacte de la corruption par
  # reencodage : les octets de tete deviennent du texte.
  $tete = [IO.File]::ReadAllBytes($exePaquet)[0..1]
  if (-not ($tete[0] -eq 0x4D -and $tete[1] -eq 0x5A)) {
    Write-Output ("     (!) tete du fichier = " + ($tete -join ',') + " au lieu de 77,90 (« MZ ») : le binaire a ete touche par un outil texte")
    $pb++
  }
}

Write-Output ""
Write-Output "== ce que le poste voisin sert reellement =="
# ATTENTION, correction du 2026-10-27. La version precedente de ce bloc concluait
# « 404 sur /api/network donc le voisin n'a pas le patch ». C'ETAIT FAUX, et le
# controleur sortait PROBLEMES=1 pour rien. Mesure :
#     parallelBridgeHandler() (parallel_bridge.go) ne sert QUE /v1/* :
#         if !strings.HasPrefix(r.URL.Path, "/v1/") -> 404 « surface OpenAI uniquement »
#     Donc /api/bridge, /api/peers, /api/network renvoient 404 sur le PORT DU PONT
#     (18190) MEME sur un binaire a jour — verifie ici, sur 0.18.15.3 : 404 aussi.
#   Ces routes ne vivent que sur le port de l'INTERFACE (18090), que le voisin
#   tient ferme (Test-NetConnection 18090 = filtre). Le seul test valide a distance
#   passe donc par /v1/* et est COMPORTEMENTAL : la carte d'identite injectee par
#   bridgeDeclareSelf ne part que si l'appelant porte l'en-tete X-Ajean-Canal.
#   On pose la meme question avec et sans en-tete et on compare les prompt_tokens :
#   la carte ajoute ~230 tokens. Mesure du 2026-10-27 chez bekka : 91 sans, 324 avec.
# $listeVoisins = @($listeVoisins) aurait ete le piege deja ecrit ailleurs dans ce
# projet : sur une variable non definie, @($undefine) rend @($null) et son Count
# vaut 1 — le bloc « aucun voisin » etait alors saute, et le foreach sur $null
# n'affiche RIEN du tout (le pipeline avale $null). Le controleur de collisions
# de casse a justement empeche le jumeau $Voisins/$voisins ; le parametre s'appelle
# donc AdresseVoisin et ne se melange jamais a la variable locale.
$listeVoisins = @($AdresseVoisin)
# Les voisins declares vivent dans la base de l'installation, joignables par
# /api/peers sur le port de l'interface (18090). C'est de la qu'on les lit : une
# liste figee dans le paquet serait fausse des qu'un voisin est ajoute ou retire.
if (-not $listeVoisins.Count) {
  try {
    $rep = Invoke-RestMethod -Uri 'http://127.0.0.1:18090/api/peers' -TimeoutSec 5
    foreach ($pair in @($rep.peers)) {
      if ($pair.url) { $listeVoisins += (($pair.url -replace '/v1\s*$', '') -replace '/$', '') }
    }
  } catch { }
}
if (-not $CleCanal) { $CleCanal = $env:AJEAN_CLE_CANAL }

if (-not $listeVoisins.Count) {
  Write-Output "  aucun voisin declare (ni par -AdresseVoisin, ni par /api/peers) — route non testee a distance"
} else {
  foreach ($v in ($listeVoisins | Select-Object -Unique)) {
    # Etape 1 : le pont est-il vivant et protege ? 401 = oui (il refuse sans cle).
    $code = $null
    try {
      $rep = Invoke-WebRequest -Uri ($v + '/v1/models') -TimeoutSec 6 -UseBasicParsing
      $code = [int]$rep.StatusCode
    } catch {
      if ($_.Exception.Response) { $code = [int]$_.Exception.Response.StatusCode }
    }
    if ($null -eq $code) { Write-Output ("  {0} : pont injoignable" -f $v) ; continue }
    if ($code -eq 401 -or $code -eq 403) {
      Write-Output ("  {0} : /v1/models = {1} -> pont vivant, porte la cle" -f $v, $code)
    } elseif ($code -eq 200) {
      Write-Output ("  {0} : /v1/models = 200 SANS CLE -> le pont de ce poste est ouvert a tout le monde" -f $v)
      $pb++
      continue
    } else { Write-Output ("  {0} : /v1/models = {1} (inattendu)" -f $v, $code) ; $pb++ ; continue }

    # Etape 2 : la carte d'identite. Sans cle du voisin, impossible : on le dit,
    # mais ce n'est pas une faute du paquet.
    if (-not $CleCanal) {
      Write-Output ("  {0} : carte d'identite NON TESTEE — donner la cle du voisin (-CleCanal ou $env:AJEAN_CLE_CANAL)" -f $v)
      continue
    }
    $q = 'Reponds en deux phrases, sans inventer : sur quelle machine tournes-tu, et as-tu des outils (shell, disque, memoire) ?'
    function Test-Complet($url, $avecEnTete, $prompt) {
      $corps = @{ model = 'ajean' ; messages = @(@{ role = 'user' ; content = $prompt }) ; max_tokens = 160 ; temperature = 0.2 } | ConvertTo-Json -Depth 5 -Compress
      $entetes = @{ Authorization = ('Bearer ' + $CleCanal) }
      if ($avecEnTete) { $entetes['X-Ajean-Canal'] = '1' }
      try {
        $r = Invoke-RestMethod -Uri $url -Method Post -Body $corps -ContentType 'application/json' -Headers $entetes -TimeoutSec 180
        $txt = ''
        if ($r.choices -and $r.choices[0].message -and $r.choices[0].message.content) { $txt = $r.choices[0].message.content }
        elseif ($r.choices -and $r.choices[0].text) { $txt = $r.choices[0].text }
        $tok = $null
        if ($r.usage -and $null -ne $r.usage.prompt_tokens) { $tok = [int]$r.usage.prompt_tokens }
        return [pscustomobject]@{ ok = $true ; texte = $txt ; tokens = $tok }
      } catch {
        $msg = $_.Exception.Message
        if ($_.Exception.Response) { $msg = 'HTTP ' + [int]$_.Exception.Response.StatusCode }
        return [pscustomobject]@{ ok = $false ; texte = $msg ; tokens = $null }
      }
    }
    $url = ($v.TrimEnd('/') + '/v1/chat/completions')
    $sans = Test-Complet $url $false $q
    $avec = Test-Complet $url $true $q
    if (-not $avec.ok) { Write-Output ("  {0} : completions refusees ({1}) — carte non verifiable" -f $v, $avec.texte) ; $pb++ ; continue }
    $delta = $null
    if ($null -ne $avec.tokens -and $null -ne $sans.tokens) { $delta = $avec.tokens - $sans.tokens }
    $connait = ($avec.texte -match 'Ajean|AJEAN|poste|reseau|pont|aucun outil|pas d.outils|no tools|machine')
    Write-Output ("  {0} : prompt_tokens sans en-tete={1} avec={2} (delta={3})" -f $v, $sans.tokens, $avec.tokens, $delta)
    $apercu = ($avec.texte -replace '\s+', ' ')
    if ($apercu.Length -gt 160) { $apercu = $apercu.Substring(0, 160) + '…' }
    Write-Output ("     reponse avec en-tete : " + $apercu)
    if ($connait -and ((($null -ne $delta) -and $delta -gt 100) -or ($avec.tokens -ge 300))) {
      Write-Output ("     -> CARTE D'IDENTITE EN PLACE la-bas : le modele prete sait qu'il est prete.")
    } elseif ($connait) {
      Write-Output ("     -> indice partiel : la reponse parle de la machine / de l'absence d'outils, mais le delta de tokens ne le confirme pas. A relire a la main.")
    } else {
      Write-Output ("     -> LA-BAS LA CARTE N'ARRIVE PAS : le modele prete ne sait toujours pas ou il est. Binaire anterieur a la revision 3." -f $v)
      $pb++
    }
  }
}

Write-Output ""
if ($pb -eq 0) { Write-Output "TOUT_VERT" } else { Write-Output ("PROBLEMES=" + $pb) ; exit 1 }
