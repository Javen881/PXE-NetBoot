@echo off
cd /d "%~dp0"
net session >nul 2>&1
if errorlevel 1 (
  echo Please run start.cmd as Administrator.
  pause
  exit /b 1
)
if not exist config.json copy config.example.json config.json >nul
windows-pxe-dhcp.exe --config config.json --check
if errorlevel 1 (
  pause
  exit /b 1
)
windows-pxe-dhcp.exe --config config.json
pause
