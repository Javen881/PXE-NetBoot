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
