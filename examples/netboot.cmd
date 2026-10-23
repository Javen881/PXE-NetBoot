@echo off
echo Initializing Windows installation network...
wpeinit
:connect
if exist Z:\sources\install.wim goto install
net use Z: \\192.168.10.220\netboot-win /user:netboot-guest ""
if errorlevel 1 goto network_failed
:install
if not exist Z:\sources\install.wim goto source_failed
echo Starting native Windows Setup from RAM...
if not exist X:\sources\setup.exe goto fallback
X:\sources\setup.exe /installfrom:Z:\sources\install.wim
set setup_rc=%errorlevel%
goto result
:fallback
Z:\setup.exe
set setup_rc=%errorlevel%
:result
echo Windows Setup returned exit code %setup_rc%.
echo Setup logs: X:\Windows\Panther and X:\$WINDOWS.~BT\Sources\Panther
goto shell
:network_failed
echo NETWORK SHARE CONNECTION FAILED. Check the error above and NIC drivers.
ipconfig
goto shell
:source_failed
echo INSTALL.WIM NOT FOUND ON Z:. Check share mapping and read permissions.
net use
:shell
echo To retry: X:\Windows\System32\netboot.cmd
cmd.exe
