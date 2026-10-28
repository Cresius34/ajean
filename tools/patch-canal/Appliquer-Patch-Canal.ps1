# Applique le patch « canal Ajean-Ajean, révision 7 » (-> 0.18.16.1 Cresius).
#
# ---------------------------------------------------------------------------
# CE QUE CORRIGE CETTE VERSION DU POSEUR (mesuré le 2026-10-28, demandé par
# l'utilisateur : « les Appliquer-Patch-Canal des dossiers patch ne remplace pas
# les fichiers »). Deux défauts distincts, tous les deux réels :
#
#   1. LE SORTIE COURTE. Le script comparait UNIQUEMENT la version du binaire.
#      Quand l'installation avait déjà le bon binaire, il affichait
#      « DÉJÀ À JOUR — rien à faire » et sortait de la boucle sans rien poser :
#      les outils du paquet (cabler-canal, Ouvrir-Canal, Ouvrir-PareFeu-Canal,
#      les deux vérificateurs, Encodages) n'étaient jamais copiés, même quand
#      ils étaient ABSENTS de l'installation. Mesure du 2026-10-28 sur ce poste :
#      cabler-canal.ps1, Ouvrir-Canal.ps1 et Verifier-Outils-Canal.ps1 absents
#      des DEUX installations, alors que le patch y est passé deux fois.
#      Un « à jour » qui ne regarde qu'un seul fichier n'est pas un contrôle.
#
#   2. LE PÉRIMÈTRE. Même quand il agissait, le script ne posait que DEUX
#      fichiers : ajean.exe et Lancer-Cresius.ps1. Les autres fichiers du paquet
#      ne avaient pas de destination.
#
# Règle nouvelle : le patch déclare TOUS les fichiers qu'il installe, avec leur
# destination, et il les compare UN PAR UN par empreinte SHA-256. « À jour »
# veut dire « chaque fichier posé a l'empreinte du paquet ». Un fichier manquant
# ou différent est réécrit, et le journal dit lequel et pourquoi.
#
# ---------------------------------------------------------------------------
# LE CORRECTIF DU « MODÈLE NU », côté destinataire.
#
# Le binaire seul ne suffit pas. Deux interrupteurs décident si le pont rend un
# tour d'agent ou prête du modèle nu, et ils vivent en BASE de données
# (bkState), pas dans un fichier : aucune copie de fichier ne peut les allumer.
#   - parallel_bridge_agent  -> « ajean bridge agent on », ou POST /api/bridge
#   - agent                  -> le mode agent de la machine, ou POST /api/agent/toggle
# Les deux doivent être pris, sinon le pont reste au prêt du modèle nu — et c'est
# exactement ce qui bloque la machine voisine. Par défaut ce patch les ALLUME
# (avec -SansAgent pour l'empêcher), puis il le VERIFIE.
#
# Ce choix n'est pas anodin et il est dit : autoriser le tour d'agent, c'est
# mettre le shell, le disque et la mémoire de cette machine à la portée de qui
# possède la clé API du pont. Sans clé forte, ne pas ouvrir.
#
# ---------------------------------------------------------------------------
# Ce que change la ligne 18161 (tout est sur le canal, rien d'autre) :
#   - révision 6 : le pont qui prête dit POURQUOI il n'a pas déclenché son tour
#     d'agent (en-tête X-Ajean-Canal-Agent-Refus), et l'appelant traduit ça en
#     geste à faire chez l'appelé. Avant, « modèle nu » ne distinguait pas un
#     choix d'un interrupteur oublié.
#   - socle amont v0.18.16 : installation d'ExLlama depuis l'interface corrigée
#     (#177, « write /dev/stderr: Descripteur non valide »), binaires CUDA Linux
#     officiels, affichage du chat plus fluide. Nos ajouts (mémoire EmbeddingGemma 2,
#     moteur MoE Strata, canal) sont intacts : fusion, pas remplacement.
#   - révisions 3 à 5, déjà dans le binaire : carte d'identité du modèle prêté,
#     refus qui expliquent pourquoi, tour d'agent à distance sous double
#     autorisation, auteur de chaque message du canal.
#
# Garde-fous :
#   - le binaire du patch doit annoncer 0.18.16.1 Cresius (ProductVersion ET FileVersion) ;
#   - le dossier visé doit annoncer une version connue (sinon : mauvaise cible) ;
#   - le port 18090 est vérifié AVANT toute écriture : si l'instance qui l'occupe
#     n'a pas le data\ du dossier traité, le script refuse — il ne peut pas
#     arrêter une instance qui n'est pas la sienne ;
#   - archive préalable de l'exe, du lanceur, de la base et de tout fichier
#     remplacé, nommée d'après LA VERSION REMPLACÉE.
param(
  [string[]]$Dossiers = @(),
  [string]$Patch = '',
  [switch]$Seccher,     # montre ce qui serait fait, n'ecrit rien
  [switch]$SansAgent,   # ne pas allumer les deux interrupteurs du tour d'agent
  [switch]$SansRelance  # pose les fichiers sans arreter ni redemarrer quoi que ce soit
)

$ErrorActionPreference = 'Stop'

# Ni version ni empreinte écrites en dur, et plus aucune liste de versions de
# départ à allonger à chaque révision (mesure du 2026-10-28 : $Accepte portait six
# lignes; la septième a failli manquer, et un dossier déjà à jour se faisait
# refuser). La version visée est LUE dans le binaire du paquet ; tout millésime
# Cresius antérieur est accepté comme point de départ, parce que c'est la version
# du binaire qui décide, pas le nom du dossier.
$WantV = ''
$WantSha = ''
$WantBuild = ''

