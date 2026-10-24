# 完整源码与配置 Demo

本文件逐一内嵌仓库的脚本、配置和启动入口。先按 [操作手册](DEMO.md) 使用，history/ 仅供追溯，不直接执行。

## config/dnsmasq.conf

[源文件](../config/dnsmasq.conf)

````ini
port=0
interface=enp4s0
bind-interfaces
enable-tftp=enp4s0
tftp-root=/home/javen03/data/netboot/tftp
tftp-port-range=40000,40031
log-facility=-
````

## config/nginx.conf

[源文件](../config/nginx.conf)

````ini
worker_processes 2;
pid /run/netboot-http/nginx.pid;
error_log /var/log/netboot/error.log;
events { worker_connections 512; }
http {
 include /etc/nginx/mime.types;
 default_type application/octet-stream;
 access_log /var/log/netboot/access.log;
 sendfile on;
 server {
  listen 192.168.10.220:8088;
  server_name _;
  root /home/javen03/data/netboot/http;
  index index.html;
  autoindex off;
  location = /iso/ubuntu.iso { alias /home/javen03/data/netboot/iso/ubuntu-26.04.1-desktop-amd64.iso; }
  location / { try_files $uri $uri/ =404; }
 }
}
````

## config/smb.conf

[源文件](../config/smb.conf)

````ini
[global]
workgroup = WORKGROUP
server role = standalone server
interfaces = lo 192.168.10.220/24
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
path = /home/javen03/data/netboot/http/windows
read only = yes
guest ok = yes
guest only = yes
hosts allow = 127.0.0.1 192.168.10.0/24 192.168.18.0/24
hosts deny = ALL

[netboot-win11]
path = /home/javen03/data/netboot/http/windows11
read only = yes
guest ok = yes
guest only = yes
hosts allow = 127.0.0.1 192.168.10.0/24 192.168.18.0/24
hosts deny = ALL
````

## examples/boot.ipxe

[源文件](../examples/boot.ipxe)

````ipxe
#!ipxe
set base http://192.168.10.220:8088
:menu
menu NetBoot - local installation media
item ubuntu Ubuntu Desktop 26.04.1 amd64
item windows Windows Enterprise LTSC 2021 x64 - LAN or site VPN
item pe10 STBOOT PE10 - maintenance (compatibility test)
item pe8 STBOOT PE8 x86 - Legacy BIOS only
item esxi DellEMC ESXi 6.7 U3 - hardware compatibility required
item win11 Windows 11 24H2 x64 - LAN or site VPN
item shell iPXE shell
item local Exit to firmware
choose --default local --timeout 30000 target || goto local
imgfree
goto ${target}
:ubuntu
kernel ${base}/ubuntu/vmlinuz initrd=initrd.magic boot=casper ip=dhcp url=${base}/iso/ubuntu.iso cloud-init=disabled --- || goto failed
initrd --name initrd ${base}/ubuntu/initrd || goto failed
boot || goto failed
:windows
kernel ${base}/boot/wimboot index=2 || goto failed
iseq ${platform} efi && initrd ${base}/windows/bootmgr.efi bootmgfw.efi || initrd ${base}/windows/bootmgr bootmgr || goto failed
initrd ${base}/windows/winpeshl.ini winpeshl.ini || goto failed
initrd ${base}/windows/netboot.cmd netboot.cmd || goto failed
initrd ${base}/windows/sources/boot.wim boot.wim || goto failed
boot || goto failed
:pe10
set pe pe10.wim
goto pe
:pe8
iseq ${platform} pcbios || goto pe8_unsupported
set pe pe8.wim
:pe
kernel ${base}/boot/wimboot || goto failed
iseq ${platform} efi && goto pe_efi || goto pe_bios
:pe_efi
initrd ${base}/pe/bootmgfw.efi bootx64.efi || goto failed
initrd ${base}/pe/bcd-efi BCD || goto failed
initrd ${base}/pe/bcd-efi EFI/Microsoft/Boot/BCD || goto failed
goto pe_common
:pe_bios
initrd ${base}/pe/bootmgr bootmgr || goto failed
initrd ${base}/pe/bcd BCD || goto failed
:pe_common
initrd ${base}/pe/boot.sdi boot.sdi || goto failed
initrd ${base}/pe/${pe} boot.wim || goto failed
boot || goto failed
:esxi
iseq ${platform} efi && goto esxi_efi || goto esxi_bios
:esxi_efi
chain ${base}/esxi/efi/boot/bootx64.efi -c ${base}/esxi/boot.cfg || goto failed
:esxi_bios
chain ${base}/esxi/mboot.c32 -c ${base}/esxi/boot.cfg || goto failed
:pe8_unsupported
echo PE8 is 32-bit. Use Legacy BIOS or choose PE10.
prompt Press a key to return
goto menu
:win11
kernel ${base}/boot/wimboot index=2 || goto failed
iseq ${platform} efi && initrd ${base}/windows11/bootmgr.efi bootmgfw.efi || initrd ${base}/windows11/bootmgr bootmgr || goto failed
initrd ${base}/windows11/winpeshl.ini winpeshl.ini || goto failed
initrd ${base}/windows11/netboot.cmd netboot.cmd || goto failed
initrd ${base}/windows11/sources/boot.wim boot.wim || goto failed
boot || goto failed
:shell
shell
goto menu
:failed
echo Boot failed. Check server access log, network and firmware compatibility.
prompt Press a key to return to the menu
goto menu
:local
exit
````

