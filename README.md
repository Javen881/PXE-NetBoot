# PXE-NetBoot：局域网与跨站点网络安装手册

> 更新：2026-10-23。本文区分现有部署、可执行配置步骤与尚未完成的客户端验收。
> 当前已部署基础服务并验证资源传输；不能据此认定 Windows、Ubuntu、PE、ESXi 已完整安装成功。
> 新增 Windows 11 的步骤在第 7 节。本仓库不分发 ISO、WIM、许可证或账户密码。

## 完整部署 Demo 与源码

本仓库已补齐脚本、网页、TFTP 入口、路由器规则和历史实现。

- [完整 Demo 操作步骤](docs/DEMO.md)：依赖、镜像准备、生成目录、启用服务、验证及回滚。
- [全部源码与配置的 Markdown 示例](docs/SOURCE-DEMO.md)：直接阅读完整实现。
- [资源准备脚本](scripts/prepare.py)：只写入新的空目录，不操作当前运行服务。
- [历史部署记录](history/README.md)：保留旧脚本，标明已知限制。

安装介质内的 WIM、内核与系统文件由本地 ISO 提取；第三方引导程序的获取方式、版本和哈希见 Demo。

## 1. 用途与原理

这套平台把服务器上预先准备的操作系统镜像，通过网络提供给待安装电脑。
用途包括批量装机、虚拟机安装、无光驱电脑重装以及通过 PE 维护。
系统选择和磁盘安装仍由操作者手动进行，没有无人值守清盘配置。

ISO 是光盘文件系统镜像，并不是所有系统都能直接从网络上的 ISO 启动。
启动程序可以读到 ISO，不代表启动后的操作系统还能访问那个虚拟光驱。
因此本方案按发行版使用各自的网络安装方式：

| 系统 | 第一阶段 | 第二阶段安装源 |
| --- | --- | --- |
| Windows | iPXE → wimboot → 本版本 boot.wim | WinPE 通过 SMB 访问本版本安装目录 |
| Ubuntu Desktop | iPXE → 本镜像 vmlinuz + initrd | casper 从 HTTP 获取同一个 ISO |
| PE | iPXE → wimboot → PE WIM | 可能还需要外置工具包、网卡/存储驱动 |
| ESXi | VMware 引导器 + boot.cfg | 按配置逐个读取 HTTP 模块 |

PXE 固件先通过 DHCP 获取 IP 和引导地址，再通过 TFTP 下载 iPXE。
iPXE 重新请求 DHCP，获得 HTTP 菜单；后续大文件经 HTTP 下载。
区别两次 DHCP 请求的规则可以防止无限重复加载 iPXE。

~~~mermaid
flowchart TD
  C[待安装机器：PXE 固件] --> D[本地路由器 DHCP]
  D --> T[服务器 TFTP：iPXE]
  T --> M[HTTP 启动菜单]
  M --> W[wimboot + Windows boot.wim]
  W --> S[SMB 只读安装源]
  M --> U[Ubuntu 内核 + initrd]
  U --> I[HTTP Ubuntu ISO]
  M --> E[ESXi 引导器与模块]
~~~

### 1.1 互联网安装的边界

DHCP 广播不会自动跨互联网。只做端口转发不能让远端电脑的固件找到家里的 PXE 服务。

目前采用站点 VPN：每个站点保留本地 DHCP，远端路由器告诉客户端去访问 VPN 对面的服务器。
没有站点 VPN 时，需要本地 iPXE 启动介质或本地引导服务，再访问公开 HTTPS 安装源；
Windows 的 SMB 安装方式也需要改造或走 VPN。当前未部署公网域名、HTTPS 和公网 Windows 下载流程。
不要把 SMB 445、DHCP 或 TFTP 作为公网安装入口直接映射。

## 2. 当前环境与验收边界

| 项目 | 当前值 |
| --- | --- |
| 服务器 | Ubuntu，192.168.10.220/24 |
| 网卡 | enp4s0 |
| A 路由器 | 192.168.10.1 |
| B 路由器 / B 网段 | 192.168.18.1 / 192.168.18.0/24 |
| 站点互联 | 已存在的 VPN；不由本仓库建立 |
| 服务器根目录 | /home/javen03/data/netboot |
| 镜像目录 | /home/javen03/data/netboot/iso |
| HTTP | http://192.168.10.220:8088/ |
| 原 Windows 共享 | \\192.168.10.220\netboot-win |

