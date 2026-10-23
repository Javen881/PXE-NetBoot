#!/bin/bash
set -euo pipefail
root="$(realpath "$1")"
test "$EUID" -eq 0
for unit in netboot-http netboot-tftp netboot-smb; do
  if test -e "/etc/systemd/system/$unit.service"; then
    echo "Existing unit $unit; refusing to overwrite." >&2
    exit 1
  fi
done
if ss -H -lntup | grep -Eq ':(445|69|8088)[[:space:]]'; then
  echo "Required port in use; resolve service conflicts first." >&2
  exit 1
fi
id javen03 >/dev/null
install -d -o javen03 -g javen03 /run/netboot-http /var/log/netboot
install -d /run/netboot-smb /var/lib/netboot-smb /var/cache/netboot-smb
nginx -t -c "$root/config/nginx.conf"
dnsmasq --test --conf-file="$root/config/dnsmasq.conf"
testparm -s "$root/config/smb.conf"
install -m 644 "$root"/systemd/netboot-*.service /etc/systemd/system/
systemctl daemon-reload
systemctl enable --now netboot-http netboot-tftp netboot-smb
systemctl is-active netboot-http netboot-tftp netboot-smb

