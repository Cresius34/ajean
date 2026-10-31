# Test du SELECTEUR DE LANCEUR du poseur de canal.
#
# Pourquoi ce fichier : depuis la suppression de Lancer-Cresius-principale.ps1,
# le poseur ne peut plus departager par un nom ecrit en dur. Il doit choisir le
# lanceur que le paquet CONTIENT reellement. Ce test le prouve sur des paquets
# fabriques, sans rien installer et sans demarrer l'application.
#
# Ce que la regression doit attraper si le nom du lanceur est re-introduit :
#   - paquet a UN lanceur, cible sans suffixe  -> ce lanceur, pas un nom invente ;
#   - paquet a DEUX lanceurs, cible a suffixe  -> celui dont l'etiquette matche ;
#   - paquet vide de lanceur                  -> sentinelle, jamais un nom en dur.
#
# La fonction n'est PAS recopiee ici : elle est extraite du poseur par son AST.
# Un test qui recopie la logique testee ne prouve rien.
$ErrorActionPreference = 'Stop'

# Ce test vit DANS tools\patch-canal, a cote de l'outil qu'il teste : il le teste
# la ou il est, depuis n'importe quel dossier courant (d'ou $PSScriptRoot, pas
# $MyInvocation, qui depend de la facon dont on l'appelle).
$here = $PSScriptRoot
$poseur = Join-Path $here 'Appliquer-Patch-Canal.ps1'
if (-not (Test-Path -LiteralPath $poseur)) { throw "poseur introuvable : $poseur" }

$null = $null
$ast = [System.Management.Automation.Language.Parser]::ParseFile(
    $poseur, [ref]$null, [ref]$null)
$fonc = @($ast.FindAll({
    param($n) $n -is [System.Management.Automation.Language.FunctionDefinitionAst] -and
              $n.Name -eq 'Inventaire' }, $true)) | Select-Object -First 1
if (-not $fonc) { throw 'fonction Inventaire introuvable dans le poseur' }
Invoke-Expression $fonc.Extent.Text

$script:echecs = 0

function NouveauPaquet($dossier, [string[]]$lanceurs) {
    $lanc = Join-Path $dossier 'lanceurs'
    New-Item -ItemType Directory -Path $lanc -Force | Out-Null
    Set-Content -LiteralPath (Join-Path $dossier 'ajean.exe') -Value 'MZ-factice' -NoNewline
    foreach ($n in $lanceurs) {
        Set-Content -LiteralPath (Join-Path $lanc $n) -Value '$expectedVersion = ''0.18.16.4 Cresius''' -Encoding UTF8
    }
    return $lanc
}

function Verifie($titre, $attenduLanceur, $attenduCible, $racinePaquet, $lanceurs, $nomCible) {
    $script:LancD = NouveauPaquet $racinePaquet $lanceurs
    $script:NewExe = Join-Path $racinePaquet 'ajean.exe'
    $script:OutilsInstalles = @()

    $cible = Join-Path (Split-Path -Parent $racinePaquet) $nomCible
    New-Item -ItemType Directory -Path $cible -Force | Out-Null

    $item = Inventaire $cible
    $entree = @($item | Where-Object { $_.dst -like '*Lancer*' })
    $luSrc = if (@($entree).Count) { Split-Path -Leaf ([string]$entree[0].src) } else { '<aucune entree>' }
    $luDst = if (@($entree).Count) { Split-Path -Leaf ([string]$entree[0].dst) } else { '<aucune entree>' }

    $ok = ($luSrc -eq $attenduLanceur) -and ($luDst -eq $attenduCible)
    if (-not $ok) { $script:echecs++ }
    $etat = if ($ok) { 'PASS' } else { 'ECHOUE' }
    Write-Output ("  [{0}] {1}" -f $etat, $titre)
    Write-Output ("         src obtenu : {0}  (attendu {1})" -f $luSrc, $attenduLanceur)
    if ($luDst -ne $attenduCible) {
        Write-Output ("         dst obtenu : {0}  (attendu {1})" -f $luDst, $attenduCible)
    }
}

$boue = Join-Path $env:TEMP ('ajean-test-selecteur-' + (Get-Date -Format 'HHmmss'))
New-Item -ItemType Directory -Path $boue -Force | Out-Null