已验证：三个服务运行；A/B 站点可以读取启动资源；TFTP 文件校验；
HTTP 大文件与 Range 响应；A 站点 SMB 读文件；B 站点 TCP 445 连通。
客户端日志中出现过 Windows boot.wim、Ubuntu 内核/initrd 和 ESXi 模块的传输。

尚待验证：修复后的 Ubuntu 进入桌面安装器、Windows 完整安装并首次重启、
PE 外置工具、ESXi 最终安装与硬件兼容性。B 站点端口连通不等于 SMB 安装已通过。

## 3. 从零准备服务器

以下命令面向当前同名用户、路径和地址的 Ubuntu 主机。
如果换机器，先统一修改 config/、systemd/、examples/ 中的地址、网卡、用户和绝对路径。
不要直接在生产服务器重复执行安装或覆盖已有配置。

命令应逐段执行；任何命令失败都应先排除原因，不要继续执行后续段落。

### 3.1 检查地址、空间和端口

~~~bash
ip -br address
ip route
df -h /home/javen03/data
id javen03
sudo ss -lntup
~~~

确认服务器固定为 192.168.10.220，客户端能路由到它。
磁盘需要容纳原始镜像及解包内容：保留至少所有 ISO 总大小两倍的可用空间，再预留余量。
8088/TCP、69/UDP、445/TCP 如果被其他实例占用，要先决定使用哪套服务，不能直接停止不明业务。

### 3.2 安装依赖

~~~bash
sudo apt update
sudo apt install nginx dnsmasq-base samba smbclient ipxe curl   7zip libarchive-tools wimtools python3
~~~

当前环境使用 7z 命令；不同 Ubuntu 版本的 7zip 包可能提供不同命令名。
先确认下面的工具存在，再继续：

~~~bash
command -v 7z
command -v bsdtar
command -v wimlib-imagex
dpkg -L ipxe
~~~

安装包可能启动发行版默认服务。
只有确认默认 nginx/smbd/nmbd 是本次安装产生且没有其他业务使用时，才停用它们：

~~~bash
sudo systemctl disable --now nginx smbd nmbd
~~~

本环境原有 GitLab 使用的 80 端口不属于本平台。不要停止 GitLab。

### 3.3 目录与仓库

在已克隆的本仓库目录执行：

~~~bash
export NETBOOT_ROOT=/home/javen03/data/netboot
sudo install -d -o javen03 -g javen03 "$NETBOOT_ROOT"
mkdir -p "$NETBOOT_ROOT"/{iso,http/boot,http/ubuntu,http/windows,http/pe,http/esxi,tftp,config,backups}
cp config/nginx.conf config/dnsmasq.conf config/smb.conf "$NETBOOT_ROOT/config/"
~~~

上例使用 Bash 的花括号展开；后续命令也请使用 Bash。
将你自己的合法镜像放到 iso/ 中，不把安装镜像提交到 Git。
本仓库配置使用现有固定文件名；如果改变文件名，同时修改 nginx 的 Ubuntu alias。

### 3.4 iPXE 和 wimboot

~~~bash
ls /usr/lib/ipxe/
cp /usr/lib/ipxe/undionly.kpxe "$NETBOOT_ROOT/tftp/"
cp /usr/lib/ipxe/ipxe-amd64.efi "$NETBOOT_ROOT/tftp/ipxe.efi"
cp /usr/lib/ipxe/snponly.efi "$NETBOOT_ROOT/tftp/"
cp /usr/lib/ipxe/ipxe.iso "$NETBOOT_ROOT/http/boot/"
curl --fail --location --retry 3   https://github.com/ipxe/wimboot/releases/download/v2.9.0/wimboot   -o "$NETBOOT_ROOT/http/boot/wimboot"
cp examples/boot.ipxe "$NETBOOT_ROOT/http/boot.ipxe"
cat > "$NETBOOT_ROOT/tftp/boot.ipxe" <<'EOF'
#!ipxe
dhcp || exit
chain http://192.168.10.220:8088/boot.ipxe
EOF
~~~

