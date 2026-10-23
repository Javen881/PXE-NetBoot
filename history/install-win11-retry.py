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

