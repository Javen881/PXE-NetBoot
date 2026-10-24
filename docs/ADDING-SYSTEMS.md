# 新增 Windows 10、Windows 11 与 Linux 镜像：逐步操作指南

适用于本项目已有 PXE 服务的环境。本文是扩容指南，不需要重新安装 DHCP、TFTP 或覆盖现有系统目录。
更新日期：2026-10-24。当前用户已确认 PE10 的 UEFI 入口可用；下文新增版本示例需要逐一完成客户端安装验收。

## 1. 原理：为什么复制 ISO 后菜单不会自动增加

启动顺序为：客户端获取 DHCP 参数 → BIOS/UEFI 下载对应 iPXE → HTTP 获取 boot.ipxe → 下载本系统的启动环境 → 安装程序读取安装源。

| 类型 | 首次启动资源 | 后续安装源 |
| --- | --- | --- |
| Windows 10/11 | 同一 ISO 的 Setup boot.wim，通过 wimboot 启动 | SMB 共享中的完整解包目录 |
| Ubuntu live/server | 同一 ISO 的 casper/vmlinuz 和 initrd | HTTP 提供完整 ISO |
| Debian Installer | 官方 netboot linux 和 initrd.gz | Debian 软件源，或自行准备的内网镜像站 |
| Rocky/AlmaLinux/RHEL 类 | DVD 中 images/pxeboot 内核与 initrd | HTTP 解包安装树，包含 .treeinfo、stage2 和软件仓库 |

ISO 只是原材料。本项目菜单不是自动扫描器，新增 ISO 后还要准备启动文件、安装源和菜单入口。
新增操作系统通常不修改路由器或 Windows DHCP GUI；这些仍指向原来的 iPXE 和 boot.ipxe。

下面以 x86_64/amd64 为例。ARM64、32 位 UEFI、Secure Boot 签名链不能直接套用 x64 示例。Windows 11 使用满足该版本要求的 UEFI/TPM/CPU/内存配置。

## 2. 路径、权限与备份

所有 bash 命令在 **Linux PXE 服务器** 的普通终端运行，需要写运行目录时使用 sudo。不要在 Windows PowerShell 中粘贴 bash 命令。
先执行公共变量，后续命令在同一终端执行：

```bash
export NETBOOT_ROOT=/home/javen03/data/netboot
export PXE_REPO=/home/javen03/data/PXE
export PXE_BASE=http://192.168.10.220:8088
export PXE_BACKUP="$NETBOOT_ROOT/backups/add-system-$(date +%Y%m%d-%H%M%S)"
sudo mkdir -p "$PXE_BACKUP"
sudo cp "$NETBOOT_ROOT/http/boot.ipxe" "$PXE_BACKUP/boot.ipxe"
sudo cp "$NETBOOT_ROOT/config/smb.conf" "$PXE_BACKUP/smb.conf"
command -v 7z bsdtar wimlib-imagex curl smbclient testparm
```

某个命令不存在时，Ubuntu/Debian 服务器可安装：

```bash
sudo apt-get update
sudo apt-get install 7zip libarchive-tools wimtools curl smbclient samba
```

检查存储空间：`df -h "$NETBOOT_ROOT"`。Windows 解包与 ISO 同时保留至少要预留两份介质空间，转换 ESD 时还需要额外空间。

运行目录与仓库是两份不同的数据：

- 生效菜单：`/home/javen03/data/netboot/http/boot.ipxe`。
- 仓库示例：`/home/javen03/data/PXE/examples/boot.ipxe`。
- 仅修改 GitHub 或仓库示例，不会自动部署到运行服务。
- 不要运行 `prepare.py` 覆盖现有目录；它用于创建全新的空环境。

## 3. 新增另一个 Windows 11 或 Windows 10

### 3.1 每个版本使用独立标识

下面完整演示 `win11-extra`。加入另一套 Windows 10 时，改成 `win10-extra` 并选对应 ISO，重复整章即可。
不要把新 boot.wim 覆盖到旧 Windows 目录。

| 项目 | Windows 11 示例 | Windows 10 示例 |
| --- | --- | --- |
| 唯一标识 ID | win11-extra | win10-extra |
| HTTP 目录 | http/win11-extra | http/win10-extra |
| SMB 共享 | netboot-win11-extra | netboot-win10-extra |
| 菜单标签 | win11-extra | win10-extra |