若发行版的 iPXE 文件路径不同，以 dpkg -L ipxe 为准，不能把不存在的文件当作已复制成功。
Secure Boot 的信任链尚未验收；需要 Secure Boot 的环境必须另外验证签名引导链。

## 4. 服务配置与启动

config/nginx.conf：只监听 192.168.10.220:8088，HTTP 根为 http/，
把 /iso/ubuntu.iso 映射到本地 Ubuntu ISO。
config/dnsmasq.conf：仅启用 TFTP，没有 DHCP 地址池，不能与路由器争抢 DHCP。
config/smb.conf：提供局域网/VPN 只读安装源，允许 A/B 网段。

### 4.1 安装 systemd 单元

~~~bash
sudo install -m 644 systemd/netboot-*.service /etc/systemd/system/
sudo install -d /run/netboot-http /run/netboot-smb   /var/lib/netboot-smb /var/cache/netboot-smb /var/log/netboot
sudo chown javen03:javen03 /run/netboot-http /var/log/netboot
sudo nginx -t -c "$NETBOOT_ROOT/config/nginx.conf"
sudo dnsmasq --test --conf-file="$NETBOOT_ROOT/config/dnsmasq.conf"
sudo testparm -s "$NETBOOT_ROOT/config/smb.conf"
sudo systemctl daemon-reload
sudo systemctl enable --now netboot-http netboot-tftp netboot-smb
systemctl is-active netboot-http netboot-tftp netboot-smb
~~~

预期三个 active。配置检查失败时先修复，不能忽略错误继续启用。

### 4.2 网络与权限

允许可信 A/B 网段到服务器的 TCP 8088、TCP 445、UDP 69 和 UDP 40000–40031。
UDP 40000–40031 是 TFTP 后续传输端口，仅放行 69 可能仍无法下载。
VPN 必须同时具备正向及返回路由；NAT 会改变 SMB 实际看到的来源地址。

~~~bash
namei -l "$NETBOOT_ROOT/http/windows"
sudo ss -lntup
curl -f http://192.168.10.220:8088/boot.ipxe
~~~

父目录必须允许服务用户遍历。不要用 chmod -R 777 解决问题。
普通资源通常为 644、目录为 755；Windows 可执行映像需保留执行权限，详见第 7 节。

## 5. 路由器 PXE 配置

A/B 路由器均在其本地 LAN 发放 DHCP。下面规则添加在 OpenWrt 的 /etc/dnsmasq.conf。
已有平台目前使用这些规则；不要重复追加。先备份并检查是否已经存在。

~~~sh
cp -a /etc/dnsmasq.conf /root/dnsmasq.conf.before-netboot
cat >> /etc/dnsmasq.conf <<'EOF'
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
EOF
dnsmasq --test
/etc/init.d/dnsmasq restart
logread -e dnsmasq
~~~

在 OpenWrt 上还要检查实际生成的 /var/etc/dnsmasq.conf.* 是否包含上述配置文件，
并对实际启动使用的配置执行 dnsmasq --test -C 配置路径。
多 DHCP 实例的设备必须确认规则加载到提供客户端地址的实例。
重启会短暂影响 DHCP/DNS；先备份，再校验，最后重启。
旧 openwrt-pxe.conf.example 中的 TEST_MAC 白名单示例不代表现有实际规则。

预期：Legacy BIOS 获取 undionly.kpxe；x64 UEFI 获取 ipxe.efi；
已运行 iPXE 的客户端获取 HTTP 菜单，而不是再次获取 iPXE 二进制。

回滚：

~~~sh
cp -a /root/dnsmasq.conf.before-netboot /etc/dnsmasq.conf
dnsmasq --test
/etc/init.d/dnsmasq restart
~~~

恢复备份前确认备份时间，避免覆盖后续新增的其他 DHCP 配置。

## 6. 接入已有系统

### 6.1 Ubuntu Desktop