## examples/netboot.cmd

[源文件](../examples/netboot.cmd)

````bat
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
````

## examples/windows11/netboot.cmd

[源文件](../examples/windows11/netboot.cmd)

````bat
@echo off
setlocal EnableExtensions
echo Initializing Windows installation network...
wpeinit
set "attempt=0"
set "max_attempts=12"
:connect
set /a attempt+=1 >nul
echo Connecting to Windows 11 installation source [%attempt%/%max_attempts%]...
net use Z: \\192.168.10.220\netboot-win11 /user:netboot-guest ""
if not errorlevel 1 goto check_source
rem A previous successful mapping may already exist when this script is retried.
net use Z: | findstr /L /C:"\\192.168.10.220\netboot-win11" >nul
if not errorlevel 1 goto check_source
goto retry
:check_source
if exist Z:\sources\install.wim goto install
echo Installation source is not readable yet.
:retry
if %attempt% GEQ %max_attempts% goto network_failed
echo Waiting 5 seconds before retrying...
ping -n 6 127.0.0.1 >nul
goto connect
:install
echo Starting Windows 11 Setup...
if not exist X:\sources\setup.exe goto setup_missing
X:\sources\setup.exe /installfrom:Z:\sources\install.wim
set "setup_rc=%errorlevel%"
echo Windows Setup returned exit code %setup_rc%.
echo Setup logs: X:\Windows\Panther and X:\$WINDOWS.~BT\Sources\Panther
goto shell
:setup_missing
echo Native Windows Setup is missing. Check boot.wim index.
goto shell
:network_failed
echo Unable to connect after %max_attempts% attempts.
echo Check the VPN route, SMB access and the errors above.
ipconfig
net use
:shell
echo To retry: X:\Windows\System32\netboot.cmd
cmd.exe
endlocal
````

## examples/windows11/winpeshl.ini

[源文件](../examples/windows11/winpeshl.ini)

````ini
[LaunchApps]
"%SYSTEMROOT%\System32\cmd.exe", "/c X:\Windows\System32\netboot.cmd"
````

## examples/winpeshl.ini

[源文件](../examples/winpeshl.ini)

````ini
[LaunchApps]
"%SYSTEMROOT%\System32\cmd.exe", "/c X:\Windows\System32\netboot.cmd"
````

## history/README.md

[源文件](../history/README.md)

````markdown
# 历史脚本（仅供审计，不作为执行入口）

完整保留现有部署的 deploy.py、finalize.py、vpn-enable.py、verify.py 和早期路由器模板。

- deploy.py 会覆盖配置并停用发行版默认服务，包含早期 Windows 启动逻辑。
- finalize.py 依赖 /tmp 中的文件，不能直接用于另一台机器。
- vpn-enable.py 是一次性补丁，会覆盖旧脚本；不会创建 VPN。
- verify.py 未覆盖后加的 Windows 11。
- 早期模板使用 TEST_MAC，当前实际规则见 ../router/dnsmasq-pxe.conf。