```bash
export SYS_ID=win11-extra
export SYS_ISO="$NETBOOT_ROOT/iso/你的Windows11镜像.iso"
export SYS_DIR="$NETBOOT_ROOT/http/$SYS_ID"
export SYS_SHARE="netboot-$SYS_ID"
```

将 ISO 名替换成真实文件名。ID 仅用英文字母、数字和短横线，不用空格。

### 3.2 检查和解包

```bash
test -s "$SYS_ISO" || { echo 'ISO 不存在或为空'; exit 1; }
7z t "$SYS_ISO" || exit 1
sha256sum "$SYS_ISO"
test ! -e "$SYS_DIR" || { echo '目录已存在，请换一个 SYS_ID'; exit 1; }
sudo mkdir -p "$SYS_DIR"
sudo 7z x -y "-o$SYS_DIR" "$SYS_ISO" || exit 1
ls -lh "$SYS_DIR/sources/boot.wim" "$SYS_DIR/sources"/install.*
wimlib-imagex info "$SYS_DIR/sources/boot.wim"
```

等待上传结束后再执行；把 SHA256 与发布方提供的值比较。Linux 文件名大小写敏感，解包路径若不是 sources/boot.wim，应核对实际目录后调整所有引用。

找出 **Windows Setup 对应的索引**，常见为 2，但不能靠猜；Windows PE 基础索引不一定包含 Setup。下面先以已确认的 2 为例：

```bash
export SETUP_INDEX=2
wimlib-imagex dir "$SYS_DIR/sources/boot.wim" "$SETUP_INDEX" --path=/sources/setup.exe
```

此文件必须存在。不要将 install.wim 的版本选择索引与 boot.wim 的 Setup 索引混淆。

### 3.3 install.wim、install.esd、install.swm 的处理

下面自动启动脚本统一使用 install.wim：

- 有 `install.wim`：直接继续。
- 只有 `install.esd`：可导出全部映像为 WIM，再用本章脚本。源 ESD 保留，不改名。
- 只有分卷 `install.swm`：必须保留全部分卷，可先合并为 WIM；只有首个分卷不完整。

ESD 转 WIM（仅在 install.wim 不存在时执行）：

```bash
test ! -e "$SYS_DIR/sources/install.wim" || exit 1
sudo wimlib-imagex export "$SYS_DIR/sources/install.esd" all \
  "$SYS_DIR/sources/install.wim" --compress=LZX
wimlib-imagex info "$SYS_DIR/sources/install.wim"
```

SWM 合并（仅在 install.wim 不存在时执行）：

```bash
test ! -e "$SYS_DIR/sources/install.wim" || exit 1
sudo wimlib-imagex export "$SYS_DIR/sources/install.swm" all \
  "$SYS_DIR/sources/install.wim" --ref="$SYS_DIR/sources/install*.swm" --compress=LZX
wimlib-imagex info "$SYS_DIR/sources/install.wim"
```

转换命令失败时停止，不把不完整的目标文件当安装源。微软 Setup 也支持通过 /InstallFrom 指定第一份 SWM，但本文使用合并方式保持存在性检查与安装命令一致。

### 3.4 创建自动网络连接脚本

复用本项目已经使用的重试脚本，只修改共享名与提示，生成 CRLF 文件：

```bash
export SYS_DIR SYS_SHARE PXE_REPO
sudo --preserve-env=SYS_DIR,SYS_SHARE,PXE_REPO python3 - <<'PY'
import os
from pathlib import Path
src = Path(os.environ['PXE_REPO']) / 'examples/windows11'
dst = Path(os.environ['SYS_DIR'])
s = (src / 'netboot.cmd').read_text()
s = s.replace('netboot-win11', os.environ['SYS_SHARE'])
s = s.replace('Windows 11', 'Windows')
for name, text in [('netboot.cmd', s), ('winpeshl.ini', (src/'winpeshl.ini').read_text())]:
    (dst/name).write_bytes(text.replace('\r\n', '\n').replace('\n', '\r\n').encode('ascii'))
PY
```

这份脚本执行 wpeinit → 自动连接共享（最多 12 次）→ 运行 `X:\sources\setup.exe /installfrom:Z:\sources\install.wim`。
它不会自动选择或格式化磁盘。地址仍是 192.168.10.220；服务器地址改变时，还需修改脚本中的 UNC 地址。
脚本有变更必须重新 PXE 启动，当前 WinPE 内存里的旧脚本不会自动更新。

