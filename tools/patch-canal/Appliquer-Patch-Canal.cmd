@echo off
rem Applique le patch canal a L'INSTALLATION QUI CONTIENT CE DOSSIER.
rem
rem Aucun chemin, aucune version ecrits ici : la version visee est lue dans le
rem binaire du paquet, la cible est le dossier parent (controle structurel : un
rem ajean.exe a sa racine). C'est voulu - une version cablee dans ce fichier
rem refusait nos propres paquets des que le milesime changeait, et un nom de
rem dossier cable ment des que le dossier est renomme (nos versions sont
rem portables).
rem
rem Ce que ca fait : archive l'exe et le lanceur dans archive-<version-en-place>,
rem arrete PROPREMENT l'instance de CE dossier (Ctrl+C, attente avant toute
rem terminaison forcee), remplace ajean.exe et le lanceur que Lancer-Cresius.cmd
rem appelle reellement, pose les outils dans data\scripts, relance, puis verifie
rem que le pont repond et que le refus de /api/network dit bien POURQUOI.
rem
rem ATTENTION : l'instance de ce dossier s'arrete. Fermer ce qui y travaille.
rem Une seule instance par machine (port 18090 partage) : si 18090 est tenu par
rem une AUTRE installation, le script le dit et refuse au lieu de la tuer.
rem
rem Deux temps, volontairement :
rem   1) une SECHE, sans aucune ecriture ;
rem   2) taper O puis Entree pour appliquer vraiment.

setlocal
set SCRIPT=%~dp0Appliquer-Patch-Canal.ps1

if not exist "%SCRIPT%" (
  echo Le script est absent a cote de ce fichier : %SCRIPT%
  echo Le paquet a du etre dezippe a moitie. Re-dezipper patch-canal-18161.zip a la
  echo racine du dossier d'installation.
  pause
  exit /b 1
)

echo == Etape 1 : seche (aucune ecriture) ==
powershell -NoProfile -ExecutionPolicy Bypass -File "%SCRIPT%" -Seccher
if errorlevel 1 (
  echo.
  echo La seche signale un probleme (voir ci-dessus). Rien ne sera ecrit.
  echo Corrige la cause, puis relance ce fichier.
  pause
  exit /b 1
)

echo.
echo == Etape 2 : appliquer ? (O/N) ==
echo      L'instance de ce dossier sera arretee puis relancee.
choice /C ON /N /M "Appliquer le patch : "
if errorlevel 2 (
  echo Annule. Rien n'a ete ecrit.
  pause
  exit /b 0
)

powershell -NoProfile -ExecutionPolicy Bypass -File "%SCRIPT%"
if errorlevel 1 (
  echo.
  echo ECHEC - voir le journal Patch-canal dans data\scripts. Le point de retour est
  echo dans le dossier archive-... cree au debut de la pose.
  pause
  exit /b 1
)

echo.
echo Patch applique.
echo.
echo Reste a faire pour le canal, dans ce meme dossier :
echo    Cabler-Canal-Local.ps1 -Voisin ^<IP de l'autre machine^> -Nom ^<nom^> -Test
echo    (lit TA propre cle API depuis ton interface locale : rien a taper)
echo.
echo Et pour l'etat complet de ce poste : Diagnostic-Canal.cmd (lecture seule)
pause
endlocal
