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