~~~bash
bsdtar -xOf "$NETBOOT_ROOT/iso/ubuntu-26.04.1-desktop-amd64.iso"   casper/vmlinuz > "$NETBOOT_ROOT/http/ubuntu/vmlinuz"
bsdtar -xOf "$NETBOOT_ROOT/iso/ubuntu-26.04.1-desktop-amd64.iso"   casper/initrd > "$NETBOOT_ROOT/http/ubuntu/initrd"
test -s "$NETBOOT_ROOT/http/ubuntu/vmlinuz"
test -s "$NETBOOT_ROOT/http/ubuntu/initrd"
~~~

对应菜单：

~~~ipxe
:ubuntu
kernel ${base}/ubuntu/vmlinuz initrd=initrd.magic boot=casper ip=dhcp url=${base}/iso/ubuntu.iso --- || goto failed
initrd --name initrd ${base}/ubuntu/initrd || goto failed
boot || goto failed
~~~

内核、initrd、ISO 必须来自同一版本。不要将 Windows wimboot 的“目标文件名”写法照搬到 Linux initrd。
当前 ISO 大约 6 GiB；测试虚拟机建议先分配 16 GiB RAM，减少内存因素干扰。
这是测试配置建议，并非所有 Ubuntu 版本的统一最低要求。

### 6.2 Windows LTSC 2021

~~~bash
7z t "$NETBOOT_ROOT/iso/SW_DVD9_WIN_ENT_LTSC_2021_64BIT_ChnSimp_MLF_X22-84402.ISO"
7z x -y "-o$NETBOOT_ROOT/http/windows"   "$NETBOOT_ROOT/iso/SW_DVD9_WIN_ENT_LTSC_2021_64BIT_ChnSimp_MLF_X22-84402.ISO"
cp examples/netboot.cmd examples/winpeshl.ini "$NETBOOT_ROOT/http/windows/"
find "$NETBOOT_ROOT/http/windows" -type d -exec chmod 755 {} +
find "$NETBOOT_ROOT/http/windows" -type f -exec chmod 644 {} +
find "$NETBOOT_ROOT/http/windows" -type f   \( -iname '*.exe' -o -iname '*.dll' -o -iname '*.ocx' -o -iname '*.cpl' \) -exec chmod 755 {} +
wimlib-imagex info "$NETBOOT_ROOT/http/windows/sources/boot.wim"
~~~

原镜像的 Windows Setup 在索引 2，菜单明确使用 wimboot index=2。
不要假定所有自定义镜像的安装环境都在索引 2，应以 WIM 信息为准。

### 6.3 PE

~~~bash
bsdtar -xOf "$NETBOOT_ROOT/iso/Full.iso" modules/PE10.WIM > "$NETBOOT_ROOT/http/pe/pe10.wim"
bsdtar -xOf "$NETBOOT_ROOT/iso/Full.iso" modules/PE8.WIM > "$NETBOOT_ROOT/http/pe/pe8.wim"
bsdtar -xOf "$NETBOOT_ROOT/iso/Full.iso" boot/BOOT.SDI > "$NETBOOT_ROOT/http/pe/boot.sdi"
~~~

当前 PE10 为旧 x64 Win10 build 10586，PE8 为 x86 Win8 build 9200。
PE8 菜单限制 Legacy BIOS。PE 能进入桌面不等于支持安装 Windows 11：
需要匹配的网卡/存储驱动、正确位数、安装工具和完整安装源。
Full.iso 内其他 Linux 工具尚未适配，不能保证提取 WIM 后外置工具自动可用。

### 6.4 ESXi

~~~bash
bsdtar -xf "$NETBOOT_ROOT/iso/DellEMC-ESXi-6.7U3-17700523-A11-customized.iso"   -C "$NETBOOT_ROOT/http/esxi" --no-same-owner
~~~

解包后将目录与文件名统一成小写，避免 Linux 文件名大小写导致 404。
boot.cfg 增加 prefix=http://192.168.10.220:8088/esxi，
并去掉 kernel 与各 modules 项开头的斜线，保留原始模块顺序和参数。
执行下面脚本完成当前 ESXi 目录的路径适配；它会先备份 boot.cfg，遇到大小写目标冲突则停止：