### 3.5 文件权限与 SMB 共享

```bash
sudo find "$SYS_DIR" -type d -exec chmod 755 {} +
sudo find "$SYS_DIR" -type f -exec chmod 644 {} +
sudo find "$SYS_DIR" -type f \( -iname '*.exe' -o -iname '*.dll' -o -iname '*.ocx' -o -iname '*.cpl' \) -exec chmod 755 {} +
sudo nano "$NETBOOT_ROOT/config/smb.conf"
```

在文件末尾添加一次以下段落。若使用 win10-extra，要同时替换段名和 path。不要重复创建同名共享。

```ini
[netboot-win11-extra]
path = /home/javen03/data/netboot/http/win11-extra
read only = yes
guest ok = yes
guest only = yes
hosts allow = 127.0.0.1 192.168.10.0/24 192.168.18.0/24 192.168.50.0/24
hosts deny = ALL
```

这里沿用本环境的受信 LAN/VPN guest 方案。允许网段按 Samba 实际看到的客户端源 IP 填写；现场 50 网段不要遗漏。
若其他版本 WinPE 拒绝 guest，改用专用只读 SMB 账号并修改 net use 的认证参数，不能仅靠修改 hosts allow 解决认证错误。账户密码不要上传 GitHub。

```bash
sudo testparm -s "$NETBOOT_ROOT/config/smb.conf" || exit 1
sudo systemctl kill --kill-whom=main --signal=HUP netboot-smb
smbclient "//192.168.10.220/$SYS_SHARE" -U 'netboot-guest%' -c 'cd sources; ls install.wim'
```

当前服务没有 ExecReload，使用上面的 HUP。若 testparm 失败先修正语法，不继续发布。

### 3.6 菜单入口：两处都要添加

```bash
sudo nano "$NETBOOT_ROOT/http/boot.ipxe"
```

第一处：在 `choose` **之前**的 item 列表内增加：

```ipxe
item win11-extra Windows 11 - additional version
```

第二处：在 `:shell` 之前增加下面的标签块。示例 index=2 必须替换为 3.2 检查到的 Setup 索引。标签名、目录名必须一致。

```ipxe
:win11-extra
kernel ${base}/boot/wimboot index=2 || goto failed
initrd --name winpeshl.ini ${base}/win11-extra/winpeshl.ini winpeshl.ini || goto failed
initrd --name netboot.cmd ${base}/win11-extra/netboot.cmd netboot.cmd || goto failed
initrd --name boot.wim ${base}/win11-extra/sources/boot.wim boot.wim || goto failed
boot || goto failed
```

这是标准 Windows 安装介质的 wimboot 方式：从 boot.wim 自动提取引导管理器和 BCD，减少手工混搭。
若定制或精简 boot.wim 删除了 Windows/Boot 组件，这个模板不保证可用，应先核对 WIM 内容或使用完整官方安装介质。
不要照搬维护 PE 的 BCD：原 PE 光盘可能引用 modules/PE10.WIM，与 Windows 安装目录不同。
Windows 10 只需把两处标签/目录 `win11-extra` 换为 `win10-extra`，显示文字改成 Windows 10。

新增的 Windows 11 入口按 UEFI x64 验收；菜单能下载成功不代表满足 Windows 11 硬件要求。

### 3.7 验证新增 Windows

```bash
for rel in sources/boot.wim sources/install.wim netboot.cmd winpeshl.ini; do
  curl -fI "$PXE_BASE/$SYS_ID/$rel" || break
done
curl -f "$PXE_BASE/$SYS_ID/netboot.cmd"
sudo tail -f /var/log/netboot/access.log
```

客户端验收顺序：新菜单可见 → 下载新目录的 boot.wim → WinPE 获得 IP → 共享映射成功 → Setup 版本正确 → 安装到测试磁盘 → 重启从磁盘启动。
失败时 Shift+F10 打开 WinPE 命令行：

```bat
ipconfig
net use
net use Z: \\192.168.10.220\netboot-win11-extra /user:netboot-guest ""
dir Z:\sources\install.wim
X:\Windows\System32\netboot.cmd
```

如果 Z: 已占用，先确认映射对象再删除错误映射；不要盲目运行 net use * /delete。

## 4. 新增 Ubuntu Server / Desktop

### 4.1 准备同版本内核、initrd、完整 ISO