请使用 ../scripts/prepare.py 和 ../docs/DEMO.md；不要直接执行历史脚本。


另外保留 Linux initrd、TFTP、Win11 接入及重试安装的一次性补丁。它们包含临时路径或一次性断言，仅供追溯；当前 Win11 脚本见 examples/windows11/netboot.cmd。
````

## history/deploy.py

[源文件](../history/deploy.py)

````python
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
iseq ${platform} efi && initrd ${base}/windows/efi/microsoft/boot/bcd BCD || initrd ${base}/windows/boot/bcd BCD || goto failed
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
````

## history/finalize.py

[源文件](../history/finalize.py)

````python
import pathlib,subprocess,shutil
P=pathlib.Path
r=P('/home/javen03/data/netboot')
def run(*args):subprocess.run(args,check=True)
for d in ['/run/netboot-smb','/var/lib/netboot-smb','/var/cache/netboot-smb','/var/log/netboot']:
 P(d).mkdir(parents=True,exist_ok=True)
shutil.chown('/var/log/netboot',user='javen03',group='javen03')
menu=r/'http/boot.ipxe'
s=menu.read_text().replace('STBOOT PE8 - maintenance (compatibility test)','STBOOT PE8 x86 - Legacy BIOS only')
s=s.replace(':pe8\nset pe',':pe8\niseq ${platform} pcbios || goto pe8_unsupported\nset pe')
s=s.replace(':shell\n',':pe8_unsupported\necho PE8 is 32-bit. Use Legacy BIOS or choose PE10.\nprompt Press a key to return\ngoto menu\n:shell\n')
menu.write_text(s)
run('testparm','-s',str(r/'config/smb.conf'))
run('systemctl','disable','--now','nginx','smbd','nmbd')
run('systemctl','daemon-reload')
run('systemctl','enable','--now','netboot-http','netboot-tftp','netboot-smb')
run('systemctl','is-active','netboot-http','netboot-tftp','netboot-smb')
(r/'scripts').mkdir(exist_ok=True)
shutil.copyfile('/tmp/netboot-deploy.py',r/'scripts/deploy.py')
shutil.copyfile('/tmp/netboot-finalize.py',r/'scripts/finalize.py')

# Windows SMB executable images require execute permission.
for image in (r/'http/windows').rglob('*'):
 if image.is_file() and image.suffix.lower() in ('.exe','.dll','.ocx','.cpl'): image.chmod(0o755)
````

## history/install-win11-retry.py

[源文件](../history/install-win11-retry.py)

````python
from pathlib import Path
import datetime,shutil
root=Path('/home/javen03/data/netboot')
target=root/'http/windows11/netboot.cmd'
backup=root/'backups'/('win11-retry-'+datetime.datetime.now().strftime('%Y%m%d-%H%M%S'))
backup.mkdir(parents=True)
shutil.copy2(target,backup/'netboot.cmd')
data=Path('/tmp/win11-retry.cmd').read_text()
assert 'netboot-win11' in data and 'max_attempts=12' in data
tmp=target.with_suffix('.new')
tmp.write_bytes(data.replace('\n','\r\n').encode('ascii'))
tmp.chmod(0o644)
tmp.replace(target)
print('Updated:',target)
print('Backup:',backup)
````

## history/netboot-fix-linux-initrd.py

[源文件](../history/netboot-fix-linux-initrd.py)

````python
from pathlib import Path
from datetime import datetime
import shutil
r=Path('/home/javen03/data/netboot')
p=r/'http/boot.ipxe'
backup=r/'backups'/('linux-initrd-'+datetime.now().strftime('%Y%m%d-%H%M%S'))
backup.mkdir(parents=True)
shutil.copyfile(p,backup/'boot.ipxe')
old='initrd ${base}/ubuntu/initrd initrd || goto failed'
new='initrd --name initrd ${base}/ubuntu/initrd || goto failed'
s=p.read_text()
assert old in s or new in s,'Unexpected Ubuntu entry; refusing to modify'
s=s.replace(old,new).replace('vmlinuz initrd=initrd boot=casper','vmlinuz initrd=initrd.magic boot=casper')
p.write_text(s)
p.chmod(0o644)
p=r/'scripts/deploy.py'
if p.exists():
 s=p.read_text().replace(old,new).replace('vmlinuz initrd=initrd boot=casper','vmlinuz initrd=initrd.magic boot=casper')
 p.write_text(s)