~~~bash
python3 - <<'PY'
from pathlib import Path
import shutil
root = Path("/home/javen03/data/netboot/http/esxi")
for path in sorted(root.rglob("*"), key=lambda p: len(p.parts), reverse=True):
    target = path.with_name(path.name.lower())
    if target != path:
        if target.exists():
            raise SystemExit(f"Name collision: {target}")
        path.rename(target)
cfg = root / "boot.cfg"
backup = root / "boot.cfg.original"
if not backup.exists():
    shutil.copy2(cfg, backup)
lines = []
for line in cfg.read_text().splitlines():
    if line.startswith("prefix="):
        continue
    if line.startswith("kernel="):
        line = "kernel=" + line[len("kernel="):].lstrip("/")
    elif line.startswith("modules="):
        line = "modules=" + " --- ".join(
            item.strip().lstrip("/") for item in line[len("modules="):].split(" --- ")
        )
    lines.append(line)
cfg.write_text("prefix=http://192.168.10.220:8088/esxi\n" + "\n".join(lines) + "\n")
PY
~~~

不同 ESXi 版本不能混用模块。当前菜单包含 BIOS/UEFI 两种引导入口；
硬件、网卡、存储驱动与许可证仍需独立确认。

## 7. 新增 Windows 11：完整操作案例

### 7.1 当前发现与接入原则

当前新增文件为 Win11_24H2_Pro_Chinese_Simplified_x64.iso。
2026-10-23 检查时文件大小持续增长，7-Zip 报 Headers Error / Unexpected end of archive；
当时属于传输中的不完整状态。用户确认传输结束后复检，7z t 已返回 Everything is Ok。
最终文件大小为 5,606,453,248 字节；boot.wim 为 604,512,985 字节；install.wim 为 4,726,588,024 字节。
已读取 boot.wim 元数据：x86_64，build 26100.1742，索引 1 为 Windows PE，索引 2 为 Windows Setup。
因此本镜像菜单使用 index=2，安装源使用 install.wim。
本地 SHA256：84d9b60c5cf7bbfefe56c530c373ceb35a593750fb8ff0f79345f110e2d32668。
此哈希尚未与发行方校验值核对。
文件名不能证明镜像来源、版本或完整性。

Windows 11 使用独立目录 windows11、共享 netboot-win11、菜单 win11。
保留 LTSC 原目录和入口，不覆盖它的 boot.wim 或 install.wim。
当前已完成 Windows 11 安装源、共享、启动脚本和菜单接入。客户端已进入过安装程序；尚未完成整机安装验收。已有环境不要重复添加菜单和共享。

### 7.2 等待传输结束并检查镜像

~~~bash
export NETBOOT_ROOT=/home/javen03/data/netboot
export WIN11_ISO="$NETBOOT_ROOT/iso/Win11_24H2_Pro_Chinese_Simplified_x64.iso"
stat -c '%s %y %n' "$WIN11_ISO"
7z t "$WIN11_ISO"
sha256sum "$WIN11_ISO"
~~~

确认传输程序已完成，7z t 退出码为 0，并与发布方提供的 SHA256 比较。
仅计算哈希没有可信参考值时，只能记录文件指纹，不能证明来源可信。

### 7.3 备份和解包

~~~bash
export CHANGE_BACKUP="$NETBOOT_ROOT/backups/win11-$(date +%Y%m%d-%H%M%S)"
mkdir -p "$CHANGE_BACKUP"
cp "$NETBOOT_ROOT/http/boot.ipxe" "$CHANGE_BACKUP/"
cp "$NETBOOT_ROOT/config/smb.conf" "$CHANGE_BACKUP/"
test ! -e "$NETBOOT_ROOT/http/windows11"
~~~

最后一条若返回非零，说明目录已存在，先核对旧内容，不能盲目覆盖。

~~~bash
mkdir "$NETBOOT_ROOT/http/windows11"
7z x "-o$NETBOOT_ROOT/http/windows11" "$WIN11_ISO"
test -s "$NETBOOT_ROOT/http/windows11/sources/boot.wim"
ls -lh "$NETBOOT_ROOT/http/windows11/sources"/install.*
wimlib-imagex info "$NETBOOT_ROOT/http/windows11/sources/boot.wim"
~~~

