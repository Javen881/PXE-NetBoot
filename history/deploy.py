#!/usr/bin/python3
"""Install isolated NetBoot services; never modify original ISO files."""
import os, pathlib, subprocess, shutil, json, hashlib
P = pathlib.Path
root = P('/home/javen03/data/netboot')
web = root / 'http'
cfg = root / 'config'
ip = '192.168.10.220'
base = f'http://{ip}:8088'
def run(*args):
    subprocess.run(args, check=True)
def write(path, data):
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(data)
def extract(iso, member, target):
    target.parent.mkdir(parents=True, exist_ok=True)
    with target.open('wb') as f:
        subprocess.run(['bsdtar','-xOf',str(iso),member],stdout=f,check=True)
    if not target.stat().st_size: raise RuntimeError(f'Empty file: {target}')
assert os.geteuid() == 0
for d in (web, cfg, root/'tftp',web/'ubuntu',web/'pe',web/'windows',web/'esxi',web/'boot'):
    d.mkdir(parents=True,exist_ok=True)
iso = root/'iso'
ubuntu = iso/'ubuntu-26.04.1-desktop-amd64.iso'
windows = iso/'SW_DVD9_WIN_ENT_LTSC_2021_64BIT_ChnSimp_MLF_X22-84402.ISO'
full = iso/'Full.iso'
esxi = iso/'DellEMC-ESXi-6.7U3-17700523-A11-customized.iso'
for p in (ubuntu,windows,full,esxi):
    if not p.is_file(): raise RuntimeError(f'Missing ISO: {p}')
extract(ubuntu,'casper/vmlinuz',web/'ubuntu/vmlinuz')
extract(ubuntu,'casper/initrd',web/'ubuntu/initrd')
for member in ('modules/PE10.WIM','modules/PE8.WIM','boot/BOOT.SDI'):
    extract(full,member,web/'pe'/P(member).name.lower())
run('7z','x','-y',f'-o{web / "windows"}',str(windows))
run('bsdtar','-xf',str(esxi),'-C',str(web/'esxi'),'--no-same-owner')
# ESXi ISO uses uppercase names; its boot.cfg uses lowercase module names.
for p in sorted((web/'esxi').rglob('*'),key=lambda p:len(p.parts),reverse=True):
    if p.name != p.name.lower(): p.rename(p.with_name(p.name.lower()))
bootcfg=(web/'esxi/boot.cfg').read_text()
bootcfg='\n'.join(line for line in bootcfg.splitlines() if not line.startswith('prefix='))
bootcfg=bootcfg.replace('kernel=/','kernel=').replace('modules=/','modules=').replace(' --- /',' --- ')
write(web/'esxi/boot.cfg',f'prefix={base}/esxi\n'+bootcfg+'\n')
for src,dst in [('undionly.kpxe','undionly.kpxe'),('ipxe-amd64.efi','ipxe.efi'),('snponly.efi','snponly.efi')]:
    shutil.copyfile('/usr/lib/ipxe/'+src,root/'tftp'/dst)
shutil.copyfile('/usr/lib/ipxe/ipxe.iso',web/'boot/ipxe.iso')
shutil.copyfile(root/'tftp/ipxe.efi',web/'boot/ipxe.efi')
run('curl','--fail','--location','--retry','3','--output',str(web/'boot/wimboot'),
    'https://github.com/ipxe/wimboot/releases/latest/download/wimboot')
