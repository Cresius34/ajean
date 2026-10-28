@echo off
rem Ouvre le pare-feu Windows pour le canal Ajean-Ajean (port du pont, 18190 par defaut).
rem
rem Pourquoi ce fichier : le binaire essaie lui-meme d'ouvrir le port quand le pont
rem s'allume, mais il echoue sans droits administrateur (le jeton de cette session
rem est filtre : BUILTIN\Administrateurs est "groupe utilise pour les refus uniquement").
rem Ce .cmd se relance lui-meme en tant qu'administrateur (UAC), une seule fois.
rem
rem IMPORTANT ? le nom de la regle : le binaire SUPPRIME sa regle quand le pont se
rem coupe (stopParallelBridge -> firewallClose efface ? AJEAN moteur (port 18190) ?).
rem Une regle portee ce nom disparaitrait a la premiere coupure. Ici elle s'appelle
rem ? AJEAN canal (port 18190) ? : elle survit au pont.
rem
rem Port different ? Editer la ligne set PORT= plus bas, ou passer le port en argument :
rem   Ouvrir-PareFeu-Canal.cmd 18195

setlocal
set PORT=%~1
if "%PORT%"=="" set PORT=18190

rem Test d'elevation canonique, sans dependance de service. ? net session ? est
rem souvent employe mais il echoue quand le service Serveur est arrete : on
rem repartirait en elevation en boucle. Ici exit 0 = deja administrateur.
powershell -NoProfile -ExecutionPolicy Bypass -Command "$p=[Security.Principal.WindowsPrincipal][Security.Principal.WindowsIdentity]::GetCurrent(); exit ([int](-not $p.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)))"
if %errorlevel%==0 goto :eleve
echo Relance en tant qu'administrateur (une fenetre UAC va s'ouvrir)...
powershell -NoProfile -ExecutionPolicy Bypass -Command "try { Start-Process -FilePath '%~f0' -ArgumentList '%PORT%' -Verb RunAs; exit 0 } catch { exit 2 }"
if errorlevel 2 (
  echo Echec de l'elevation (annulee ou refusee). Colle la commande affichee plus bas
  echo dans un terminal ADMINISTRATEUR :
  echo   netsh advfirewall firewall add rule name="AJEAN canal (port %PORT%)" dir=in action=allow protocol=TCP localport=%PORT% profile=private,domain remoteip=localsubnet
  pause
  exit /b 1
)
exit /b 0

:eleve
set NOM=AJEAN canal (port %PORT%)
echo Regle : %NOM%  (profil prive et domaine, depuis le sous-reseau local seulement)

rem Idempotence : pas d'empilement de regles homonymes a chaque double-clic.
netsh advfirewall firewall delete rule name="%NOM%" >nul 2>&1

rem Le mot-cle est remoteip (et sa valeur localsubnet) ? pas remoteaddr, pas
rem remoteaddress : les deux sont refuses. Mesure faite sur ce poste.
netsh advfirewall firewall add rule name="%NOM%" dir=in action=allow protocol=TCP localport=%PORT% profile=private,domain remoteip=localsubnet

rem Ne PAS se fier au code de sortie de netsh : il rend 0 meme quand il refuse
rem ("L'operation demandee requiert une elevation") et meme sur un argument
rem invalide. La preuve, c'est la relecture de la regle par Get-NetFirewallRule,
rem dont l'erreur n'est pas localisee.
powershell -NoProfile -ExecutionPolicy Bypass -Command "try { $null = Get-NetFirewallRule -DisplayName '%NOM%' -ErrorAction Stop; Write-Output 'REGLE_EN_PLACE'; exit 0 } catch { Write-Output 'REGLE_ABSENTE'; exit 1 }"
if errorlevel 1 (
  echo.
  echo ECHEC ? la regle n'est pas en place. La commande exacte a coller dans un terminal ADMINISTRATEUR :
  echo   netsh advfirewall firewall add rule name="%NOM%" dir=in action=allow protocol=TCP localport=%PORT% profile=private,domain remoteip=localsubnet
  pause
  exit /b 1
)

echo.
echo Controle de ce qui est reellement en place :
netsh advfirewall firewall show rule name="%NOM%"
echo.
echo A refaire sur l'AUTRE machine du canal, avec son propre port de pont.
pause
endlocal
