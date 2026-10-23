#!/bin/bash
set -euo pipefail
test "$EUID" -eq 0
systemctl disable --now netboot-http netboot-tftp netboot-smb
echo "Services stopped and disabled. Units and media retained."
echo "Restore router dnsmasq backup separately, as documented."

