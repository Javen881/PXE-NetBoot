$ErrorActionPreference = 'Stop'
$env:CGO_ENABLED = '0'
$env:GOOS = 'windows'
$env:GOARCH = 'amd64'
go test ./...
New-Item -ItemType Directory -Force dist | Out-Null
go build -trimpath -ldflags '-s -w' -o dist/windows-pxe-dhcp.exe .
Copy-Item config.example.json,start.cmd,check.cmd,firewall-install.ps1,README.md,LICENSE -Destination dist
$hash = (Get-FileHash dist/windows-pxe-dhcp.exe -Algorithm SHA256).Hash.ToLowerInvariant()
"$hash  windows-pxe-dhcp.exe" | Set-Content dist/SHA256.txt -Encoding ascii
Compress-Archive -Force -Path dist/windows-pxe-dhcp.exe,dist/config.example.json,dist/start.cmd,dist/check.cmd,dist/firewall-install.ps1,dist/README.md,dist/LICENSE,dist/SHA256.txt -DestinationPath dist/windows-pxe-dhcp-amd64.zip
