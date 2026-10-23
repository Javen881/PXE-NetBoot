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
