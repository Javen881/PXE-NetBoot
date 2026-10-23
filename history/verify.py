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