以下示例为 Ubuntu Server 24.04 系列的 live-server amd64 ISO。不同版本先查看 ISO 中 casper 的实际文件名：

```bash
export LINUX_ID=ubuntu-server-extra
export LINUX_ISO="$NETBOOT_ROOT/iso/你的Ubuntu-live-server-amd64.iso"
export LINUX_DIR="$NETBOOT_ROOT/http/$LINUX_ID"
7z t "$LINUX_ISO" || exit 1
bsdtar -tf "$LINUX_ISO" | grep '^casper/'
test ! -e "$LINUX_DIR" || exit 1
sudo mkdir -p "$LINUX_DIR"
bsdtar -xOf "$LINUX_ISO" casper/vmlinuz | sudo tee "$LINUX_DIR/vmlinuz" >/dev/null
bsdtar -xOf "$LINUX_ISO" casper/initrd | sudo tee "$LINUX_DIR/initrd" >/dev/null
sudo cp "$LINUX_ISO" "$LINUX_DIR/install.iso"
sudo chmod 755 "$LINUX_DIR"
sudo chmod 644 "$LINUX_DIR/vmlinuz" "$LINUX_DIR/initrd" "$LINUX_DIR/install.iso"
test -s "$LINUX_DIR/vmlinuz" && test -s "$LINUX_DIR/initrd"
```

复制 ISO 到 HTTP 子目录会额外占一份空间，但不需要修改 nginx alias。原 `/iso/ubuntu.iso` 在当前 nginx 中是专门指向旧 ISO 的固定 alias，不要拿它作为新版本下载地址。

### 4.2 添加菜单

在 choose 前增加：

```ipxe
item ubuntu-server-extra Ubuntu Server - additional version
```

在 :shell 前增加：

```ipxe
:ubuntu-server-extra
kernel ${base}/ubuntu-server-extra/vmlinuz initrd=initrd.magic boot=casper ip=dhcp url=${base}/ubuntu-server-extra/install.iso cloud-config-url=/dev/null --- || goto failed
initrd --name initrd ${base}/ubuntu-server-extra/initrd || goto failed
boot || goto failed
```

本项目现有 iPXE 使用 `initrd.magic` 的内核参数和 `--name initrd` 组合；不要把 Windows 的尾部虚拟文件名参数照搬到 Linux 行。
此入口保持交互安装，不加入 autoinstall、无人值守分区或清盘配置。
Desktop 可按同样的 casper 流程另建目录，但须核对目标版本的网络启动参数，并为下载 ISO 和 live 环境预留足够内存。
如果同一 ISO 在光驱可启动但 HTTP 启动失败，检查目标发行版是否支持该 live 网络安装方式，不能认为所有 Linux ISO 都支持 url=。

```bash
curl -fI "$PXE_BASE/ubuntu-server-extra/vmlinuz"
curl -fI "$PXE_BASE/ubuntu-server-extra/initrd"
curl -fI "$PXE_BASE/ubuntu-server-extra/install.iso"
curl -f -r 0-2047 -o /tmp/ubuntu-extra-range.bin "$PXE_BASE/ubuntu-server-extra/install.iso"
wc -c /tmp/ubuntu-extra-range.bin
```

Range 文件预期为 2048 字节。安装器起来后若找不到 rootfs，优先核对 ISO、内核和 initrd 是否同一版本以及 URL 是否可达。

## 5. 新增 Debian：使用官方 netboot 安装器

Debian 的 netinst ISO 内核/initrd 面向光盘安装，不能简单加上 Ubuntu 的 url= 参数。这里使用 Debian 13 trixie 的官方 amd64 netboot 文件；安装过程还需访问 Debian 软件源。

```bash
export DEB_DIR="$NETBOOT_ROOT/http/debian-trixie"
export DEB_NETBOOT=https://deb.debian.org/debian/dists/trixie/main/installer-amd64/current/images/netboot/debian-installer/amd64
test ! -e "$DEB_DIR" || exit 1
sudo mkdir -p "$DEB_DIR"
sudo curl --fail --location --retry 3 "$DEB_NETBOOT/linux" -o "$DEB_DIR/linux"
sudo curl --fail --location --retry 3 "$DEB_NETBOOT/initrd.gz" -o "$DEB_DIR/initrd.gz"
sudo chmod 755 "$DEB_DIR"
sudo chmod 644 "$DEB_DIR/linux" "$DEB_DIR/initrd.gz"
sha256sum "$DEB_DIR/linux" "$DEB_DIR/initrd.gz"
```

