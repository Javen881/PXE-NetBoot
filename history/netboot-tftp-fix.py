from pathlib import Path
import subprocess
p=Path('/home/javen03/data/netboot/config/dnsmasq.conf')
p.write_text(p.read_text().replace('no-dhcp-interface=enp4s0\n','').replace('enable-tftp\n','enable-tftp=enp4s0\n'))
subprocess.run(['dnsmasq','--test','--conf-file='+str(p)],check=True)
subprocess.run(['systemctl','restart','netboot-tftp'],check=True)