print('Ubuntu initrd loading corrected; backup:',backup)
````

## history/netboot-tftp-fix.py

[源文件](../history/netboot-tftp-fix.py)

````python
from pathlib import Path
import subprocess
p=Path('/home/javen03/data/netboot/config/dnsmasq.conf')
p.write_text(p.read_text().replace('no-dhcp-interface=enp4s0\n','').replace('enable-tftp\n','enable-tftp=enp4s0\n'))
subprocess.run(['dnsmasq','--test','--conf-file='+str(p)],check=True)
subprocess.run(['systemctl','restart','netboot-tftp'],check=True)
````

## history/openwrt-pxe.conf.early-example

[源文件](../history/openwrt-pxe.conf.early-example)

````text
# Replace TEST_MAC with the test machine's wired MAC before including this file.
dhcp-host=TEST_MAC,set:pxetest
dhcp-userclass=set:ipxe,iPXE
dhcp-match=set:bios,option:client-arch,0
dhcp-match=set:efi64,option:client-arch,7
dhcp-match=set:efi64,option:client-arch,9
dhcp-boot=tag:pxetest,tag:ipxe,http://192.168.10.220:8088/boot.ipxe
dhcp-boot=tag:pxetest,tag:!ipxe,tag:bios,undionly.kpxe,,192.168.10.220
dhcp-boot=tag:pxetest,tag:!ipxe,tag:efi64,ipxe.efi,,192.168.10.220
````

## history/pxe-enable-win11.py

[源文件](../history/pxe-enable-win11.py)

````python
from pathlib import Path
import subprocess, shutil, datetime, os
r=Path('/home/javen03/data/netboot')
dst=r/'http/windows11'
assert not dst.exists(), 'windows11 already exists; inspect before proceeding'
backup=r/'backups'/('win11-'+datetime.datetime.now().strftime('%Y%m%d-%H%M%S'))
backup.mkdir(parents=True)
for p in [r/'http/boot.ipxe',r/'http/index.html',r/'config/smb.conf']:
    shutil.copy2(p,backup/p.name)
stage=r/'http/.windows11-staging'
assert not stage.exists()
stage.mkdir()
subprocess.run(['7z','x','-y','-o'+str(stage),str(r/'iso/Win11_24H2_Pro_Chinese_Simplified_x64.iso')],check=True,stdout=subprocess.DEVNULL)
for name in ['sources/boot.wim','sources/install.wim','bootmgr']:
    assert (stage/name).stat().st_size>0
subprocess.run(['wimlib-imagex','info',str(stage/'sources/boot.wim'),'2'],check=True)
cmd=(r/'http/windows/netboot.cmd').read_bytes().replace(b'netboot-win ',b'netboot-win11 ')
assert b'netboot-win11' in cmd
(stage/'netboot.cmd').write_bytes(cmd)
shutil.copy2(r/'http/windows/winpeshl.ini',stage/'winpeshl.ini')
for p in [stage,*stage.rglob('*')]:
    p.chmod(0o755 if p.is_dir() or p.suffix.lower() in ('.exe','.dll','.ocx','.cpl') else 0o644)
stage.rename(dst)
smb=r/'config/smb.conf'
old=smb.read_text()
assert '[netboot-win11]' not in old
smb.write_text(old+'''
[netboot-win11]
path = /home/javen03/data/netboot/http/windows11
read only = yes
guest ok = yes
guest only = yes
hosts allow = 127.0.0.1 192.168.10.0/24 192.168.18.0/24
hosts deny = ALL
''')
try:
    subprocess.run(['testparm','-s',str(smb)],check=True,stdout=subprocess.DEVNULL)
    subprocess.run(['systemctl','kill','--kill-whom=main','--signal=HUP','netboot-smb'],check=True)
except Exception:
    shutil.copy2(backup/'smb.conf',smb)
    raise
