$ErrorActionPreference = 'Stop'
$env:CGO_ENABLED = '0'
$env:GOOS = 'windows'
$env:GOARCH = 'amd64'
go test ./...
New-Item -ItemType Directory -Force dist | Out-Null
go build -trimpath -ldflags '-s -w' -o dist/windows-pxe-dhcp.exe .
go run github.com/akavel/rsrc@v0.10.2 -manifest gui/app.manifest -o gui/rsrc_windows_amd64.syso
go build -trimpath -ldflags '-s -w -H windowsgui' -o dist/windows-pxe-dhcp-gui.exe ./gui
Copy-Item config.example.json,start.cmd,check.cmd,firewall-install.ps1,README.md,LICENSE -Destination dist
$serverHash = (Get-FileHash dist/windows-pxe-dhcp.exe -Algorithm SHA256).Hash.ToLowerInvariant()
$guiHash = (Get-FileHash dist/windows-pxe-dhcp-gui.exe -Algorithm SHA256).Hash.ToLowerInvariant()
"$serverHash  windows-pxe-dhcp.exe`n$guiHash  windows-pxe-dhcp-gui.exe" | Set-Content dist/SHA256.txt -Encoding ascii
Compress-Archive -Force -Path dist/windows-pxe-dhcp.exe,dist/windows-pxe-dhcp-gui.exe,dist/config.example.json,dist/start.cmd,dist/check.cmd,dist/firewall-install.ps1,dist/README.md,dist/LICENSE,dist/SHA256.txt -DestinationPath dist/windows-pxe-dhcp-amd64.zip
