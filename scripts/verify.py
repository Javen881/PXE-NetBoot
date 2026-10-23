#!/usr/bin/env python3
"""Read-only checks; not proof of a complete OS installation."""
import argparse,urllib.request,subprocess,pathlib
p=argparse.ArgumentParser()
p.add_argument('--server',default='192.168.10.220')
p.add_argument('--root',default='/home/javen03/data/netboot')
a=p.parse_args()
base='http://'+a.server+':8088'
urls=['/','/boot.ipxe','/boot/wimboot','/ubuntu/vmlinuz','/ubuntu/initrd','/iso/ubuntu.iso',
'/windows/sources/boot.wim','/windows11/sources/boot.wim','/windows11/netboot.cmd',
'/pe/pe10.wim','/pe/pe8.wim','/pe/boot.sdi','/esxi/efi/boot/bootx64.efi','/esxi/mboot.c32']
for line in (pathlib.Path(a.root)/'http/esxi/boot.cfg').read_text().splitlines():
    if line.startswith(('kernel=','modules=')):
        urls.extend('/esxi/'+v.strip().split()[0] for v in line.split('=',1)[1].split('---'))
for url in urls:
    with urllib.request.urlopen(urllib.request.Request(base+url,method='HEAD'),timeout=15) as r:
        assert r.status==200 and int(r.headers['Content-Length'])>0,url
print('HTTP checked:',len(urls))
with urllib.request.urlopen(urllib.request.Request(base+'/iso/ubuntu.iso',headers={'Range':'bytes=0-2047'}),timeout=15) as r:
    assert r.status==206 and len(r.read(2049))==2048
for share in ('netboot-win','netboot-win11'):
    subprocess.run(['smbclient','//'+a.server+'/'+share,'-U','netboot-guest%',
                    '-c','cd sources; ls install.wim'],check=True)
print('HTTP range and both SMB shares passed. Test PXE and installation on a client next.')