menu=r/'http/boot.ipxe'
m=menu.read_text()
assert 'item win11 ' not in m and ':win11\n' not in m
m=m.replace('item shell ', 'item win11 Windows 11 24H2 x64 - LAN or site VPN\nitem shell ',1)
block=''' :win11
kernel ${base}/boot/wimboot index=2 || goto failed
iseq ${platform} efi && initrd ${base}/windows11/bootmgr.efi bootmgfw.efi || initrd ${base}/windows11/bootmgr bootmgr || goto failed
initrd ${base}/windows11/winpeshl.ini winpeshl.ini || goto failed
initrd ${base}/windows11/netboot.cmd netboot.cmd || goto failed
initrd ${base}/windows11/sources/boot.wim boot.wim || goto failed
boot || goto failed
'''.lstrip()
m=m.replace(':shell\n',block+':shell\n',1)
temp=menu.with_suffix('.tmp')
temp.write_text(m)
temp.chmod(0o644)
temp.replace(menu)
page=r/'http/index.html'
page.write_text(page.read_text().replace('</ul>','<li>Windows 11 24H2 x64（局域网/VPN 安装）</li></ul>',1))
print('Windows 11 deployed; backup:',backup)
````

## history/verify.py

[源文件](../history/verify.py)

````python
import pathlib,urllib.request,socket,struct,hashlib
r=pathlib.Path('/home/javen03/data/netboot')
b='http://192.168.10.220:8088'
files=['/','/boot.ipxe','/ubuntu/vmlinuz','/ubuntu/initrd','/iso/ubuntu.iso','/boot/wimboot','/pe/pe10.wim','/pe/pe8.wim','/pe/boot.sdi','/windows/boot/bcd','/windows/sources/boot.wim','/windows/sources/install.wim','/windows/setup.exe','/windows/netboot.cmd','/windows/winpeshl.ini','/esxi/efi/boot/bootx64.efi','/esxi/mboot.c32']
for line in (r/'http/esxi/boot.cfg').read_text().splitlines():
 if line.startswith(('kernel=','modules=')):
  files.extend('/esxi/'+f.strip() for f in line.split('=',1)[1].split('---'))
for f in files:
 with urllib.request.urlopen(urllib.request.Request(b+f,method='HEAD'),timeout=10) as resp:
  assert resp.status==200 and int(resp.headers['Content-Length'])>0,(f,resp.status)
print('HTTP: all',len(files),'menu files and ESXi modules return 200')
with urllib.request.urlopen(urllib.request.Request(b+'/iso/ubuntu.iso',headers={'Range':'bytes=0-2047'})) as res:
 assert res.status==206 and len(res.read())==2048
print('ISO partial download: 206 OK')
for name in ['undionly.kpxe','ipxe.efi','boot.ipxe']:
 with socket.socket(socket.AF_INET,socket.SOCK_DGRAM) as s:
  s.settimeout(10);s.sendto(b'\x00\x01'+name.encode()+b'\x00octet\x00',('192.168.10.220',69))
  result=bytearray(); expected=1
  while True:
   packet,addr=s.recvfrom(4096);op,block=struct.unpack('!HH',packet[:4])
   assert op==3,(name,packet)
   s.sendto(struct.pack('!HH',4,block),addr)
   if block!=expected:continue
   result.extend(packet[4:]);expected=(expected+1)%65536
   if len(packet)<516:break
  assert hashlib.sha256(result).digest()==hashlib.sha256((r/'tftp'/name).read_bytes()).digest()
 print('TFTP:',name,'verified',len(result),'bytes')
import subprocess
subprocess.run(['smbclient','//192.168.10.220/netboot-win','-U','netboot-guest%','-c','ls; cd sources; ls *.wim'],check=True)
print('SMB installation source accessible')
````

## history/vpn-enable.py

[源文件](../history/vpn-enable.py)

````python
from pathlib import Path
import shutil,subprocess,datetime
r=Path('/home/javen03/data/netboot')
stamp=datetime.datetime.now().strftime('%Y%m%d-%H%M%S')
backup=r/'backups'/('vpn-'+stamp)
backup.mkdir(parents=True)
for rel in ('config/smb.conf','http/windows/netboot.cmd','http/boot.ipxe'):
 shutil.copyfile(r/rel,backup/Path(rel).name)
