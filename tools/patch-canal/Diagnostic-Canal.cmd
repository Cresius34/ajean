@echo off
rem Diagnostic-Canal.cmd - LANCEUR. Toute la logique est dans Diagnostic-Installation.ps1.
rem
rem Un seul appel, aucune logique ici : un .cmd qui porte la sonde en une ligne
rem (c'etait le cas de la revision 5) casse au premier guillemet et l'erreur
rem remonte dix lignes plus loin, sans dire quelle sonde est morte.
rem
rem LECTURE SEULE : aucune ecriture, aucun arret, aucun telechargement, pas de
rem droits administrateur.

setlocal
set SCRIPT=%~dp0Diagnostic-Installation.ps1

if not exist "%SCRIPT%" (
  echo Diagnostic-Installation.ps1 absent a cote de ce fichier : %SCRIPT%
  pause
  exit /b 1
)

powershell -NoProfile -ExecutionPolicy Bypass -File "%SCRIPT%"
echo.
pause
endlocal