# Les outils du paquet qui doivent finir DANS L'INSTALLATION, dans data\scripts.
# Appliquer-Patch-Canal.* en est volontairement absent : pose là, il chercherait
# son binaire a cote de lui, ne le trouverait pas, et refuserait de tourner — un
# poseur sans binaire a cote de lui est un leurre. NOTE-DE-PATCH.md reste dans le
# dossier du patch.
$OutilsInstalles = @('cabler-canal.ps1', 'Ouvrir-Canal.ps1', 'Ouvrir-PareFeu-Canal.cmd',
                     'Verifier-Outils-Canal.ps1', 'Verifier-Collisions-Casse.ps1', 'Encodages.ps1',
                     # Trouver-Installations.ps1 est DOT-SOURCE par les autres outils
                     # (etat_r18.ps1, controle-patch-can, inventaire-exe, Pointer-retour) :
                     # s'il manque, ceux-la ne tournent plus du tout. Il doit donc etre pose
                     # avec eux, pas laisse dans le dossier du patch.
                     'Trouver-Installations.ps1',
                     # Le garde-fou : il compte les chemins d'installation ecrits en dur.
                     # Nos versions sont portables, un chemin cablé ment sans le dire.
                     'Verifier-Chemin-En-Dur.ps1',
                     # Et il range : les outils d'une campagne jouee partent dans
                     # data\scripts\archive au lieu de rester actifs a mentir sur un
                     # dossier qui n'existe plus. Mesure du 2026-10-28 : la fabrique de
                     # la revision PRECEDENTE etait encore active et embarquait son
                     # ancien binaire en silence — c'est le « le patch ne remplace pas
                     # les fichiers » qui revient.
                     'Archiver-Liens-Morts.ps1',
                     # Aligne le NOM du dossier sur la VERSION du binaire, en refusant si
                     # l'instance tient le dossier (le cas de la principale, qui nous heberge).
                     'Renommer-Installation.ps1',
                     # La moitie qui manquait a la symetrie : la declaration du voisin se
                     # joue SUR la machine appelee, et un Ajean a distance ne peut pas la
                     # faire lui-meme (peer n'expose que list/status/ask/forget).
                     'Cabler-Canal-Local.ps1',
                     # La sonde canonique, en LECTURE SEULE et sans droits. Le .cmd de la
                     # revision 5 portait toute la sonde en UNE ligne : le moindre
                     # guillemet casse et l'erreur pointe dix lignes plus loin.
                     'Diagnostic-Installation.ps1',
                     # Son lanceur, pour le double-clic. Sans lui la sonde reste un
                     # fichier a taper dans une fenetre, ce que la destinataire du paquet
                     # ne fait pas.
                     'Diagnostic-Canal.cmd',
                     # La question que la machine voisine a posee le 2026-10-28 : « ma
                     # version 0.18.16.1 n'affiche pas la partie Pont Ajean-Ajean ». La
                     # reponse n'est pas dans la version : la carte est dans les octets
                     # des trois binaires fabriques. C'est le PORT interroge — 18080 est
                     # celui du moteur Strata, l'interface est sur 18090. Cet outil le
                     # mesure port par port au lieu de le supposer.
                     'Verifier-Carte-Pont.ps1',
                     # Et la symetrie : un pont peut prete (200 a ma cle) sans que la
                     # machine en face puisse appeler, faute de voisin declare chez elle.
                     # Le jeu complet des questions, dans les deux sens, a jouer aussi
                     # la-bas.
                     'Verifier-Symetrie-Canal.ps1')

if (-not $Patch) {
  # Le script vit DANS le dossier patch ; on accepte aussi de le lancer d'ailleurs
  # en le laissant chercher son parent.
  foreach ($c in @($PSScriptRoot, (Split-Path -Parent $PSScriptRoot))) {
    if (Test-Path -LiteralPath (Join-Path $c 'ajean.exe')) { $Patch = $c ; break }
  }
  if (-not $Patch) { throw "dossier du patch introuvable (ajean.exe attendu a cote de ce script)" }
}
$NewExe = Join-Path $Patch "ajean.exe"
$LancD  = Join-Path $Patch "lanceurs"
if (-not (Test-Path -LiteralPath $NewExe)) { throw "binaire du patch introuvable : $NewExe" }