p=r/'config/smb.conf'
s=p.read_text()
s=s.replace('hosts allow = 127.0.0.1 192.168.10.0/24','hosts allow = 127.0.0.1 192.168.10.0/24 192.168.18.0/24')
p.write_text(s)
result=subprocess.run(['testparm','-s',str(p)],capture_output=True,text=True)
if result.returncode:
 shutil.copyfile(backup/'smb.conf',p)
 raise RuntimeError(result.stderr)
cmd=r'''@echo off
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
'''
(r/'http/windows/netboot.cmd').write_bytes(cmd.replace('\n','\r\n').encode('ascii'))
p=r/'http/boot.ipxe'
s=p.read_text().replace('Windows Enterprise LTSC 2021 x64 - LAN install','Windows Enterprise LTSC 2021 x64 - LAN or site VPN')
s=s.replace(':windows\nkernel ${base}/boot/wimboot ||', ':windows\nkernel ${base}/boot/wimboot index=2 ||')
p.write_text(s)
for rel in ('config/smb.conf','http/windows/netboot.cmd','http/boot.ipxe'):(r/rel).chmod(0o644)
subprocess.run(['systemctl','reload','netboot-smb'],check=False)
# The isolated unit has no ExecReload; send the documented Samba reload signal.
subprocess.run(['systemctl','kill','--kill-whom=main','--signal=HUP','netboot-smb'],check=True)
shutil.copyfile('/tmp/netboot-vpn-enable.py',r/'scripts/vpn-enable.py')
print('B subnet allowed; Windows native Setup and diagnostics updated. Backup:',backup)
````

## http/index.html

[源文件](../http/index.html)

````html
<!doctype html><meta charset="utf-8"><title>NetBoot</title>
<h1>本地镜像网络安装</h1><p>服务器资源已准备，需客户端实际启动验证。</p>
<ul><li>Ubuntu Desktop 26.04.1</li><li>Windows LTSC 2021（局域网共享安装源）</li>
<li>STBOOT PE10 / PE8（待兼容性测试）</li><li>Dell EMC ESXi 6.7 U3</li><li>Windows 11 24H2 x64（局域网/VPN 安装）</li></ul>
<p><a href="boot.ipxe">iPXE 启动菜单</a> · <a href="boot/ipxe.iso">iPXE 启动光盘</a></p>
<p>iPXE 命令：<code>dhcp &amp;&amp; chain http://192.168.10.220:8088/boot.ipxe</code></p>
````

## media.json

[源文件](../media.json)

````json
{
  "ubuntu": "ubuntu-26.04.1-desktop-amd64.iso",
  "windows": "SW_DVD9_WIN_ENT_LTSC_2021_64BIT_ChnSimp_MLF_X22-84402.ISO",
  "windows11": "Win11_24H2_Pro_Chinese_Simplified_x64.iso",
  "pe": "Full.iso",
  "esxi": "DellEMC-ESXi-6.7U3-17700523-A11-customized.iso"
}
````

## router/dnsmasq-pxe.conf

[源文件](../router/dnsmasq-pxe.conf)

````ini
# BEGIN NETBOOT PXE
dhcp-vendorclass=set:netboot_pxe,PXEClient
dhcp-userclass=set:netboot_ipxe,iPXE
dhcp-match=set:netboot_bios,option:client-arch,0
dhcp-match=set:netboot_efi64,option:client-arch,7
dhcp-match=set:netboot_efi64,option:client-arch,9
dhcp-boot=tag:netboot_ipxe,http://192.168.10.220:8088/boot.ipxe
dhcp-boot=tag:netboot_pxe,tag:!netboot_ipxe,tag:netboot_bios,undionly.kpxe,,192.168.10.220
dhcp-boot=tag:netboot_pxe,tag:!netboot_ipxe,tag:netboot_efi64,ipxe.efi,,192.168.10.220
# END NETBOOT PXE
````

## scripts/install-services.sh

[源文件](../scripts/install-services.sh)

````bash
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
````

## scripts/prepare.py

[源文件](../scripts/prepare.py)

````python
#!/usr/bin/env python3
"""Prepare a fresh NetBoot root. Does not alter running services or routers."""
import argparse, pathlib, subprocess, shutil, json, hashlib
P=pathlib.Path
REPO=P(__file__).resolve().parents[1]
def run(*args, **kw):
    return subprocess.run([str(x) for x in args],check=True,**kw)
