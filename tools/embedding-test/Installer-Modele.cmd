@echo off
setlocal
powershell.exe -NoLogo -NoProfile -ExecutionPolicy Bypass -File "%~dp0Installer-Modele.ps1"
if errorlevel 1 pause
endlocal
