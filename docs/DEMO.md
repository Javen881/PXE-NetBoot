# 完整 Demo：从仓库生成可部署目录

本 demo 面向本次固定环境：Ubuntu、用户 javen03、网卡 enp4s0、地址 192.168.10.220，
A/B 网段为 192.168.10.0/24 与 192.168.18.0/24。它不是支持任意发行版和任意地址的一键安装器。
部署到不同环境，应先统一修改 config、systemd、examples、router、tftp 与脚本中的地址和用户。

## 1. 仓库覆盖范围

| 目录 | 内容及作用 |
| --- | --- |
| scripts/ | 资源准备、独立服务启用、HTTP/SMB 验证、停用回滚 |
| config/ | nginx、仅 TFTP 的 dnsmasq、两套 Windows 只读共享 |
| systemd/ | 三个独立服务单元 |
| examples/ | 完整 iPXE 菜单、LTSC 脚本、Win11 自动重试脚本 |
| http/ | 网页入口 |
| tftp/ | iPXE 链式入口 |
| router/ | 当前 OpenWrt DHCP 引导规则 |
| media.json | 五个本地镜像文件名映射 |
| history/ | 原环境的四个历史脚本和早期配置，供追溯，不直接执行 |
| docs/boot-assets.deployed.json | 当前启动资源大小和 SHA256 |
| docs/SOURCE-DEMO.md | 全部脚本和文本配置的 Markdown 内嵌版本 |

除 ISO 外，WIM、ESD、Windows 解包文件、Ubuntu 内核/initrd、ESXi 模块也是安装介质内容，
由用户自己的镜像提取，不重复上传。iPXE/wimboot 是第三方启动程序，按第 3 节恢复。
不上传服务器日志、备份中的其他配置、SSH 密钥、账号口令或运行目录。

## 2. 获取仓库与安装依赖

~~~bash
git clone https://github.com/Javen881/PXE-NetBoot.git
cd PXE-NetBoot
sudo apt update
sudo apt install python3 nginx dnsmasq-base samba smbclient ipxe curl 7zip libarchive-tools wimtools
~~~

先检查已有服务和端口；安装依赖可能启动发行版默认服务。
只有确认默认 nginx/smbd/nmbd 没有业务时，才按主手册停用它们。
demo 不会替你关闭已有服务。

查看 media.json，把五个镜像放入一个目录，保持文件名一致。
原始镜像目录不会被修改。新增任意系统不能只改文件名映射，
还需根据安装器类型适配提取路径、启动参数和安装源；Windows 11 的完整案例见主手册第 7 节。

## 3. 生成资源（不启动服务）

先检查输入：

~~~bash
python3 scripts/prepare.py --iso-dir /home/javen03/data/netboot/iso \
  --root /home/javen03/data/netboot-demo --check
~~~

再执行准备：

~~~bash
python3 scripts/prepare.py --iso-dir /home/javen03/data/netboot/iso \
  --root /home/javen03/data/netboot-demo
~~~

目标必须为空；脚本拒绝覆盖正在使用的部署目录。
失败后保留部分输出用于排错，修复问题后选择另一个空目录重试。
该步骤会产生约十余 GB 的解包资源，需预留充足空间。

脚本完成以下工作：

1. 检查五个镜像、依赖及 iPXE 软件包文件。
2. 提取 Ubuntu 内核/initrd、两套 Windows、PE 与 ESXi。
3. 检查 Windows 镜像容器以及 boot.wim 的 Setup 索引 2、install.wim。
4. 复制 Win11 自动重试脚本，写成 CRLF。
5. 转换 ESXi 模块大小写并设置 HTTP prefix。
6. 从 /usr/lib/ipxe 复制 BIOS、UEFI、光盘引导程序。
7. 下载固定版本 wimboot v2.9.0。
8. 生成当前菜单、网页、TFTP 入口、配置和 systemd 单元。
9. 将配置中的根目录改为输出目录，Ubuntu alias 指向原始 ISO。
10. 设置资源权限，记录启动文件 SHA256 和原始镜像路径。

