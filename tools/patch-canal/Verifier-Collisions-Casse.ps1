# Verifier-Collisions-Casse.ps1 — detecte, dans un .ps1, une variable locale dont
# le nom ne differe que par la CASSE d'un parametre du param() (ou d'une autre
# variable locale).
#
# Piege mesuré le 2026-10-27 : [switch]$Cle + $cle = '' plus bas → le switch
# devient une chaine vide et l'appel du script echoue a la liaison des arguments,
# meme sans passer le switch. PSParser ne voit rien, Get-Command non plus.
# PowerShell ignore la casse des noms : $Voisins et $voisins sont la MEME variable.
#
# Deux faux positifs mesurés le 2026-10-27, corrigés ici :
#   1. le bloc param() etait borne a « la premiere ligne qui finit par ) seul » :
#      sur un param() tenu sur une seule ligne, il continuait de collecter des $
#      dans tout l'en-tete du fichier et inventait des parametres ;
#   2. les $ ecrits dans les chaines (un regex comme '\$Want\w*\s*=…') etaient
#      pris pour des affectations. Un controle qui crie partout ne sert a rien :
#      il avait deja fait compter « une collision » la ou il n'y en avait aucune.
#   3. mesuré le 2026-10-27 a 20h : ce fichier ne voyait PAS son propre parametre
#      $Chemin. Cause : il cherchait « param( » dans le texte BRUT et tombait sur
#      le mot param() cite dans son commentaire d'en-tete (ligne 3). Le bloc
#      trouve etait donc « param() » vide, et les commentaires contaient aussi
#      comme affectations. Il fallait retirer les commentaires aussi.
param([Parameter(Mandatory = $true)][string]$Chemin)
$ErrorActionPreference = 'Stop'
$t = [IO.File]::ReadAllText($Chemin, [Text.Encoding]::UTF8)

# Les noms cites dans une chaine ne sont pas des variables du script. On retire
# les literals (simple et double quotes, y compris here-strings) avant de chercher.
function Retirer-Les-Chaines([string]$s) {
  $out = [regex]::Replace($s, '@".*?"@', ' ', 'Singleline')
  $out = [regex]::Replace($out, "@'.*?'@", ' ', 'Singleline')
  $out = [regex]::Replace($out, '"(?:\\.|[^"\\])*"', '""')
  $out = [regex]::Replace($out, "'(?:[^']|'')*'", "''")
  return $out
}
# Un commentaire n'est pas du code. Retire APRES les chaines : le caractere #
# n'a plus de concurrent a l'interieur d'un literal.
function Retirer-Les-Commentaires([string]$s) {
  $L = $s -split "`r?`n"
  for ($i = 0; $i -lt $L.Count; $i++) {
    $k = $L[$i].IndexOf('#')
    if ($k -ge 0) { $L[$i] = $L[$i].Substring(0, $k) }
  }
  return ($L -join "`n")
}
$tNu = Retirer-Les-Commentaires (Retirer-Les-Chaines $t)

# ---- le bloc param(), borne a ses parentheses et pas a une ligne ----
$parametres = @()
$idp = $tNu.IndexOf('param(')
if ($idp -ge 0) {
  $profondeur = 0 ; $fin = -1
  for ($i = $idp + 5; $i -lt $tNu.Length; $i++) {
    if ($tNu[$i] -eq '(') { $profondeur++ }
    elseif ($tNu[$i] -eq ')') { $profondeur-- ; if ($profondeur -eq 0) { $fin = $i ; break } }
  }
  if ($fin -lt 0) { $fin = $tNu.Length - 1 }
  $bloc = $tNu.Substring($idp, $fin - $idp + 1)
  foreach ($m in [regex]::Matches($bloc, '\$(\w+)')) { $parametres += $m.Groups[1].Value }
  $parametres = @($parametres | Select-Object -Unique)
}

# ---- toute affectation $x = …, $x += …, foreach ($x in …) ----
$affect = @()
$L = $tNu -split "`r?`n"
for ($i = 0; $i -lt $L.Count; $i++) {
  foreach ($m in [regex]::Matches($L[$i], '^\s*\$(\w+)\s*(=|\+=|-=|\+\+|--)(?!=)')) { $affect += , @($m.Groups[1].Value, ($i + 1)) }
  foreach ($m in [regex]::Matches($L[$i], 'foreach\s*\(\s*\$(\w+)\s+in')) { $affect += , @($m.Groups[1].Value, ($i + 1)) }
  foreach ($m in [regex]::Matches($L[$i], '\$(\w+)\s*(:=|\+=)')) { $affect += , @($m.Groups[1].Value, ($i + 1)) }
}

Write-Output ('fichier : ' + (Split-Path -Leaf $Chemin))
Write-Output ('parametres : ' + $(if ($parametres.Count) { $parametres -join ', ' } else { 'aucun' }))
$mauvais = 0
foreach ($p in $parametres) {
  foreach ($a in $affect) {
    if ($a[0] -ieq $p -and $a[0] -cne $p) {
      Write-Output ('   COLLISION : parametre $' + $p + ' ecrase par $' + $a[0] + ' a la ligne ' + $a[1])
      $mauvais++
    }
  }
}
for ($i = 0; $i -lt $affect.Count; $i++) {
  for ($j = $i + 1; $j -lt $affect.Count; $j++) {
    if ($affect[$i][0] -ieq $affect[$j][0] -and $affect[$i][0] -cne $affect[$j][0]) {
      Write-Output ('   COLLISION locale : $' + $affect[$i][0] + ' (ligne ' + $affect[$i][1] + ') vs $' + $affect[$j][0] + ' (ligne ' + $affect[$j][1] + ')')
      $mauvais++
    }
  }
}
if ($mauvais -eq 0) { Write-Output '   aucune collision de casse' }