write(web/'windows/winpeshl.ini','[LaunchApps]\n"%SYSTEMROOT%\\System32\\cmd.exe", "/c X:\\Windows\\System32\\netboot.cmd"\n')
write(web/'windows/netboot.cmd',r'''@echo off
wpeinit
echo Connecting to the LAN Windows installation source...
net use Z: \\192.168.10.220\netboot-win /user:netboot-guest ""
if errorlevel 1 goto failed
Z:\setup.exe
goto end
:failed
echo Cannot reach the installation share. Check NIC drivers and LAN connectivity.
echo This entry requires the local LAN or a site-to-site VPN. Do not expose SMB publicly.
ipconfig
:end
cmd.exe
'''.replace('\n','\r\n'))
write(web/'boot.ipxe',r'''#!ipxe
set base BASE
:menu
menu NetBoot - local installation media
item ubuntu Ubuntu Desktop 26.04.1 amd64
item windows Windows Enterprise LTSC 2021 x64 - LAN install
item pe10 STBOOT PE10 - maintenance (compatibility test)
item pe8 STBOOT PE8 x86 - Legacy BIOS only
item esxi DellEMC ESXi 6.7 U3 - hardware compatibility required
item shell iPXE shell
item local Exit to firmware
choose --default local --timeout 30000 target || goto local
imgfree
goto ${target}
:ubuntu
kernel ${base}/ubuntu/vmlinuz initrd=initrd.magic boot=casper ip=dhcp url=${base}/iso/ubuntu.iso --- || goto failed
initrd --name initrd ${base}/ubuntu/initrd || goto failed
boot || goto failed
:windows
kernel ${base}/boot/wimboot || goto failed
initrd ${base}/windows/winpeshl.ini winpeshl.ini || goto failed
initrd ${base}/windows/netboot.cmd netboot.cmd || goto failed
initrd ${base}/windows/sources/boot.wim boot.wim || goto failed
boot || goto failed
:pe10
set pe pe10.wim
goto pe
:pe8
iseq ${platform} pcbios || goto failed
set pe pe8.wim
:pe
kernel ${base}/boot/wimboot || goto failed
initrd ${base}/windows/boot/bcd BCD || goto failed
initrd ${base}/pe/boot.sdi boot.sdi || goto failed
initrd ${base}/pe/${pe} boot.wim || goto failed
boot || goto failed
:esxi
iseq ${platform} efi && goto esxi_efi || goto esxi_bios
:esxi_efi
chain ${base}/esxi/efi/boot/bootx64.efi -c ${base}/esxi/boot.cfg || goto failed
:esxi_bios
chain ${base}/esxi/mboot.c32 -c ${base}/esxi/boot.cfg || goto failed
:shell
shell
goto menu
:failed
echo Boot failed. Check server access log, network and firmware compatibility.
prompt Press a key to return to the menu
goto menu
:local
exit
'''.replace('BASE',base))
write(root/'tftp/boot.ipxe',f'#!ipxe\ndhcp || exit\nchain {base}/boot.ipxe\n')
write(web/'index.html','''<!doctype html><meta charset="utf-8"><title>NetBoot</title>
<h1>本地镜像网络安装</h1><p>服务器资源已准备，需客户端实际启动验证。</p>
<ul><li>Ubuntu Desktop 26.04.1</li><li>Windows LTSC 2021（局域网共享安装源）</li>
<li>STBOOT PE10 / PE8（待兼容性测试）</li><li>Dell EMC ESXi 6.7 U3</li></ul>
<p><a href="boot.ipxe">iPXE 启动菜单</a> · <a href="boot/ipxe.iso">iPXE 启动光盘</a></p>
<p>iPXE 命令：<code>dhcp &amp;&amp; chain http://192.168.10.220:8088/boot.ipxe</code></p>
''')
write(cfg/'nginx.conf',f'''worker_processes 2;
pid /run/netboot-http/nginx.pid;
error_log /var/log/netboot/error.log;
events {{ worker_connections 512; }}
http {{
 include /etc/nginx/mime.types;
 default_type application/octet-stream;
 access_log /var/log/netboot/access.log;
 sendfile on;
 server {{
  listen {ip}:8088;
  server_name _;
  root {web};
  index index.html;
  autoindex off;
  location = /iso/ubuntu.iso {{ alias {ubuntu}; }}
  location / {{ try_files $uri $uri/ =404; }}
 }}
}}
''')
write(cfg/'dnsmasq.conf',f'''port=0
interface=enp4s0
bind-interfaces
enable-tftp=enp4s0
tftp-root={root}/tftp
tftp-port-range=40000,40031
log-facility=-
''')
write(cfg/'smb.conf',f'''[global]
workgroup = WORKGROUP
server role = standalone server
interfaces = lo {ip}/24
bind interfaces only = yes
smb ports = 445
server min protocol = SMB2
map to guest = Bad User
guest account = javen03
load printers = no
disable spoolss = yes
printing = bsd
printcap name = /dev/null
pid directory = /run/netboot-smb
lock directory = /run/netboot-smb
state directory = /var/lib/netboot-smb
cache directory = /var/cache/netboot-smb
log file = /var/log/netboot/smb.%m.log
[netboot-win]
path = {web}/windows
read only = yes
guest ok = yes
guest only = yes
hosts allow = 127.0.0.1 192.168.10.0/24
hosts deny = ALL
''')
units={
'netboot-http':f'''[Unit]
Description=NetBoot HTTP on port 8088
After=network-online.target
Wants=network-online.target
[Service]
Type=simple
User=javen03
Group=javen03
RuntimeDirectory=netboot-http
LogsDirectory=netboot
ExecStart=/usr/sbin/nginx -c {cfg}/nginx.conf -g "daemon off;"
Restart=on-failure
NoNewPrivileges=true
PrivateTmp=true
[Install]
WantedBy=multi-user.target
''',
'netboot-tftp':f'''[Unit]
Description=NetBoot TFTP only (no DHCP)
After=network-online.target
Wants=network-online.target
[Service]
ExecStartPre=/usr/sbin/dnsmasq --test --conf-file={cfg}/dnsmasq.conf
ExecStart=/usr/sbin/dnsmasq --keep-in-foreground --user=javen03 --conf-file={cfg}/dnsmasq.conf
Restart=on-failure
[Install]
WantedBy=multi-user.target
''',
'netboot-smb':f'''[Unit]
Description=NetBoot LAN-only read-only Windows installation source
After=network-online.target
Wants=network-online.target
[Service]
Type=simple
RuntimeDirectory=netboot-smb
StateDirectory=netboot-smb
CacheDirectory=netboot-smb
ExecStart=/usr/sbin/smbd --foreground --no-process-group --configfile={cfg}/smb.conf
Restart=on-failure
[Install]
WantedBy=multi-user.target
'''}
for name,unit in units.items():write(P('/etc/systemd/system')/(name+'.service'),unit)
write(cfg/'openwrt-pxe.conf.example',f'''# Replace TEST_MAC with the test machine's wired MAC before including this file.
dhcp-host=TEST_MAC,set:pxetest
dhcp-userclass=set:ipxe,iPXE
dhcp-match=set:bios,option:client-arch,0
dhcp-match=set:efi64,option:client-arch,7
dhcp-match=set:efi64,option:client-arch,9
dhcp-boot=tag:pxetest,tag:ipxe,{base}/boot.ipxe
dhcp-boot=tag:pxetest,tag:!ipxe,tag:bios,undionly.kpxe,,{ip}
dhcp-boot=tag:pxetest,tag:!ipxe,tag:efi64,ipxe.efi,,{ip}
''')
run('dnsmasq','--test',f'--conf-file={cfg}/dnsmasq.conf')
for directory in ('/run/netboot-smb','/var/lib/netboot-smb','/var/cache/netboot-smb','/var/log/netboot'):
    P(directory).mkdir(parents=True,exist_ok=True)
    P(directory).chmod(0o755)
shutil.chown('/var/log/netboot',user='javen03',group='javen03')
for directory in (web,cfg,root/'tftp'):
    for item in [directory,*directory.rglob('*')]:
        if not item.is_symlink(): item.chmod(0o755 if item.is_dir() or (item.is_relative_to(web/'windows') and item.suffix.lower() in ('.exe','.dll','.ocx','.cpl')) else 0o644)
run('testparm','-s',str(cfg/'smb.conf'))
# Newly installed distro defaults are unused; existing GitLab's port 80 is untouched.
run('systemctl','disable','--now','nginx','smbd','nmbd')
run('systemctl','daemon-reload')
run('systemctl','enable','--now',*units)
run('systemctl','is-active',*units)
print('NetBoot resources prepared and services started.',flush=True)