同一次下载两份文件，并记录来源、时间和哈希；官方 current 会更新。需要长期复现时保存经验证的这两份文件。

```ipxe
# item 放 choose 前；标签块放 :shell 前
item debian-trixie Debian 13 amd64 network installer
```

```ipxe
:debian-trixie
kernel ${base}/debian-trixie/linux initrd=initrd.magic ip=dhcp --- || goto failed
initrd --name initrd ${base}/debian-trixie/initrd.gz || goto failed
boot || goto failed
```

安装器中选择国家、网卡、镜像站和分区。没有互联网时，需要准备 Debian 内网软件源，含 dists/、pool/ 和元数据，并在安装器中指定内网镜像站。
**把 Debian ISO 放进 HTTP 目录并不等同于完整镜像站**。本文此方案不是离线安装方案。

## 6. 新增 Rocky Linux / AlmaLinux / RHEL 系列

### 6.1 使用完整 DVD ISO 建 HTTP 安装树

下面以 Rocky 9 x86_64 DVD 为模板。AlmaLinux 同系列可替换 ISO 和 ID；跨主要版本须核对发行方文档。Boot/minimal ISO 不一定提供完整软件仓库。

```bash
export RPM_ID=rocky9-extra
export RPM_ISO="$NETBOOT_ROOT/iso/你的Rocky-9-x86_64-dvd.iso"
export RPM_DIR="$NETBOOT_ROOT/http/$RPM_ID"
7z t "$RPM_ISO" || exit 1
test ! -e "$RPM_DIR" || exit 1
sudo mkdir -p "$RPM_DIR"
sudo bsdtar -xf "$RPM_ISO" -C "$RPM_DIR" --no-same-owner
sudo find "$RPM_DIR" -type d -exec chmod 755 {} +
sudo find "$RPM_DIR" -type f -exec chmod 644 {} +
ls -lh "$RPM_DIR/.treeinfo" "$RPM_DIR/images/pxeboot/vmlinuz" \
  "$RPM_DIR/images/pxeboot/initrd.img"
cat "$RPM_DIR/.treeinfo"
```

保留隐藏文件 `.treeinfo`，以及它引用的 stage2 路径（常见 images/install.img）、BaseOS、AppStream 和 repodata。不要只复制 images/pxeboot 两个文件。
当前 nginx 配置没有屏蔽 .treeinfo；如果你的自定义配置屏蔽所有隐藏文件，需单独允许读取这份安装元数据。

### 6.2 菜单

```ipxe
item rocky9-extra Rocky Linux 9 - additional version
```

```ipxe
:rocky9-extra
kernel ${base}/rocky9-extra/images/pxeboot/vmlinuz initrd=initrd.magic ip=dhcp inst.repo=${base}/rocky9-extra/ inst.stage2=${base}/rocky9-extra/ || goto failed
initrd --name initrd ${base}/rocky9-extra/images/pxeboot/initrd.img || goto failed
boot || goto failed
```

`inst.stage2` 用于启动安装器运行环境，`inst.repo` 用于软件安装源；仅启动内核不代表安装源齐全。菜单里的地址是解包树根目录，不是 ISO 文件 URL。

```bash
curl -fI "$PXE_BASE/$RPM_ID/.treeinfo"
curl -fI "$PXE_BASE/$RPM_ID/images/pxeboot/vmlinuz"
curl -fI "$PXE_BASE/$RPM_ID/images/pxeboot/initrd.img"
```

随后验证 .treeinfo 指定的 stage2 和 repo 元数据 URL。进入安装界面确认“安装源”及软件选择可用，再安装到测试磁盘。

## 7. 发布、验收、回滚

### 7.1 哪些服务需要重载

| 修改对象 | 如何生效 |
| --- | --- |
| boot.ipxe、WIM、内核、initrd、普通 HTTP 文件 | 无需重启 nginx/TFTP，客户端重启重新下载 |
| SMB 共享配置 | testparm 检查后向 netboot-smb 主进程发送 HUP |
| nginx 配置（本指南复制 ISO 的方式不需要改） | 先用实际配置路径执行 nginx -t，再重载对应服务 |
| DHCP 初始启动服务器、网段 | 才需要修改路由器或 Windows DHCP 配置 |

### 7.2 最低验收标准