记录 Windows Setup 对应的索引。如果没有 Windows Setup 映像，不直接套用 index=2。
install.wim、install.esd、install.swm 属于不同形式：
下文脚本支持 WIM 和 ESD；分卷 SWM 应保留所有分卷并使用对应版本 Setup 的安装介质流程，
不要只把 install.swm 改名成 install.wim。

### 7.4 新增独立共享

当前配置快照已含 [netboot-win11]，不要重复添加；仅旧配置缺少时追加：

~~~ini
[netboot-win11]
path = /home/javen03/data/netboot/http/windows11
read only = yes
guest ok = yes
guest only = yes
hosts allow = 127.0.0.1 192.168.10.0/24 192.168.18.0/24
hosts deny = ALL
~~~

沿用当前受信 LAN/VPN 的 guest 共享方案。如果 WinPE 客户端策略拒绝 guest，
需要配置专用只读账号和相应客户端连接方式，不要把密码写到公开仓库。

### 7.5 安装自动连接与重试脚本

在本仓库根目录执行：

~~~bash
cp examples/windows11/netboot.cmd examples/windows11/winpeshl.ini "$NETBOOT_ROOT/http/windows11/"
~~~

脚本见 [netboot.cmd](examples/windows11/netboot.cmd)，启动入口见 [winpeshl.ini](examples/windows11/winpeshl.ini)。

1. 执行 wpeinit 初始化网络。
2. 自动映射 Z: 到 Windows 11 安装共享。
3. 映射失败或 install.wim 不可读时，等待约 5 秒再试，最多尝试 12 次。
4. 若 Z: 已连接同一个共享，使用现有映射检查安装源。
5. 安装源可读后，自动运行本机 WinPE 的 Setup，使用共享中的 install.wim。
6. 重试上限耗尽后显示 IP、共享状态并保留命令行。

总等待时间还包含每次 net use 的连接超时，并非固定 60 秒。
当前脚本针对 install.wim；使用只有 install.esd 的其他镜像时，
必须同时修改脚本中的存在性检查和 /installfrom 路径。分卷 SWM 不适用此脚本。

脚本自动连接安装源，不会自动选择磁盘、分区或格式化。
重新 PXE 启动才能加载更新后的脚本；已运行的 WinPE 使用内存中的旧版本。
已核对 HTTP 返回内容和 CRLF 格式，客户端自动重试后的完整安装仍待验收。

### 7.6 权限与配置检查

~~~bash
find "$NETBOOT_ROOT/http/windows11" -type d -exec chmod 755 {} +
find "$NETBOOT_ROOT/http/windows11" -type f -exec chmod 644 {} +
find "$NETBOOT_ROOT/http/windows11" -type f   \( -iname '*.exe' -o -iname '*.dll' -o -iname '*.ocx' -o -iname '*.cpl' \) -exec chmod 755 {} +
sudo testparm -s "$NETBOOT_ROOT/config/smb.conf"
sudo systemctl kill --kill-whom=main --signal=HUP netboot-smb
smbclient //192.168.10.220/netboot-win11 -U 'netboot-guest%'   -c 'ls; cd sources; ls install.*'
~~~

预期能列出本次 Windows 11 安装源。服务单元没有 ExecReload，
所以不要使用 systemctl reload netboot-smb；这里发送 HUP 重新加载配置。

### 7.7 修改 iPXE 菜单

当前菜单快照已包含此入口；仅迁移到旧菜单时，在 choose 之前添加：

~~~ipxe
item win11 Windows 11 24H2 x64
~~~

在 :shell 之前添加以下标签块。索引 2 必须已通过 7.3 确认，否则替换为实际索引：

~~~ipxe
:win11
kernel ${base}/boot/wimboot index=2 || goto failed
initrd ${base}/windows11/bootmgr bootmgr || goto failed
initrd ${base}/windows11/winpeshl.ini winpeshl.ini || goto failed
initrd ${base}/windows11/netboot.cmd netboot.cmd || goto failed
initrd ${base}/windows11/sources/boot.wim boot.wim || goto failed
boot || goto failed
~~~

