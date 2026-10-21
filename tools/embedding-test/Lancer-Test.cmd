@echo off
setlocal
powershell.exe -NoLogo -NoProfile -ExecutionPolicy Bypass -File "%~dp0Lancer-Test.ps1"
if errorlevel 1 pause
endlocal
