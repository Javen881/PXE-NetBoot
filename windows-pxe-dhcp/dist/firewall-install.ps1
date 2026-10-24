$ErrorActionPreference = 'Stop'
$exe = Join-Path $PSScriptRoot 'windows-pxe-dhcp.exe'
if (-not (Test-Path $exe)) { throw "Missing $exe" }
Get-NetFirewallRule -DisplayName 'Portable PXE DHCP' -ErrorAction SilentlyContinue | Remove-NetFirewallRule
New-NetFirewallRule -DisplayName 'Portable PXE DHCP' -Direction Inbound -Action Allow `
  -Program $exe -Protocol UDP -LocalPort 67 -Profile Private
Write-Host 'Firewall rule installed for Private networks. Keep the LAN profile Private.'