item 后的 win11 必须与 :win11 完全一致。
这里只修改静态菜单，无需重启 nginx、TFTP 或路由器。
如果同时修改了首页 index.html，可更新显示列表，但首页不是实际引导菜单。

### 7.8 服务器与客户端验收

~~~bash
curl -fI http://192.168.10.220:8088/windows11/sources/boot.wim
curl -f http://192.168.10.220:8088/windows11/netboot.cmd
curl -f http://192.168.10.220:8088/boot.ipxe
sudo tail -f /var/log/netboot/access.log
~~~

使用 UEFI x64 测试虚拟机，网卡桥接到有 PXE DHCP 的 LAN；
为 Windows 11 配置符合所选版本要求的 CPU、内存、磁盘、TPM 和固件。
本项目不包含绕过 Windows 11 硬件检查。
若开启 Secure Boot 后 iPXE 被拒绝，应先解决签名引导链；不能认定镜像故障。

按顺序记录：
1. 获得正确 IP、网关与引导文件。
2. 进入菜单并看到新增 Windows 11。
3. boot.wim 下载完成，WinPE 网卡取得地址。
4. Z: 映射成功，显示本次 Windows 11 安装源。
5. Setup 显示正确版本并识别目标测试磁盘。
6. 人工确认目标磁盘后安装；首次重启从硬盘启动。
7. 完成首次设置并核对系统版本。

步骤 1–4 通过不能替代步骤 5–7。
如果出错，在 WinPE 执行 ipconfig、net use、dir Z:\sources 并保留 Panther 日志。

## 8. 新增其他类型系统时改哪里

| 新增类型 | 准备资源 | 需要修改 | 常见陷阱 |
| --- | --- | --- | --- |
| Windows / Server | 独立完整解包目录、对应 boot.wim | 新共享、启动脚本、item 与标签块 | 混用不同版本 Setup；索引错误 |
| Ubuntu Desktop | 同版本 vmlinuz、initrd、ISO | 新 alias、独立菜单参数 | 内核与 ISO 不匹配 |
| Ubuntu Server | 发行版提供的内核/initrd与安装介质 | 根据该版本安装器文档配置参数 | 不应直接套 Desktop casper 参数 |
| Debian | 官方 netboot 内核/initrd、软件源 | 独立菜单和安装源参数 | ISO 与 netboot 不是同一流程 |
| Rocky / Alma / RHEL 类 | pxeboot/vmlinuz、initrd.img、完整安装树 | inst.repo / inst.stage2 等对应版本参数 | 缺少隐藏 .treeinfo、stage2 |
| ESXi | 完整模块目录、boot.cfg | prefix、模块路径、独立标签 | 模块顺序/大小写/版本混用 |
| 自定义 PE | WIM、依赖启动文件、外置工具 | 独立 wimboot 入口 | PE 过旧、缺网卡/磁盘驱动 |

此表是分流依据，不是所有发行版通用的可复制命令。
每次新增都采用独立目录，不覆盖已验证版本。
新增原始 ISO 不会自动识别、解包或更新菜单；本平台没有自动镜像导入器。

## 9. 已遇到的故障与定位步骤

### 9.1 Windows 连接共享成功后“拒绝访问”

曾经将 Windows 所有文件设成 644。共享能读取，不代表 Samba 允许以执行访问方式打开 EXE/DLL。
修复可执行文件权限，并优先使用 boot.wim 中 X:\sources\setup.exe，
通过 /InstallFrom 读取共享中的系统映像。不要通过把共享改为可写来解决执行问题。

### 9.2 Ubuntu VFS: Unable to mount root fs on unknown-block(0,0)

这说明内核启动后无法得到可用根文件系统，不能仅凭截图确定物理磁盘损坏。
本次发现 initrd 下载命令错误：尾部目标文件名导致 Linux initrd 被按注入文件方式处理。
已调整为 initrd --name initrd URL，并保留内核参数 initrd=initrd.magic。
原 initrd 已核对与 ISO 内文件一致。修复后仍需客户端重新启动验收。
如果继续失败，依次检查内核参数、initrd 内容、HTTP ISO 地址、内存和网卡驱动。

