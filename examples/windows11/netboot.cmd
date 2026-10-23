@echo off
setlocal EnableExtensions
echo Initializing Windows installation network...
wpeinit
set "attempt=0"
set "max_attempts=12"
:connect
set /a attempt+=1 >nul
echo Connecting to Windows 11 installation source [%attempt%/%max_attempts%]...
net use Z: \\192.168.10.220\netboot-win11 /user:netboot-guest ""
if not errorlevel 1 goto check_source
rem A previous successful mapping may already exist when this script is retried.
net use Z: | findstr /L /C:"\\192.168.10.220\netboot-win11" >nul
if not errorlevel 1 goto check_source
goto retry
:check_source
if exist Z:\sources\install.wim goto install
echo Installation source is not readable yet.
:retry
if %attempt% GEQ %max_attempts% goto network_failed
echo Waiting 5 seconds before retrying...
ping -n 6 127.0.0.1 >nul
goto connect
:install
echo Starting Windows 11 Setup...
if not exist X:\sources\setup.exe goto setup_missing
X:\sources\setup.exe /installfrom:Z:\sources\install.wim
set "setup_rc=%errorlevel%"
echo Windows Setup returned exit code %setup_rc%.
echo Setup logs: X:\Windows\Panther and X:\$WINDOWS.~BT\Sources\Panther
goto shell
:setup_missing
echo Native Windows Setup is missing. Check boot.wim index.
goto shell
:network_failed
echo Unable to connect after %max_attempts% attempts.
echo Check the VPN route, SMB access and the errors above.
ipconfig
net use
:shell
echo To retry: X:\Windows\System32\netboot.cmd
cmd.exe
endlocal