def main():
    ap=argparse.ArgumentParser(description=__doc__)
    ap.add_argument('--root',required=True,help='New empty output directory, not the running deployment')
    ap.add_argument('--iso-dir',required=True)
    ap.add_argument('--check',action='store_true',help='Validate inputs only; write nothing')
    a=ap.parse_args()
    root=P(a.root).resolve(); iso=P(a.iso_dir).resolve()
    media=json.loads((REPO/'media.json').read_text())
    for name in media.values():
        if not (iso/name).is_file(): raise SystemExit('Missing ISO: '+str(iso/name))
    for exe in ('7z','bsdtar','curl','wimlib-imagex'):
        if not shutil.which(exe): raise SystemExit('Missing executable: '+exe)
    for name in ('undionly.kpxe','ipxe-amd64.efi','snponly.efi','ipxe.iso'):
        if not (P('/usr/lib/ipxe')/name).is_file(): raise SystemExit('Missing iPXE asset: '+name)
    if root.exists() and any(root.iterdir()): raise SystemExit('Refusing nonempty output root: '+str(root))
    if a.check:
        print('Inputs present; check mode did not test entire ISO integrity.'); return
    root.mkdir(parents=True,exist_ok=True)
    web=root/'http'
    for rel in ('http/boot','http/ubuntu','http/windows','http/windows11','http/pe','http/esxi','config','tftp','systemd'):
        (root/rel).mkdir(parents=True,exist_ok=True)
    # The ISO remains at its original location; record the source for reproduction.
    (root/'media-source.json').write_text(json.dumps({'iso_dir':str(iso),'media':media},indent=2)+'\n')
    def extract(key,member,destination):
        with destination.open('wb') as out:
            run('bsdtar','-xOf',iso/media[key],member,stdout=out)
        if not destination.stat().st_size: raise RuntimeError('Empty: '+str(destination))
    extract('ubuntu','casper/vmlinuz',web/'ubuntu/vmlinuz')
    extract('ubuntu','casper/initrd',web/'ubuntu/initrd')
    for key in ('windows','windows11'):
        run('7z','t',iso/media[key],stdout=subprocess.DEVNULL)
        run('7z','x','-y','-o'+str(web/key),iso/media[key],stdout=subprocess.DEVNULL)
        info=run('wimlib-imagex','info',web/key/'sources/boot.wim','2',capture_output=True,text=True).stdout
        if 'Windows Setup' not in info: raise SystemExit('Verify Setup index for '+key)
        if not (web/key/'sources/install.wim').is_file(): raise SystemExit('This demo requires install.wim: '+key)
        source=REPO/'examples' if key=='windows' else REPO/'examples/windows11'
        for f in ('netboot.cmd','winpeshl.ini'):
            data=(source/f).read_text().replace('\r\n','\n')
            (web/key/f).write_bytes(data.replace('\n','\r\n').encode('ascii'))
    for member,dst in [('modules/PE10.WIM','pe10.wim'),('modules/PE8.WIM','pe8.wim'),('boot/BOOT.SDI','boot.sdi')]:
        extract('pe',member,web/'pe'/dst)
    run('bsdtar','-xf',iso/media['esxi'],'-C',web/'esxi','--no-same-owner')
    for path in sorted((web/'esxi').rglob('*'),key=lambda p:len(p.parts),reverse=True):
        target=path.with_name(path.name.lower())
        if target!=path:
            if target.exists(): raise SystemExit('Filename collision: '+str(target))
            path.rename(target)
    cfg=web/'esxi/boot.cfg'
    cfg.chmod(0o644)  # ISO may preserve read-only file permissions.
    lines=[]
    for line in cfg.read_text().splitlines():
        if line.startswith('prefix='): continue
        if line.startswith('kernel='): line='kernel='+line[7:].lstrip('/')
        if line.startswith('modules='):
            line='modules='+' --- '.join(x.strip().lstrip('/') for x in line[8:].split('---'))
        lines.append(line)
    cfg.write_text('prefix=http://192.168.10.220:8088/esxi\n'+'\n'.join(lines)+'\n')
    for src,dst in [('undionly.kpxe','undionly.kpxe'),('ipxe-amd64.efi','ipxe.efi'),('snponly.efi','snponly.efi')]:
        shutil.copy2(P('/usr/lib/ipxe')/src,root/'tftp'/dst)
    shutil.copy2('/usr/lib/ipxe/ipxe.iso',web/'boot/ipxe.iso')
    shutil.copy2(root/'tftp/ipxe.efi',web/'boot/ipxe.efi')
    run('curl','--fail','--location','--retry','3',
        'https://github.com/ipxe/wimboot/releases/download/v2.9.0/wimboot','-o',web/'boot/wimboot')
    for src,dst in [('examples/boot.ipxe','http/boot.ipxe'),('http/index.html','http/index.html'),('tftp/boot.ipxe','tftp/boot.ipxe')]:
        shutil.copy2(REPO/src,root/dst)
    old='/home/javen03/data/netboot'
    for folder in ('config','systemd'):
        for source in (REPO/folder).iterdir():
            if not source.is_file(): continue
            text=source.read_text().replace(old,str(root))
            if source.name=='nginx.conf':
                text=text.replace(str(root/'iso'/media['ubuntu']),str(iso/media['ubuntu']))
            (root/folder/source.name).write_text(text)
    for path in root.rglob('*'):
        if path.is_symlink(): continue
        executable=path.is_relative_to(web) and path.suffix.lower() in ('.exe','.dll','.ocx','.cpl')
        path.chmod(0o755 if path.is_dir() or executable else 0o644)
    hashes={}
    for path in [* (root/'tftp').iterdir(),*(web/'boot').iterdir()]:
        if path.is_file(): hashes[str(path.relative_to(root))]=hashlib.sha256(path.read_bytes()).hexdigest()
    (root/'boot-assets.sha256.json').write_text(json.dumps(hashes,indent=2)+'\n')
    print('Prepared:',root)
    print('Services were not installed. See docs/DEMO.md before activation.')
