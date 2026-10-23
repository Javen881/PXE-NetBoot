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
initrd ${base}/windows11/bootmgr bootmgr || goto failed
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