iPXE 版本由 Ubuntu 软件包提供；不同软件包版本的哈希可能不同。
wimboot 下载来自其官方 GitHub release：
https://github.com/ipxe/wimboot/releases/tag/v2.9.0
仓库中的哈希记录是当前环境指纹，不构成发行方签名或下载完整性的替代。
离线环境可提前按同样路径提供软件包与官方启动文件，再调整下载步骤。

## 4. 启用服务（仅在新主机或完成迁移后）

当前运行的 192.168.10.220 已有服务，不能同时启用第二套同端口服务。
install-services.sh 会拒绝已有同名单元或被占用的端口。

在新主机完成地址、用户、网卡、文件权限、路由和防火墙配置后：

~~~bash
sudo bash scripts/install-services.sh /home/javen03/data/netboot-demo
~~~

此步骤先检查 nginx/dnsmasq/Samba 配置，再安装和启用三个服务。
若某个启动失败，使用 systemctl status 和 journalctl 定位；
需要退出本次部署时运行第 7 节停用脚本。准备步骤不会自动更改路由器。

## 5. 路由器 Demo

把 router/dnsmasq-pxe.conf 内容合并到为客户端提供 DHCP 的 OpenWrt 实例。
先备份 /etc/dnsmasq.conf；如果已经存在 NETBOOT PXE 段，替换旧段而非重复添加。
确认实际 /var/etc/dnsmasq.conf.* 包含该文件，校验实际配置后重启 dnsmasq。
完整命令见主手册第 5 节。

站点 VPN 由现有网络提供，不包含建立 VPN 的脚本。
确保 B 网段到服务器 HTTP、SMB、TFTP 及其后续 UDP 端口双向可达。
Wi-Fi 集中管理、公网 HTTPS 和脱离 VPN 的公网 Windows 安装未实现。

## 6. 验证

~~~bash
python3 scripts/verify.py --root /home/javen03/data/netboot-demo --server 192.168.10.220
~~~

脚本逐个检查 HTTP 资源及 ESXi 模块、Ubuntu ISO Range 响应、两个 Windows 共享。
它必须针对实际运行的服务执行，不能仅准备目录后就宣称验证完成。
TFTP 下载校验参考 history/verify.py 中的实现与主手册记录；
完整验收还包括客户端 DHCP、iPXE、WinPE/安装器、磁盘识别、安装和首次重启。

## 7. 停用与回滚

~~~bash
sudo bash scripts/rollback-services.sh
~~~

仅停止并取消自启，不删除镜像、单元或配置。
路由器需单独恢复修改前备份并校验、重启 dnsmasq。
已有部署迁移前，备份三个 systemd 单元、config/、菜单和 Windows 启动脚本；
恢复后执行 daemon-reload 和服务重启。不要在运行中覆盖整套资源目录。

## 8. 阅读完整源码

[完整源码 Demo](SOURCE-DEMO.md) 内嵌所有脚本、配置和入口文件。
可执行文件以仓库相应路径为准；Markdown 用于阅读和复制。
历史脚本在独立章节标注，不能把它们当成新部署步骤。


## 9. 本次验证记录（2026-10-23）

- 使用真实五个 ISO，在 /tmp/pxe-demo-validation-v3 完整运行 prepare.py，成功生成资源。
- 已修复实跑发现的 ESXi boot.cfg 只读权限问题。
- 核对静态菜单引用文件、输出根路径、原 ISO alias 与 Win11 CRLF 脚本。
- dnsmasq 与 Samba 配置语法检查通过。
- verify.py 对现有运行环境检查 185 项 HTTP 资源、ISO Range 和两套 SMB 共享，全部通过。
- 未在当前生产主机执行 install-services.sh 或 rollback-services.sh，避免替换正在使用的服务；只完成其 Shell 语法检查。
- 客户端完整系统安装、Secure Boot 与公网 HTTPS 不在上述通过范围内。

临时磁盘配额独立于主磁盘剩余空间。多次完整解包曾达到临时配额；只清理本次测试生成的目录后重跑通过。