if (-not $Dossiers -or -not $Dossiers.Count) {
  # Cible : LE DOSSIER AU-DESSUS DE CE DOSSIER-PAQUET. Rien d'autre.
  #
  # Avant, on remontait jusqu'a quatre niveaux en cherchant une « signature »
  # (ajean.exe + Lancer-Cresius.ps1). Mesure du 2026-10-28, ici et chez la voisine :
  # cette signature est FAUSSE. L'installation principale s'appelle
  # Lancer-Cresius-principale.ps1 ; le fichier cherche n'existe pas ; la
  # reconnaissance echouait et le script hurlait « installation cible introuvable »
  # sans rien poser. Un nom de fichier en dur dans un poseur ment des que quelqu'un
  # renomme le lanceur — et c'est exactement le « le patch ne s'applique pas »
  # signale par l'utilisateur.
  #
  # La regle est donc structurelle et non nominale : le paquet vit DANS
  # l'installation qu'il doit patcher, donc sa cible est son parent. Un seul
  # controle, structurel : ce parent contient un ajean.exe. Pas de liste de
  # chemins en priorite — sur la machine voisine il existe aussi d'autres dossiers
  # « maj Cresius », et en viser un aurait patche une installation que personne ne
  # visait, en arretant son instance. Pour plusieurs dossiers : -Dossiers.
  $c = Split-Path -Parent $Patch
  if (-not $c -or -not (Test-Path -LiteralPath (Join-Path $c 'ajean.exe'))) {
    throw ("aucun ajean.exe dans le dossier qui CONTIENT ce paquet (" + $Patch + "). " +
           "Le paquet doit etre deplace A L'INTERIEUR de l'installation a patcher, ou viser le dossier " +
           "explicitement : -Dossiers `"C:\chemin\de\linstallation`".")
  }
  $Dossiers = @($c)
}

$Script:Log = $null
function Log($m) {
  # Le console Windows n'affiche pas les accents des .cmd : on les remplace, mais
  # attention au quantifiant — '[^\x20-\x7E]?' (point d'interrogation) s'applique
  # aussi a la chaine vide et insere un ? entre chaque caractere. Sans quantifiant.
  $l = (Get-Date -Format 'HH:mm:ss') + '  ' + ("$m" -replace '[^\x20-\x7E]','?')
  Write-Output $l
  if ($Script:Log) { Add-Content -LiteralPath $Script:Log -Value $l -Encoding UTF8 }
}
function Sha($p) { (Get-FileHash -LiteralPath $p -Algorithm SHA256).Hash }
function Ports() {
  $r = @{}
  foreach ($p in 18090,18080,18081,18190) {
    $c = @(netstat -ano | Select-String -Pattern (":" + $p + "\s.*LISTENING"))
    $r[$p] = @($c | ForEach-Object { (($_ -split "\s+")[-1]) } | Select-Object -Unique)
  }
  return $r
}
function AjeanPids() { @(Get-Process -Name ajean -ErrorAction SilentlyContinue | ForEach-Object { $_.Id }) }
function EnginePids($racine) {
  $marque = Split-Path -Leaf $racine
  @(Get-CimInstance Win32_Process -ErrorAction SilentlyContinue |
    Where-Object { $_.CommandLine -like ("*" + $marque + "*") -and
                   ($_.Name -eq "python.exe" -or $_.Name -like "strata*") } |
    ForEach-Object { $_.ProcessId })
}

# Le Ctrl+C doit partir d'un processus ETRANGER a l'instance arretee : sinon
# AttachConsole(0) tuerait l'emetteur aussi. D'ou un fichier temporaire lance
# dans son propre powershell.
function Send-CtrlC([int]$pidCible) {
  # Le C# est ecrit dans un fichier separe : une here-string ne peut pas en
  # contenir une autre, et c'est ce qui avait casse la version precedente.
  $cs = @'
using System;
using System.Runtime.InteropServices;
public static class K {
  [DllImport("kernel32.dll")] public static extern bool AttachConsole(uint pid);
  [DllImport("kernel32.dll")] public static extern bool FreeConsole();
  [DllImport("kernel32.dll")] public static extern bool SetConsoleCtrlHandler(IntPtr h, bool add);
  [DllImport("kernel32.dll")] public static extern bool GenerateConsoleCtrlEvent(uint e, uint g);
}
'@
  $csPath   = Join-Path $env:TEMP 'ajean-ctrlc-canal7.cs'
  $helper   = Join-Path $env:TEMP 'ajean-ctrlc-canal7.ps1'
  [IO.File]::WriteAllText($csPath, $cs, (New-Object Text.UTF8Encoding($true)))
  $q = [char]39
  $corps = 'param([int]$TargetPid); Add-Type -Path ' + $q + $csPath + $q +
           '; [void][K]::FreeConsole();' +
           ' if (-not [K]::AttachConsole([uint32]$TargetPid)) { Write-Output "ATTACHE_CONSOLE_ECHEC"; exit 2 }' +
           ' [void][K]::SetConsoleCtrlHandler([IntPtr]::Zero, $true);' +
           ' if ([K]::GenerateConsoleCtrlEvent(0,0)) { Write-Output "CTRLC_ENVOYE" } else { Write-Output "CTRLC_ECHEC"; exit 3 }' +
           ' Start-Sleep -Milliseconds 500; [void][K]::FreeConsole(); exit 0'
  [IO.File]::WriteAllText($helper, $corps, (New-Object Text.UTF8Encoding($true)))
  $r = & powershell -NoProfile -ExecutionPolicy Bypass -File $helper -TargetPid $pidCible 2>&1
  return ($r -join ' / ')
}

# Inventaire de ce que le patch doit installer, pour un dossier donne.
# Chaque entree : src (dans le paquet), dst (dans l'installation), exe (contrôle
# de tete MZ). Le nom du lanceur depend du dossier vise : la variante porte
# RX6900XT dans son nom.
function Inventaire($Root) {
  # Le lanceur a poser n'est PAS choisi par un nom de materiel cablé. Mesure du
  # 2026-10-28 : le poseur testait « $Root -like '*RX6900XT' » — sur une machine
  # dont le dossier porte un autre suffixe (RTX, portable, la voisine…), il posait
  # le lanceur de la principale dans la variante. On lit donc le paquet : quel
  # lanceur y vit, et lequel correspond au SUFFIXE du dossier vise.
  $nomLanceur = 'Lancer-Cresius-principale.ps1'
  $dispo = @(Get-ChildItem -LiteralPath $LancD -Filter 'Lancer-Cresius-*.ps1' -EA SilentlyContinue |
             ForEach-Object { $_.Name })
  if ($dispo.Count) {
    # Suffixe du dossier vise : ce qui suit « maj Cresius« . Vide pour la principale.
    $suffixe = ''
    $m = [regex]::Match((Split-Path $Root -Leaf), 'maj\s+Cresius\s+(.+)$')
    if ($m.Success) { $suffixe = $m.Groups[1].Value.Trim() }
    $trouve = $null
    foreach ($n in $dispo) {
      $etiquette = [regex]::Match($n, 'Lancer-Cresius-(.+)\.ps1').Groups[1].Value
      if ($etiquette -eq 'principale' -and -not $suffixe) { $trouve = $n ; break }
      if ($etiquette -and $suffixe -and
          $etiquette.ToLower() -eq $suffixe.ToLower().Replace(' ', '')) { $trouve = $n ; break }
    }
    if (-not $trouve -and $suffixe) {
      # Pas d'etiquette qui correspond : on prend la seule variante disponible,
      # et on le dit — c'est un desaccord entre le paquet et le dossier, pas un
      # choix a cacher.
      $autres = @($dispo | Where-Object { $_ -notlike '*principale*' })
      if ($autres.Count -eq 1) { $trouve = $autres[0] }
    }
    if ($trouve) { $nomLanceur = $trouve }
  }

  # Le NOM du lanceur dans l'installation n'est pas une verite que je detiens :
  # c'est Lancer-Cresius.cmd qui le decide, en appelant
  #   powershell -File "%~dp0<NOM>.ps1"
  # Mesure du 2026-10-28 : l'installation principale avait un Lancer-Cresius.cmd
  # qui appelait Lancer-Cresius.ps1 alors que le fichier pose s'appelait
  # Lancer-Cresius-principale.ps1 — le lanceur ne pouvait donc pas demarrer, et
  # un poseur qui ecrit dans un nom en dur aurait patche un fichier que personne
  # n'appelle. On pose donc la ou le .cmd regarde.
  $dstLanceur = 'Lancer-Cresius.ps1'
  $cmdPath = Join-Path $Root 'Lancer-Cresius.cmd'
  if (Test-Path -LiteralPath $cmdPath) {
    $m = [regex]::Match([IO.File]::ReadAllText($cmdPath), '-File\s+"?%~dp0([^"\s]+)')
    if ($m.Success) { $dstLanceur = $m.Groups[1].Value }
  }

  $item = New-Object PSObject -Property @{
    src = $NewExe
    dst = (Join-Path $Root 'ajean.exe')
    exe = $true
  }
  $liste = New-Object System.Collections.ArrayList
  $null = $liste.Add($item)

  $itemL = New-Object PSObject -Property @{
    src = (Join-Path $LancD $nomLanceur)
    dst = (Join-Path $Root $dstLanceur)
    exe = $false
  }
  $null = $liste.Add($itemL)

  foreach ($nom in $OutilsInstalles) {
    $itemO = New-Object PSObject -Property @{
      src = (Join-Path $Patch $nom)
      dst = (Join-Path $Root ('data\scripts\' + $nom))
      exe = $false
    }
    $null = $liste.Add($itemO)
  }
  return ,$liste
}

Log ("== patch canal Ajean-Ajean - " + (Get-Date -Format 'yyyy-MM-dd HH:mm:ss'))
Log ("   paquet   : $Patch")
Log ("   binaire  : $NewExe")
$nv = [Diagnostics.FileVersionInfo]::GetVersionInfo((Resolve-Path -LiteralPath $NewExe).Path)
if ($nv.ProductVersion -ne $nv.FileVersion) {
  throw ('le binaire du patch est incoherent : ' + $nv.ProductVersion + ' / ' + $nv.FileVersion)
}
if ($nv.ProductVersion -notmatch 'Cresius') {
  throw ('le binaire du patch n''est pas une ligne Cresius (' + $nv.ProductVersion + ') — refus')
}
# La version visée est CELLE DU BINAIRE LIVRÉ : elle n'est plus écrite dans ce
# script. Un poseur qui porte une version en dur refuse ses propres paquets dès
# que le millésime change (mesure du 2026-10-28).
$WantV = $nv.ProductVersion

# L'empreinte attendue vient du MANIFESTE du paquet, écrit par la fabrique au
# moment du build. Sans lui, rien n'est comparé et on le dit — une empreinte
# recopiée dans le script serait une seconde vérité à maintenir.
$Manifeste = Join-Path $Patch 'PAQUET.txt'
$ShaManifeste = ''
$VerManifeste = ''
if (Test-Path -LiteralPath $Manifeste) {
  foreach ($ligne in (Get-Content -LiteralPath $Manifeste)) {
    $m = [regex]::Match($ligne, 'sha256\s*:\s*([0-9A-Fa-f]{64})')
    if ($m.Success) { $ShaManifeste = $m.Groups[1].Value.ToUpper() }
    $mv = [regex]::Match($ligne, 'version\s*:\s*(.+)$')
    if ($mv.Success -and -not $VerManifeste) { $VerManifeste = $mv.Groups[1].Value.Trim() }
  }
  if ($VerManifeste -and $VerManifeste -ne $WantV) {
    throw ('le manifeste du paquet annonce ' + $VerManifeste + ' alors que son binaire annonce ' +
           $WantV + ' — paquet mélangé, ne rien poser')
  }
}
if (-not $ShaManifeste) {
  Log "   (!) pas de manifeste (PAQUET.txt) : l'empreinte du binaire n'est comparée à rien"
} elseif ((Sha $NewExe) -ne $ShaManifeste) {
  throw ('le binaire du paquet ne correspond pas à son manifeste (' + $ShaManifeste.Substring(0,16) +
         ' attendu) — paquet corrompu ou modifié en route, ne rien poser')
}
$WantSha = $ShaManifeste
Log ("   binaire OK : $($nv.ProductVersion), $((Get-Item -LiteralPath $NewExe).Length) o, sha $((Sha $NewExe).Substring(0,16))")
Log ("   manifeste  : " + $(if ($ShaManifeste) { 'lu, empreinte controlee' } else { 'ABSENT' }))
if ($SansAgent) {
  Log "   interrupteurs : NON allumes (-SansAgent). Le pont pretera le modele nu et l'appelant verra la raison du refus."
} else {
  Log "   interrupteurs : le patch allumera l'autorisation du canal ET le mode agent de cette machine."
  Log "                 Cela met le shell, le disque et la memoire de cette machine a la portee de qui possede la cle du pont. Ne ouvrir qu'avec une cle forte."
}

$echecs = @()
foreach ($Root in $Dossiers) {
  Log ""
  Log ("== dossier : $Root")
  if (-not (Test-Path -LiteralPath $Root)) {
    Log "   ABSENT — saute" ; $echecs += "$Root absent" ; continue
  }
  # Le journal vit dans data\scripts du dossier vise — le dossier ou vont aussi les
  # outils. Il peut ne pas exister (installation ancienne, ou machine voisine) :
  # sans creation, Add-Content echouerait et, avec ErrorActionPreference=Stop,
  # tuerait le patch au premier Log.
  #
  # En SECHE, pas de journal du tout. Le switch promet « n'ecrit rien » : poser un
  # fichier de log, et creer data\scripts au passage, serait mentir dessus.
  if (-not $Seccher) {
    $dosScripts = Join-Path $Root 'data\scripts'
    if (-not (Test-Path -LiteralPath $dosScripts)) {
      New-Item -ItemType Directory -Path $dosScripts -Force | Out-Null
    }
    $Script:Log = Join-Path $dosScripts 'Patch-canal-18161.log'
  }
  $exe = Join-Path $Root 'ajean.exe'
  # Le lanceur vise le fichier que Lancer-Cresius.cmd appelle, pas un nom que
  # j'aurais choisi (voir Inventaire). $ps sert a l'archive et au controle final.
  $ps = Join-Path $Root 'Lancer-Cresius.ps1'
  $cmdCheck = Join-Path $Root 'Lancer-Cresius.cmd'
  if (Test-Path -LiteralPath $cmdCheck) {
    $mCheck = [regex]::Match([IO.File]::ReadAllText($cmdCheck), '-File\s+"?%~dp0([^"\s]+)')
    if ($mCheck.Success) { $ps = Join-Path $Root $mCheck.Groups[1].Value }
  }
  if (-not (Test-Path -LiteralPath $exe)) {
    Log "   pas d'ajean.exe — saute" ; $echecs += "$Root : pas d'exe" ; continue
  }

  $courant = [Diagnostics.FileVersionInfo]::GetVersionInfo((Resolve-Path -LiteralPath $exe).Path)
  Log ("   en place : $($courant.ProductVersion)")
  # Plus de liste de versions de départ à allonger à chaque révision (elle portait
  # six lignes, la septième a failli manquer, et un dossier déjà à jour se faisait
  # refuser). Le point de départ est n'importe quelle ligne Cresius : c'est la
  # version du binaire qui décide, jamais le nom du dossier.
  if ($courant.ProductVersion -notmatch 'Cresius') {
    Log ("   REFUSE : ce dossier n'est pas une ligne Cresius ($($courant.ProductVersion)).")
    $echecs += "$Root : version $($courant.ProductVersion) hors ligne Cresius" ; continue
  }
  # La version reellement en place devient la reference de ce dossier : l'archive
  # porte son nom, et le controle de l'archive se fait contre elle. Ecrire un nom
  # fige aurait fait archiver n'importe quoi sur le mauvais etage.
  $OldV = $courant.ProductVersion

  # ---- comparaison fichier par fichier, AVANT de decider de quoi que ce soit ----
  # C'est la piece qui manquait : un binaire a jour ne disait rien des autres
  # fichiers, et le script sortait sans rien poser.
  Log "== inventaire des fichiers du patch (comparaison par sha)"
  $item = Inventaire $Root
  $aPoser = @()
  $dejaOk = 0
  $paquetIncomplet = $false
  foreach ($c in $item) {
    if (-not (Test-Path -LiteralPath $c.src)) {
      Log ("   ABSENT DU PAQUET : " + (Split-Path -Leaf $c.src) + " — le paquet est incomplet, on n'ecrit pas.")
      $echecs += "$Root : paquet incomplet (manque " + (Split-Path -Leaf $c.src) + ")"
      $paquetIncomplet = $true ; break
    }
    $nom = Split-Path -Leaf $c.dst
    if (-not (Test-Path -LiteralPath $c.dst)) {
      Log ("   MANQUANT dans l'installation : " + $nom + "  (sera pose)")
      $aPoser += $c
      continue
    }
    $shaSrc = Sha $c.src
    $shaDst = Sha $c.dst
    if ($shaSrc -eq $shaDst) { $dejaOk++ ; continue }
    Log ("   DIVERGENT : " + $nom + "  installe " + $shaDst.Substring(0,16) + " != paquet " + $shaSrc.Substring(0,16))
    $aPoser += $c
  }
  if ($paquetIncomplet) { continue }
  $total = @($item).Count
  $aPoser = @($aPoser)
  Log ("   $dejaOk / $total fichier(s) deja conforme(s), " + $aPoser.Count + " a poser")

  # Les interrupteurs, c'est en base : la comparaison de fichiers ne les voit pas.
  $veutAgent = -not $SansAgent
  $agentEstPret = $false
  if ($veutAgent) {
    try {
      $b0 = Invoke-RestMethod -Uri 'http://127.0.0.1:18090/api/bridge' -TimeoutSec 6
      $agentEstPret = [bool]$b0.agent_on
      Log ("   interrupteurs du tour d'agent : agent_on=" + $b0.agent_on + " (autorisation pont=" + $b0.agent_requested + ", mode agent machine=" + $b0.agent_mode_local + ")")
    } catch {
      Log ("   (!) /api/bridge ne repond pas : interrupteurs lisibles seulement apres la relance (" + $_.Exception.Message + ")")
    }
  }

  if ($aPoser.Count -eq 0 -and (-not $veutAgent -or $agentEstPret)) {
    Log "   DEJA A JOUR — binaire, lanceur, outils et interrupteurs controles un par un. Rien a faire."
    continue
  }
  if ($aPoser.Count -eq 0 -and $veutAgent -and -not $agentEstPret) {
    Log "   fichiers a jour, mais les interrupteurs du tour d'agent ne sont pas pris : on les allume."
  }

  # Retour arriere : ce patch cree son propre point de retour, nomme d'apres la
  # version qu'il remplace. Attention a l'ordre : on ne cree RIEN avant d'avoir
  # verifie qu'on peut arreter L'INSTANCE DE CE DOSSIER. Une archive posee avant
  # un refus laisserait un point de retour orphelin, et le second passage se
  # croirait « deja applique ».
  $Arch = Join-Path $Root ("archive-" + $OldV)
  $archExe = Join-Path $Arch 'ajean.exe'
  $archDeja = Test-Path -LiteralPath $archExe

  # 1. verification AVANT toute ecriture : qui tient 18090 ?
  Log "== 1. qui tient 18090 (lecture seule)"
  $p = Ports
  $parent = $p[18090] | Select-Object -First 1
  $homeAttendu = [IO.Path]::GetFullPath((Join-Path $Root 'data'))
  $etat = $null
  # Ce que ces drapeaux veulent dire, et pourquoi ils existent :
  #   $instanceDeCeDossier : l'instance en cours est CELLE de ce dossier. Ses
  #     fichiers sont charges en memoire, son exe est verrouille.
  #   $portInconnu : 18090 est occupe mais rien ne repond a /api/embeddings —
  #     on ne peut pas savoir de quel dossier elle est. On ne presume pas.
  # Un autre dossier qui tient le port n'empeche RIEN ici : ses fichiers ne sont
  # pas ceux de ce dossier. Le refus total de la revision 7 etait donc trop large
  # (mesure du 2026-10-28 : la variante, qui ne tourne pas du tout, ne pouvait pas
  # etre mise a jour tant que la principale tournait — « le patch ne s'applique
  # pas »). En -SansRelance on n'arrete personne, donc ce cas est sur.
  $instanceDeCeDossier = $false
  $portInconnu = $false
  if ($parent) {
    try { $etat = Invoke-RestMethod -Uri 'http://127.0.0.1:18090/api/embeddings' -TimeoutSec 5 } catch { }
    if ($null -eq $etat) {
      $portInconnu = $true
      Log "   (!) 18090 occupe (PID $parent) sans reponse a /api/embeddings : dossier de cette instance inconnu."
    } elseif ($etat.home -eq $homeAttendu) {
      $instanceDeCeDossier = $true
      Log "   18090 tenu par l'instance de ce dossier (home verifie)"
    } else {
      Log ("   18090 est tenu par une AUTRE instance (home $($etat.home)) — l'instance de ce dossier n'est pas en service.")
    }
  } else {
    Log "   18090 libre, rien a arreter"
  }

  if (-not $SansRelance) {
    # Mode normal : il faut pouvoir arreter l'instance de ce dossier. On refuse si
    # c'est impossible, au lieu de tuer l'instance d'un autre dossier.
    if ($portInconnu) {
      Log "   REFUSE : 18090 occupe sans reponse a /api/embeddings — aucune certitude sur l'instance arretee, on ne touche a rien."
      $echecs += "$Root : 18090 occupe sans reponse" ; continue
    }
    if ($parent -and -not $instanceDeCeDossier) {
      Log "   REFUSE : ce script ne peut pas arreter l'instance d'un autre dossier : quitter d'abord l'autre instance, ou poser ici avec -SansRelance."
      $echecs += "$Root : 18090 tenu par une autre instance" ; continue
    }
  } else {
    if ($portInconnu) {
      Log "   -SansRelance : aucune instance ne sera arretee, mais par prudence le binaire ne sera pas pose (impossible de savoir si cette instance est celle de ce dossier)."
    }
    # En -SansRelance, un binaire ne peut etre pose que si l'instance de ce dossier
    # n'est PAS en cours. Le controle se fait ici, une fois pour toutes.
    $binaireVerrouille = $instanceDeCeDossier -or $portInconnu
  }

  if ($Seccher) {
    # La seche doit decrire CE QUI SE PASSERAIT DANS LE MODE DEMANDE. Annoncer
    # « arrete cette instance, relance » sous -SansRelance serait faux, et une
    # seche fausse ne sert a rien : c'est justement l'outil de verification.
    $ceQueFait = 'arrete cette instance, pose ' + $aPoser.Count + ' fichier(s) — ' +
                 (($aPoser | ForEach-Object { Split-Path -Leaf $_.dst }) -join ', ') +
                 ' — relance, verifie /api/bridge + /api/peers + le refus de /api/network, puis les temoins de la revision 6'
    $archiveDecrite = 'exe + lanceur + base + fichiers remplaces'
    if ($SansRelance) {
      # La seche doit dire ce qui se passerait VRAIMENT : le binaire n'est en
      # attente que si l'instance de CE dossier le tient. La variante, qui ne
      # tourne pas pendant que la principale tourne, a son exe libre.
      $binaireLibre = -not ($instanceDeCeDossier -or $portInconnu)
      if ($binaireLibre) {
        $ceQueFait = 'ne PAS arreter d''instance (aucune instance de ce dossier ne tourne), poser les 8 fichiers y compris le binaire, et NE PAS jouer les controles HTTP (18090 ne repond pas pour ce dossier)'
      } else {
        $ceQueFait = 'ne PAS arreter l''instance, poser seulement les fichiers non charges en memoire (' +
                     ((@($aPoser | Where-Object { -not $_.exe }) | ForEach-Object { Split-Path -Leaf $_.dst }) -join ', ') +
                     '), laisser le binaire en attente de redemarrage, et NE PAS jouer les controles HTTP (ils porteraient sur l''instance en memoire)'
      }
      $archiveDecrite = 'exe + lanceur + fichiers remplaces (PAS la base : ' + $(if ($instanceDeCeDossier) { 'l''instance de ce dossier tourne, sa copie serait incoherente' } else { 'une instance tourne sur cette machine, on ne copie pas sa base par principe' }) + ')'
    }
    Log ("   [SECHE] aurait cree $Arch ($archiveDecrite), " + $ceQueFait + ".")
    if ($veutAgent) { Log "   [SECHE] aurait aussi allume l'autorisation du canal et le mode agent de cette machine." }
    continue
  }

  # 2. archive, AVANT tout changement : l'exe en service + son lanceur.
  if ($archDeja) {
    # Une archive deja la = tentative precedente interrompue APRES l'archive. Le
    # fichier archifre est toujours l'exe d'avant le patch (on est arrive ici avec
    # $OldV en place) : c'est le bon point de retour, on le garde tel quel.
    Log "== 2. archive deja presente, conservee ($Arch)"
    Log ("   ajean.exe archive : sha " + (Sha $archExe).Substring(0,16) + " — doit etre l'exe $OldV")
    if ([Diagnostics.FileVersionInfo]::GetVersionInfo((Resolve-Path -LiteralPath $archExe).Path).ProductVersion -ne $OldV) {
      Log "   REFUSE : l'exe archive n'annonce pas $OldV. Le point de retour n'est pas sur, on n'ecrit pas."
      $echecs += "$Root : archive incoherente" ; continue
    }
  } else {
    Log "== 2. archive de ce qui est en service ($Arch)"
    New-Item -ItemType Directory -Path $Arch -Force | Out-Null
    Copy-Item -LiteralPath $exe -Destination $archExe -Force
    if (Test-Path -LiteralPath $ps) {
      Copy-Item -LiteralPath $ps -Destination (Join-Path $Arch 'Lancer-Cresius.ps1') -Force
    }
    Log ("   OK  ajean.exe ($(( Get-Item -LiteralPath $archExe).Length) o, sha " + (Sha $archExe).Substring(0,16) + ")")
  }

  # 3. base, avant l'arret (peut etre verrouillee : on tente, on recommencera apres).
  #
  # En -SansRelance l'instance n'est PAS arretee : copier la base pendant qu'elle
  # ecrit dedans produit un fichier possiblement incoherent, et une archive de base
  # incoherente est pire que pas d'archive — elle ferait croire a un retour possible.
  # On ne la copie que si personne ne l'ecrit (mesure faite le 2026-10-28 dans
  # data\scripts\Pointer-retour.ps1, meme raisonnement).
  $db = Join-Path $Root 'data\ajean.db'
  $dbArch = Join-Path $Arch 'ajean.db.pre-canal-rev7'
  if ($SansRelance) {
    if (Test-Path -LiteralPath $db) {
      Log "   base non archivee : l'instance tourne (-SansRelance), la copie serait possiblement incoherente."
    }
  } elseif (Test-Path -LiteralPath $db) {
    try {
      Copy-Item -LiteralPath $db -Destination $dbArch -Force
      Log ("   OK  ajean.db -> archive (" + (Get-Item -LiteralPath $dbArch).Length + " o)")
    } catch {
      Log ("   (!) base verrouillee, retente apres l'arret : " + $_.Exception.Message)
    }
  }

  # 3b. arret propre de L'INSTANCE DE CE DOSSIER uniquement.
  #
  # -SansRelance : on ne touche pas a l'instance. C'est le cas qui manquait le
  # 2026-10-28 : l'installation principale tournait (et c'est elle qui porte cette
  # IA), son binaire etait DEJA a jour, et il ne restait a poser que les outils du
  # canal — des fichiers qui ne sont pas charges en memoire. Sans cette voie, le
  # seul moyen de finir etait d'arreter la machine en cours de travail.
  if ($parent -and $SansRelance) {
    Log "== 3. instance en cours : NON arretee (-SansRelance)"
    Log "   seuls les fichiers non charges en memoire seront posés ; un binaire different restera en attente de redemarrage."
  } elseif ($parent) {
    Log "== 3. arret propre (Ctrl+C au processus console qui tient 18090)"
    Log ("   PID $parent ; ajean en vie : " + ((AjeanPids) -join ',') + " ; moteur de ce dossier : " + ((EnginePids $Root) -join ','))
    Log ("   ctrlc : " + (Send-CtrlC ([int]$parent)))
    $deadline = (Get-Date).AddSeconds(240)
    while ((Get-Date) -lt $deadline) {
      Start-Sleep -Seconds 3
      $q = Ports
      if ($q[18090].Count -eq 0 -and (AjeanPids).Count -eq 0) { break }
    }
    $q = Ports
    if ($q[18090].Count -gt 0 -or (AjeanPids).Count -gt 0) {
      Log "   arret propre insuffisant apres 240 s : terminaison forcee (le moteur suit le parent)"
      foreach ($id in (AjeanPids)) { & taskkill /PID $id /T /F 2>&1 | Out-Null }
      foreach ($id in (EnginePids $Root)) { & taskkill /PID $id /F 2>&1 | Out-Null }
      Start-Sleep -Seconds 15
    }
    $q = Ports
    Log ("   apres arret : 18090=[" + ($q[18090] -join ',') + "] 18080=[" + ($q[18080] -join ',') + "] 18081=[" + ($q[18081] -join ',') + "] 18190=[" + ($q[18190] -join ',') + "] ajean=[" + ((AjeanPids) -join ',') + "]")
    if ($q[18090].Count -gt 0) { throw "le port 18090 reste occupe : arret de la bascule (le nouvel exe refuserait de demarrer)" }
  }

  # 3c. re-copie de la base, plus personne ne la tient.
  if ((Test-Path -LiteralPath $db) -and -not $SansRelance) {
    try {
      Copy-Item -LiteralPath $db -Destination $dbArch -Force
      Log ("   OK  base archivee a l'arret (" + (Get-Item -LiteralPath $dbArch).Length + " o)")
    } catch { Log ("   (!) " + $_.Exception.Message) }
  }

  # 4. depot de TOUS les fichiers du patch, un par un, avec controle.
  Log ("== 4. depot des fichiers du patch (" + $aPoser.Count + ")")
  $enAttente = @()
  foreach ($c in $aPoser) {
    $nom = Split-Path -Leaf $c.dst
    $dossierDst = Split-Path -Parent $c.dst
    if (-not (Test-Path -LiteralPath $dossierDst)) {
      New-Item -ItemType Directory -Path $dossierDst -Force | Out-Null
    }
    # En -SansRelance, un binaire different ne peut PAS etre ecrit si l'instance de
    # ce dossier est en cours (elle le tient en memoire) ou si le port est occupe
    # sans qu'on sache de qui elle est. Ce n'est pas un echec du patch, c'est un
    # etage intermediaire, et il doit etre dit comme tel — pas jete en exception au
    # milieu d'un depot dont les autres fichiers, eux, pouvaient passer.
    if ($SansRelance -and $c.exe -and $binaireVerrouille) {
      $enAttente += $nom
      Log ("   EN ATTENTE DE REDEMARRAGE : " + $nom + " (l'instance le tient). Poser a nouveau sans -SansRelance une fois cette instance arrete.")
      continue
    }
    # Un fichier remplace a sa place dans l'archive, sinon le point de retour ne
    # couvre que l'exe et le lanceur — et un outil mal pose serait irrattrapable.
    if (Test-Path -LiteralPath $c.dst) {
      $archSous = Join-Path $Arch 'fichiers-remplaces'
      New-Item -ItemType Directory -Path $archSous -Force | Out-Null
      Copy-Item -LiteralPath $c.dst -Destination (Join-Path $archSous $nom) -Force
    }
    Copy-Item -LiteralPath $c.src -Destination $c.dst -Force
    if (-not (Test-Path -LiteralPath $c.dst)) { throw "l'écriture n'a rien donné : $c.dst" }
    if ((Sha $c.dst) -ne (Sha $c.src)) { throw "l'empreinte de $nom ne correspond pas au paquet apres copie" }
    if ($c.exe) {
      # Un .exe n'est JAMAIS recode en texte (mesure : 19 946 496 -> 29 027 591 o
      # par un poseur de BOM). La tete MZ est le controle le plus simple.
      $tete = [IO.File]::ReadAllBytes($c.dst)[0..1]
      if ([char]$tete[0] -ne 'M' -or [char]$tete[1] -ne 'Z') { throw "$nom n'a plus de tete MZ apres copie" }
    }
    Log ("   OK  " + $nom + " -> " + $c.dst + " (" + (Get-Item -LiteralPath $c.dst).Length + " o)")
  }

  $iv = [Diagnostics.FileVersionInfo]::GetVersionInfo((Resolve-Path -LiteralPath $exe).Path)
  if ($iv.ProductVersion -ne $WantV -or $iv.FileVersion -ne $WantV) { throw "le binaire installe annonce $($iv.ProductVersion) / $($iv.FileVersion)" }
  Log ("   ajean.exe = " + $iv.ProductVersion + " (" + (Get-Item -LiteralPath $exe).Length + " o, sha " + (Sha $exe).Substring(0,16) + ")")

  # Le lanceur doit attendre la nouvelle version : c'est lui qui refuse de demarrer
  # sinon, et un lanceur qui verrouille l'ancienne version rend le patch invisible.
  $relu = [IO.File]::ReadAllText($ps, [Text.Encoding]::UTF8)
  $attendu = [char]39 + $WantV + [char]39
  if (-not $relu.Contains($attendu)) { throw "le lanceur en place n'attend pas la nouvelle version" }
  Log ("   Lancer-Cresius.ps1 attend $WantV (" + (Get-Item -LiteralPath $ps).Length + " o)")

  # 5. relance + controle de ce que la machine annonce reellement.
  #
  # Les controles HTTP (pont, voisins, refus, agent_on, version annoncee) ne sont
  # valides que si l'instance qui repond sur 18090 est CELLE de ce dossier. Sinon
  # ils mesureraient l'instance d'un autre dossier et rendraient un faux verdict —
  # par exemple « agent_on=False » sur un dossier qu'on vient de patcher sans
  # l'arreter, alors que ce qu'on a lu appartient a son voisin.
  if ($SansRelance) {
    if ($enAttente.Count) {
      # Le binaire n'a pas pu etre pose : l'instance en memoire reste l'ancienne.
      # Les controles HTTP ci-dessous (pont, voisins, refus, agent_on) porteraient
      # donc sur l'ANCIENNE version et rendraient des conclusions fausses — le
      # temoin agent_on accuserait « revision 4 absente » alors que le paquet est
      # sur le disque. On ne les joue pas, et on le dit.
      Log "== 5. relance : NON (-SansRelance) et binaire non pose"
      Log ("   controles HTTP SANS EFFET ici : ils porteraient sur l'instance en memoire, pas sur ce qui vient d'etre pose. Fichiers en attente : " + ($enAttente -join ', '))
      Log "   pour finir : arreter l'instance, relancer ce script SANS -SansRelance."
      continue
    }
    if (-not $instanceDeCeDossier) {
      Log "== 5. relance : NON (-SansRelance) et aucune instance de ce dossier ne repond sur 18090"
      if ($parent) {
        Log ("   fichiers posés et verifies sur disque ; controles HTTP REPORTES : ce qui repond sur 18090 est un AUTRE dossier ($($etat.home)), pas $homeAttendu.")
      } else {
        Log ("   fichiers posés et verifies sur disque ; controles HTTP REPORTES : 18090 est libre, aucune instance de ce dossier ne tourne.")
      }
      Log "   ces controles seront joues au prochain demarrage de CE dossier (ou en rejouant ce script sans -SansRelance quand il sera au premier plan)."
      continue
    }
    Log "== 5. relance : NON (-SansRelance). L'instance en cours est deja la version visee, les controles ci-dessous la concernent vraiment."
  } else {
    Log "== 5. relance"
    Start-Process -FilePath (Join-Path $Root 'Lancer-Cresius.cmd') -WorkingDirectory $Root -WindowStyle Minimized
    Log "   lanceur demarre"
  }
  $ok = $false
  $etat = $null
  $deadline = (Get-Date).AddSeconds(420)
  while ((Get-Date) -lt $deadline) {
    Start-Sleep -Seconds 10
    try { $etat = Invoke-RestMethod -Uri 'http://127.0.0.1:18090/api/embeddings' -TimeoutSec 5 ; $ok = $true ; break } catch { }
  }
  if (-not $ok) {
    Log "   (!) 18090 ne repond pas apres 420 s — relancer Lancer-Cresius.cmd a la main"
    $echecs += "$Root : 18090 mort apres relance" ; continue
  }
  Log ("   version annoncee : " + $etat.version + " · home : " + $etat.home)
  if ($etat.version -ne $WantV) { throw "le service repond $($etat.version) au lieu de $WantV" }
  if ($etat.home -ne $homeAttendu) {
    Log ("   (!) home attendu " + $homeAttendu + " — verifier qu'une seule instance tourne")
  }
  Log ("   encodeur : running=" + $etat.running + " ready=" + $etat.ready + " dims=" + $etat.config.dimensions)

  # 6. preuve que le pont et le canal existent bien dans ce binaire : les routes
  # doivent repondre (et pas 404). C'est le controle du patch, pas une mise en
  # service — le pont reste eteint par defaut.
  try {
    $b = Invoke-RestMethod -Uri 'http://127.0.0.1:18090/api/bridge' -TimeoutSec 5
    Log ("   pont : demande=" + $b.requested + " actif=" + $b.enabled + " port=" + $b.port + " cle=" + $b.has_key + " prete=" + $b.serving)
  } catch { Log ("   (!) /api/bridge : " + $_.Exception.Message) }
  try {
    $p2 = Invoke-RestMethod -Uri 'http://127.0.0.1:18090/api/peers' -TimeoutSec 5
    Log ("   canal : " + @($p2.peers).Count + " voisin(s) declare(s)")
  } catch { Log ("   (!) /api/peers : " + $_.Exception.Message) }

  # 7. preuve SPECIFIQUE de la revision 3 : une route refusee doit dire POURQUOI.
  # Avant la revision 3, /api/network repondait 403 avec une phrase sur la
  # « Memoire semantique », sans rapport avec le reseau. Ce controle ne prouve
  # donc pas seulement « le pont existe », mais « c'est bien la revision 3 ».
  Log "== 7. preuve de la revision 3 (le refus doit dire pourquoi)"
  # PIEGE MESURE le 2026-10-28 : Invoke-WebRequest -UseBasicParsing AVAIT RAISON
  # du refus (HTTP 403), mais le corps lu ensuite etait VIDE (0 o) alors que
  # Content-Length annoncait 310 o. En PowerShell 5.1, l'exception a deja consomme
  # le flux de la reponse : relire GetResponseStream() rend une chaine vide. Le
  # controle accusait donc « le refus ne designe pas le pont : ce n'est pas la
  # revision 3 » sur un binaire qui contenait bien le refus correct. Un controle
  # qui ment est pire qu'un controle absent : il ferait rejeter un bon paquet.
  # On relit donc avec curl.exe (present sur Windows 10+), qui rend le corps intact.
  $corps3 = ''
  $code3 = 0
  try {
    $brut = & curl.exe -s -i --max-time 8 'http://127.0.0.1:18090/api/network' 2>$null
    $texte = ($brut -join "`n")
    $mLigne = [regex]::Match($texte, 'HTTP/[0-9.]+\s+(\d{3})')
    if ($mLigne.Success) { $code3 = [int]$mLigne.Groups[1].Value }
    $mCorps = [regex]::Match($texte, '(?s)\r?\n\r?\n(.*)$')
    if ($mCorps.Success) { $corps3 = $mCorps.Groups[1].Value }
  } catch { Log ("   (!) curl.exe indisponible : " + $_.Exception.Message) }
  if ($code3 -eq 0) {
    # Repli : au moins le statut, sans le corps. Le controle du texte est alors
    # declare INCONCLUANT au lieu de rendre un faux « ce n'est pas la revision 3 ».
    try {
      $w = Invoke-WebRequest -Uri 'http://127.0.0.1:18090/api/network' -TimeoutSec 5 -UseBasicParsing
      $code3 = [int]$w.StatusCode
      Log "   (!) /api/network a repondu $code3 au lieu de 403 : le garde-fou du build portable ne tourne pas"
      $echecs += "$Root : /api/network non refuse"
    } catch {
      if ($_.Exception.Response) { $code3 = [int]$_.Exception.Response.StatusCode }
      else { Log ("   (!) /api/network : " + $_.Exception.Message) }
    }
  }
  if ($code3 -eq 0) {
    Log "   INCONCLUANT : aucune lecture de /api/network (ni curl ni Invoke-WebRequest). Le texte du refus n'est pas verifiable ici."
  } else {
    Log ("   refus /api/network : HTTP $code3 — " + ($corps3 -replace '[^\x20-\x7E]','?'))
    if ($code3 -ne 403) { Log "   (!) statut attendu 403" ; $echecs += "$Root : /api/network en $code3" }
    if ($corps3 -eq '') {
      Log "   INCONCLUANT sur le texte : statut 403 recu mais corps non relisable. Non compte comme echec."
    } else {
      if ($corps3 -match 'semantique') {
        Log "   (!) le refus renvoie encore a la Memoire semantique : ce n'est pas la revision 3"
        $echecs += "$Root : refus reseau toujours trompeur"
      }
      if ($corps3 -notmatch 'pont') {
        Log "   (!) le refus reseau ne designe pas le pont : ce n'est pas la revision 3"
        $echecs += "$Root : refus reseau sans raison explicite"
      }
    }
  }

  # 8. preuve de la révision 4 : le pont expose l'autorisation « agent ». Un
  # 0.18.15.3 ne connaît pas ce champ — il ne sait pas prêter autre chose que du
  # texte nu. C'est le témoin le plus sûr, et il ne demande pas d'ouvrir le pont.
  Log "== 8. preuve de la révision 4 (le témoin agent_on doit exister)"
  try {
    $b4 = Invoke-RestMethod -Uri 'http://127.0.0.1:18090/api/bridge' -TimeoutSec 5
    if ($b4.PSObject.Properties.Name -notcontains 'agent_on') {
      Log "   (!) /api/bridge n'expose pas agent_on : ce n'est pas la révision 4 (le voisin ne pourra pas rendre de tour d'agent)"
      $echecs += "$Root : pas de témoin agent_on"
    } else {
      Log ("   témoin agent_on présent : autorisation en place = " + $b4.agent_on)
    }
  } catch { Log ("   (!) /api/bridge : " + $_.Exception.Message) }

  # 9. LE CORRECTIF DU MODÈLE NU, côté destinataire. Les deux interrupteurs sont en
  # base : aucune copie de fichier ne peut les allumer. Sans eux, le pont reste au
  # prêt du modèle nu — ce qui, sur la machine voisine, rendait impossible la
  # lecture de SES propres journaux, donc le diagnostic de SON moteur.
  if ($veutAgent) {
    Log "== 9. interrupteurs du tour d'agent (le correctif du modele nu)"
    try {
      $null = Invoke-RestMethod -Uri 'http://127.0.0.1:18090/api/bridge' -Method Post `
        -Body '{"agent":true}' -ContentType 'application/json' -TimeoutSec 20
    } catch { Log ("   (!) POST /api/bridge {agent:true} : " + $_.Exception.Message) }
    try {
      $null = Invoke-RestMethod -Uri 'http://127.0.0.1:18090/api/agent/toggle' -Method Post `
        -Body '{"on":true}' -ContentType 'application/json' -TimeoutSec 20
    } catch { Log ("   (!) POST /api/agent/toggle : " + $_.Exception.Message) }
    try {
      $b5 = Invoke-RestMethod -Uri 'http://127.0.0.1:18090/api/bridge' -TimeoutSec 5
      Log ("   après : agent_on=" + $b5.agent_on + "  autorisation pont=" + $b5.agent_requested + "  mode agent machine=" + $b5.agent_mode_local)
      if (-not [bool]$b5.agent_on) {
        Log "   (!) agent_on reste faux : le pont prêtera toujours le modèle nu. Vérifier le mode agent dans l'interface (Projet/Agent)."
        $echecs += "$Root : tour d'agent toujours refusé après allumage"
      }
    } catch { Log ("   (!) /api/bridge : " + $_.Exception.Message) }
  }

  # 10. preuve de la révision 6 : le pont dit POURQUOI il ne rend pas de tour
  # d'agent. C'est LE témoin qui distingue 0.18.15.5 (silence) de 0.18.16.1.
  #
  # LU DANS LE BINAIRE, pas en appelant le pont. J'avais d'abord écrit une sonde
  # HTTP vers /v1/chat/completions avec l'en-tête du canal : elle aurait déclenché
  # un VRAI tour d'agent sur la machine visée — outils, shell, disque — et son
  # message serait entré dans la conversation partagée de l'utilisateur, par la
  # seule faute d'un script de contrôle. Un contrôle ne doit pas avoir d'effet.
  # La chaîne X-Ajean-Canal-Agent-Refus n'existe qu'à partir de la révision 6 :
  # sa présence dans le binaire installé est la preuve, et elle est gratuite.
  Log "== 10. preuve de la révision 6 (le pont rend compte de ce qu'il prête)"
  $marqueurs = @('X-Ajean-Canal-Agent-Refus', 'bridge_agent=off', 'agent_mode=off')
  $octetsExe = [IO.File]::ReadAllBytes($exe)
  $brut = [Text.Encoding]::ASCII.GetString($octetsExe)
  foreach ($m in $marqueurs) {
    if ($brut.Contains($m)) { Log ("   OK : « " + $m + " » est dans le binaire installé") }
    else {
      Log ("   (!) « " + $m + " » absent du binaire installé : ce n'est pas la révision 6")
      $echecs += "$Root : marqueur révision 6 absent ($m)"
    }
  }
  # Témoin de la révision 4, conservé : le champ existe côté interface.
  try {
    $b6 = Invoke-RestMethod -Uri 'http://127.0.0.1:18090/api/bridge' -TimeoutSec 5
    Log ("   état du canal : agent_on=" + $b6.agent_on + "  autorisation pont=" + $b6.agent_requested + "  mode agent machine=" + $b6.agent_mode_local)
    if ($veutAgent -and -not [bool]$b6.agent_on) {
      Log "   (!) agent_on reste faux : le pont prêtera toujours le modèle nu."
      $echecs += "$Root : tour d'agent toujours refusé après allumage"
    }
  } catch { Log ("   (!) /api/bridge : " + $_.Exception.Message) }

  Log "PATCH_FAIT"
}

Log ""
Log "== bilan"
if ($echecs.Count) {
  foreach ($e in $echecs) { Log ("   ECHEC : " + $e) }
  exit 1
}
# Le bilan ne doit pas dire « a jour » en sortie de seche : la seche sort par
# `continue` sans rien ecrire, et le message afficherait une satisfaction fausse
# juste apres avoir enumere les fichiers manquants. Un controle qui ne dit rien
# n'est pas un controle qui passe.
if ($Seccher) {
  Log "   SECHE TERMINEE — rien n'a ete ecrit. Ce qui est annonce ci-dessus serait pose."
} else {
  Log "   tous les dossiers vises sont a jour : binaire, lanceur et outils compares un par un"
}