if __name__=='__main__': main()
````

## scripts/rollback-services.sh

[源文件](../scripts/rollback-services.sh)

````bash
#!/bin/bash
set -euo pipefail
test "$EUID" -eq 0
systemctl disable --now netboot-http netboot-tftp netboot-smb
echo "Services stopped and disabled. Units and media retained."
echo "Restore router dnsmasq backup separately, as documented."
````

## scripts/verify.py

[源文件](../scripts/verify.py)

````python
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
````

## systemd/netboot-http.service

[源文件](../systemd/netboot-http.service)

````ini
[Unit]
Description=NetBoot HTTP on port 8088
After=network-online.target
Wants=network-online.target
[Service]
Type=simple
User=javen03
Group=javen03
RuntimeDirectory=netboot-http
LogsDirectory=netboot
ExecStart=/usr/sbin/nginx -c /home/javen03/data/netboot/config/nginx.conf -g "daemon off;"
Restart=on-failure
NoNewPrivileges=true
PrivateTmp=true
[Install]
WantedBy=multi-user.target
````

## systemd/netboot-smb.service

[源文件](../systemd/netboot-smb.service)

````ini
[Unit]
Description=NetBoot LAN-only read-only Windows installation source
After=network-online.target
Wants=network-online.target
[Service]
Type=simple
RuntimeDirectory=netboot-smb
StateDirectory=netboot-smb
CacheDirectory=netboot-smb
ExecStart=/usr/sbin/smbd --foreground --no-process-group --configfile=/home/javen03/data/netboot/config/smb.conf
Restart=on-failure
[Install]
WantedBy=multi-user.target
````

## systemd/netboot-tftp.service

[源文件](../systemd/netboot-tftp.service)

````ini
[Unit]
Description=NetBoot TFTP only (no DHCP)
After=network-online.target
Wants=network-online.target
[Service]
ExecStartPre=/usr/sbin/dnsmasq --test --conf-file=/home/javen03/data/netboot/config/dnsmasq.conf
ExecStart=/usr/sbin/dnsmasq --keep-in-foreground --user=javen03 --conf-file=/home/javen03/data/netboot/config/dnsmasq.conf
Restart=on-failure
[Install]
WantedBy=multi-user.target
````

## tftp/boot.ipxe

[源文件](../tftp/boot.ipxe)

````ipxe
#!ipxe
dhcp || exit
chain http://192.168.10.220:8088/boot.ipxe
````
