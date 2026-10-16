@echo off
powershell.exe -NoProfile -ExecutionPolicy Bypass -File "%~dp0Installer-Multimodal.ps1" %*
if errorlevel 1 pause