### 9.3 TFTP 超时

先检查绑定网卡和监听地址，再检查 UDP 后续端口与 VPN 回程。
曾出现只有 loopback 可用的配置，当前使用 enable-tftp=enp4s0。
用完整下载后的 SHA256 验证，不能只看请求发出或收到一个 UDP 数据包。

### 9.4 A 站点可用，B 站点不行

分别验证 B 客户端到 HTTP、TFTP、SMB 的路由。
路由器自己发包可能使用 VPN 隧道地址，不等于客户端真实来源地址。
共享 allow 列表目前允许 192.168.18.0/24；如 NAT 后来源不同，应根据实际路由调整。
不要为排障直接允许整个互联网。

### 9.5 RDP 身份验证错误

RDP 登录是系统安装后另一条链路；它不能证明 PXE DHCP、TFTP 或安装源是否可用。
按账户状态、密码、NLA、系统日志排查，避免通过改 PXE 配置解决远程桌面登录。

### 9.6 错误 53 与自动重试

用户反馈手动连接可用后，已为 Windows 11 加入自动重试。
这用于处理临时连接失败，根因尚未通过客户端日志最终确定。
若仍失败，需要核查 WinPE 到服务器的 TCP 445 连通性和 SMB 认证策略；
正常 Windows 的连接测试不能代替 WinPE 环境测试。
重试不会修复永久路由阻断、缺失驱动或访客认证策略冲突。

## 10. 日常维护、升级与回滚

~~~bash
systemctl status netboot-http netboot-tftp netboot-smb --no-pager
journalctl -u netboot-tftp -n 80 --no-pager
journalctl -u netboot-smb -n 80 --no-pager
sudo tail -n 80 /var/log/netboot/error.log
sudo tail -n 80 /var/log/netboot/access.log
~~~

修改 nginx 配置后先 nginx -t，再对 netboot-http 主进程发送 HUP。
修改 TFTP 配置先 dnsmasq --test，再重启 netboot-tftp。
修改 SMB 配置先 testparm，再 HUP。
单纯改 HTTP 静态文件不需要重启服务。

Windows 11 接入回滚：用第 7 节备份还原 boot.ipxe 和 smb.conf，检查 Samba 配置后 HUP。
保留 windows11/ 用于排障，确认无引用后再手工决定是否删除。

路由器正确回滚命令：

~~~sh
cp -a /root/dnsmasq.conf.before-netboot /etc/dnsmasq.conf
dnsmasq --test
/etc/init.d/dnsmasq restart
~~~

停止平台：

~~~bash
sudo systemctl stop netboot-http netboot-tftp netboot-smb
~~~

停止服务不会删除镜像。长期退出部署还需要移除路由器 PXE 规则。
不要重新运行旧 deploy.py 来更新一个系统：它包含固定路径及整体配置覆盖逻辑，
并不等同于已合并所有后续修复的幂等安装器。

## 11. 参考与仓库内容

- [iPXE wimboot](https://ipxe.org/wimboot)：Windows 内存启动方法。
- [iPXE imgfetch / initrd](https://ipxe.org/cmd/imgfetch)：下载与文件注入语义。
- [iPXE WinPE](https://ipxe.org/howto/winpe)：WinPE 网络启动说明。
- [Windows Setup 命令行](https://learn.microsoft.com/windows-hardware/manufacture/desktop/windows-setup-command-line-options?view=windows-11)：/InstallFrom。
- [dnsmasq 手册](https://dnsmasq.org/docs/dnsmasq-man.html)：DHCP 标签、引导与 TFTP。

config/ 和 systemd/ 是现有环境配置快照；
examples/ 是现有菜单与 Windows 启动脚本。
当前菜单已包含 Windows 11；examples/windows11/ 是已部署的自动重试脚本。
原 examples/netboot.cmd 对应 LTSC，不要混用。
本仓库只保存文本配置和说明，不保存安装镜像、账号密码或激活密钥。

