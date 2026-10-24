@echo off
cd /d "%~dp0"
if not exist config.json copy config.example.json config.json >nul
windows-pxe-dhcp.exe --config config.json --check
pause
