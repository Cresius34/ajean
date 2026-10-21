@echo off
setlocal
powershell.exe -NoLogo -NoProfile -ExecutionPolicy Bypass -File "%~dp0Copier-Memoire.ps1" %*
pause
endlocal