try {
    Write-Output 'TEST — le poseur choisit le lanceur que le paquet contient'

    # 1. LE CAS REEL D'AUJOURD'HUI : un seul lanceur dans le paquet, cible sans
    #    suffixe. Avant, le defaut « Lancer-Cresius-principale.ps1 » gagnait et
    #    la pose echouait sur « ABSENT DU PAQUET ».
    Verifie -titre 'paquet a un lanceur, cible sans suffixe -> ce lanceur' `
            -attenduLanceur 'Lancer-Cresius-RX6900XT.ps1' -attenduCible 'Lancer-Cresius.ps1' `
            -racinePaquet (Join-Path $boue 'p1') -lanceurs @('Lancer-Cresius-RX6900XT.ps1') `
            -nomCible 'Ajean maj Cresius'

    # 2. DEUX lanceurs, etiquette qui matche le suffixe du dossier vise.
    Verifie -titre 'paquet a deux lanceurs, suffixe « RX6900XT » -> la variante' `
            -attenduLanceur 'Lancer-Cresius-RX6900XT.ps1' -attenduCible 'Lancer-Cresius.ps1' `
            -racinePaquet (Join-Path $boue 'p2') `
            -lanceurs @('Lancer-Cresius-principale.ps1', 'Lancer-Cresius-RX6900XT.ps1') `
            -nomCible 'Ajean maj Cresius RX6900XT'

    # 3. DEUX lanceurs dont une « principale », cible sans suffixe -> la principale.
    Verifie -titre 'paquet a deux lanceurs, pas de suffixe -> la principale' `
            -attenduLanceur 'Lancer-Cresius-principale.ps1' -attenduCible 'Lancer-Cresius.ps1' `
            -racinePaquet (Join-Path $boue 'p3') `
            -lanceurs @('Lancer-Cresius-principale.ps1', 'Lancer-Cresius-RX6900XT.ps1') `
            -nomCible 'Ajean maj Cresius'

    # 4. PAQUET SANS LANCEUR : sentinelle, et src inexistant -> le poseur dira
    #    « ABSENT DU PAQUET ». Jamais un nom en dur qui ferait ecrire au hasard.
    $script:LancD = NouveauPaquet (Join-Path $boue 'p4') @()
    $script:NewExe = Join-Path $boue 'p4\ajean.exe'
    $script:OutilsInstalles = @()
    $c4 = Join-Path $boue 'Ajean maj Cresius'
    New-Item -ItemType Directory -Path $c4 -Force | Out-Null
    $e4 = @((Inventaire $c4) | Where-Object { $_.dst -like '*Lancer*' })
    $lu4 = if (@($e4).Count) { Split-Path -Leaf ([string]$e4[0].src) } else { '<aucune entree>' }
    $vit4 = @($e4).Count -eq 1 -and -not (Test-Path -LiteralPath ([string]$e4[0].src))
    if ($lu4 -eq 'aucun-lanceur-dans-ce-paquet.ps1' -and $vit4) {
        Write-Output '  [PASS] paquet sans lanceur -> sentinelle, src inexistant (le poseur refusera)'
    } else {
        $script:echecs++
        Write-Output ("  [ECHOUE] paquet sans lanceur -> obtenu « {0} », src existe = {1}" -f `
            $lu4, (Test-Path -LiteralPath ([string]$e4[0].src)))
    }

    # 5. LE NOM SUPPRIME NE DOIT PLUS ETRE UN DEFAUT DU POSEUR.
    $corps = [IO.File]::ReadAllText($poseur, [Text.Encoding]::UTF8)
    if ($corps -match "\`$nomLanceur\s*=\s*'Lancer-Cresius-principale\.ps1'") {
        $script:echecs++
        Write-Output '  [ECHOUE] « Lancer-Cresius-principale.ps1 » sert encore de defaut dans le poseur'
    } else {
        Write-Output '  [PASS] plus aucun defaut « Lancer-Cresius-principale.ps1 » dans le poseur'
    }

    # 6. LE FICHIER SUPPRIME NE DOIT PAS REAPPARAITRE.
    if (Test-Path -LiteralPath (Join-Path $here 'lanceurs\Lancer-Cresius-principale.ps1')) {
        $script:echecs++
        Write-Output '  [ECHOUE] le lanceur de la principale est revenu dans le depot'
    } else {
        Write-Output '  [PASS] absent du depot'
    }

    # 7. LE GENERATEUR DE VERROU tourne sans elle : il balaie les Lancer-*.ps1,
    #    un seul fichier doit suffire. JOUE SUR UNE COPIE : genere-lanceurs ecrit
    #    dans son propre dossier, et la source gelee du depot doit garder sa
    #    version a elle (le fabriqueur de paquet applique le verrou apres copie).
    $lancCopie = Join-Path $boue 'p7\lanceurs'
    New-Item -ItemType Directory -Path $lancCopie -Force | Out-Null
    Copy-Item -LiteralPath (Join-Path $here 'lanceurs\genere-lanceurs.ps1') `
              -Destination $lancCopie -Force
    Copy-Item -LiteralPath (Join-Path $here 'lanceurs\Lancer-Cresius-RX6900XT.ps1') `
              -Destination $lancCopie -Force
    & powershell -NoProfile -ExecutionPolicy Bypass -File (Join-Path $lancCopie 'genere-lanceurs.ps1') `
        -VersionLue '0.18.16.4 Cresius' | ForEach-Object { Write-Output ('         ' + $_) }
    if ($LASTEXITCODE) {
        $script:echecs++
        Write-Output '  [ECHOUE] genere-lanceurs a echoue sans le lanceur de la principale'
    } else {
        $verrou = [regex]::Match(
            [IO.File]::ReadAllText((Join-Path $lancCopie 'Lancer-Cresius-RX6900XT.ps1'), [Text.Encoding]::UTF8),
            "\`$expectedVersion\s*=\s*'([^']*)'").Groups[1].Value
        if ($verrou -eq '0.18.16.4 Cresius') {
            Write-Output '  [PASS] genere-lanceurs verrouille le lanceur restant (un seul suffit)'
        } else {
            $script:echecs++
            Write-Output ("  [ECHOUE] genere-lanceurs a pose le verrou « {0} »" -f $verrou)
        }
    }

    Write-Output ''
    if ($script:echecs) {
        Write-Output ("RESULTAT : {0} echec(s)" -f $script:echecs) ; exit 1
    }
    Write-Output 'RESULTAT : TOUT PASSE'
    exit 0
} finally {
    Remove-Item -LiteralPath $boue -Recurse -Force -EA SilentlyContinue
}
