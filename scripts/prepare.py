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
    for member,dst in [
        ('modules/PE10.WIM','pe10.wim'),
        ('modules/PE8.WIM','pe8.wim'),
        ('boot/BOOT.SDI','boot.sdi'),
        ('bootmgr','bootmgr'),
        ('boot/BCD','bcd'),
        ('efi/boot/msbootx64.efi','bootmgfw.efi'),
        ('efi/microsoft/boot/bcd','bcd-efi'),
    ]:
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