1. 菜单 ID 唯一，item 与标签一致，全部标签位于执行可达位置。
2. 每个 HTTP URL 返回 200 且文件长度正确；大文件中断也可能留下 200 日志，应核对实际传输长度。
3. Windows 检查共享权限、客户端路由与 TCP 445；Linux 检查 ISO/安装树/软件源。
4. 按支持范围分别测试 BIOS 与 UEFI。进入安装器不等于完成安装。
5. 完成一台测试机安装并从硬盘重启，记录镜像 SHA256、固件模式、结果。

不要修改已经验证可用的 PE10/PE8 标签来新增普通 Windows 安装入口。PE10 的 UEFI 已由用户验证；PE8 保留 Legacy BIOS 路径。

### 7.3 常见报错定位

| 报错 | 优先检查 |
| --- | --- |
| 菜单没有新系统 | 是否只上传 ISO；是否改了运行目录；客户端是否重新下载菜单 |
| Not found / 404 | 大小写、HTTP 根目录、实际文件是否部署，不只是放在 /tmp |
| no bootloader file found | WIM 是否被精简；UEFI 外部启动器是否用 --name bootx64.efi |
| BCD 0xc000000f | BCD 内容引用的 WIM/SDI 路径、虚拟文件名；不要仅凭错误中的路径判定缺目录 |
| WinPE 系统错误 53 | 网卡驱动、IP、路由、服务器地址和 SMB 端口 |
| 拒绝访问 | SMB 身份认证、hosts allow、Linux 目录穿越/文件权限 |
| Windows Setup 不存在 | boot.wim 的 Setup 索引选错或镜像缺安装程序 |
| Linux unable to mount root fs | 内核/initrd 混版或 initrd 未正确交给内核 |
| Linux 安装器找不到介质 | ISO URL、inst.repo/.treeinfo、软件源网络连通性 |

### 7.4 回滚

使用第 2 节保存的 PXE_BACKUP。先确认路径正确，再执行：

```bash
sudo cp "$PXE_BACKUP/boot.ipxe" "$NETBOOT_ROOT/http/boot.ipxe"
sudo cp "$PXE_BACKUP/smb.conf" "$NETBOOT_ROOT/config/smb.conf"
sudo testparm -s "$NETBOOT_ROOT/config/smb.conf" && \
  sudo systemctl kill --kill-whom=main --signal=HUP netboot-smb
```

新资源目录先保留供诊断，不立即删除。备份之后若还有别的已生效修改，改为只移除本次新增 item、标签块、共享段，不用整文件回滚。

## 8. 同步到 GitHub 与复现资料

验收后把生效菜单同步到仓库 examples/boot.ipxe；新增脚本保存为独立示例目录，更新说明。
只提交脚本、菜单、配置模板、ISO 文件名和 SHA256。不要提交 ISO/WIM、真实密码和许可证。

`media.json` 和 `scripts/prepare.py` 目前是一套固定演示环境：手动新增系统不必修改它们；
若希望“从零生成”时也包含新系统，则还要同步 media.json 的键、prepare.py 的对应提取步骤及 scripts/verify.py 的 URL/共享检查。
仅在 media.json 加一行不会自动生成菜单或解包新发行版。SOURCE-DEMO.md 是源码快照，修改脚本后也要更新对应片段。

建议每个新增版本记录：ID、ISO 名、SHA256、架构、boot.wim 索引或内核路径、安装源 URL/共享名、支持固件、已完成的安装测试。

## 9. 官方参考

- [iPXE wimboot](https://ipxe.org/wimboot)：标准 WIM 启动和附加脚本注入。
- [Microsoft Windows Setup 命令行](https://learn.microsoft.com/en-us/windows-hardware/manufacture/desktop/windows-setup-command-line-options)：/InstallFrom 与分卷镜像。
- [Ubuntu Server 网络安装](https://documentation.ubuntu.com/server/how-to/installation/netboot-the-server-installer-on-amd64/)：live-server 网络启动说明，按目标版本核对。
- [Debian 网络安装](https://wiki.debian.org/DebianInstaller/Netboot) 与 [trixie amd64 netboot 文件](https://deb.debian.org/debian/dists/trixie/main/installer-amd64/current/images/netboot/debian-installer/amd64/)。
- [RHEL 网络安装文档](https://docs.redhat.com/en/documentation/red_hat_enterprise_linux/9/html/interactively_installing_rhel_over_the_network/index)：安装树与 Anaconda 网络引导参数。
