# Windows Portable PXE DHCP

这是一个 Windows 原生、单文件的 DHCPv4/PXE 服务端，面向现场直连装机：Windows 电脑的 LAN 口为
`192.168.50.1/24`，待安装机器从该电脑获取 `192.168.50.100-199`，随后通过已打通的链路访问
`192.168.10.220` 上的 TFTP、HTTP 和安装源。

它实现现有 OpenWrt dnsmasq 规则的核心行为：

| 客户端 | 下发内容 |
| --- | --- |
| Legacy BIOS PXE，架构 0 | `undionly.kpxe`，TFTP `192.168.10.220` |
| x64 UEFI PXE，架构 7/9 | `ipxe.efi`，TFTP `192.168.10.220` |
| User Class 为 iPXE | `http://192.168.10.220:8088/boot.ipxe` |

## 使用

### GUI（推荐）

1. 解压 ZIP，右键 `windows-pxe-dhcp-gui.exe`，选择“以管理员身份运行”。
2. 在窗口中选择现场 LAN 网卡，修改地址池、DNS、TFTP、BIOS/UEFI 启动文件和 iPXE 菜单地址。
3. 点击“保存配置”，程序会在 EXE 同目录生成 `config.json`；以后修改参数不需要重新编译。
4. 点击“检查配置”，通过后点击“启动 DHCP”。运行日志会实时显示在窗口中。
5. 装机完成后点击“停止”。服务运行期间配置按钮会锁定，避免界面参数与实际下发参数不一致。

GUI 和命令行共用同一个 `config.json` 和 DHCP 引擎。GUI 包不内置镜像、TFTP 或 HTTP 文件。

### 命令行

1. Windows LAN 网卡设为静态地址 `192.168.50.1/24`，网络配置文件设为“专用”。
2. 编辑 `config.example.json` 中的 `listenInterface`，必须与 PowerShell `Get-NetAdapter` 显示的网卡名称一致。
3. 将 `config.example.json` 复制为 `config.json`，按现场情况修改地址池、网关、DNS、TFTP 和菜单地址。
4. 以管理员 PowerShell 运行 `firewall-install.ps1`，只为专用网络开放本程序 UDP 67。
5. 以管理员身份运行 `check.cmd`，通过后运行 `start.cmd`。
6. LAN 网段不能同时存在另一个 DHCP 服务端；待装机重新进行 PXE 启动。

`config.json` 是外部文件，修改参数不需要重新编译。租约写到 `data/leases.json`，运行日志写到
`logs/server.log`。程序按接口名称查找 Windows 网卡，确认它确实拥有 `serverIP`，并使用收到报文的接口索引过滤请求和发送响应，避免响应 VPN 或 Wi-Fi 上的 DHCP 请求。

## 配置检查

```bat
windows-pxe-dhcp.exe --config config.json --check
```

检查内容包括网卡名称、网卡地址、IPv4 格式、地址池顺序、地址池网段和租期。检查通过只表示本机参数有效；
仍需用隔离 LAN 上的测试虚拟机或实体机完成 DHCP DORA、BIOS/UEFI、iPXE 和安装验收。

## 边界

- 当前版本只实现 DHCPv4、一个地址池和本项目需要的 PXE 分流，不提供 DNS 缓存、DHCPv6、RA 或本地 TFTP。
- TFTP 与安装资源仍由 `192.168.10.220` 提供。
- UDP 67 需要管理员权限。Windows ICS、Hyper-V NAT、其他 DHCP 软件可能占用端口或抢答。
- 当前 EXE 在 Linux 上交叉编译并通过单元测试，但必须在 Windows 现场完成接口过滤和真实 PXE 报文验收。
- 这是针对需求实现的原生程序，不是官方 dnsmasq，也不冒用 dnsmasq 名称。官方 dnsmasq 不支持 Windows 原生平台。

## 构建

安装 Go 1.24 或更高版本，在 PowerShell 中运行：

```powershell
./build.ps1
```

产物位于 `dist/windows-pxe-dhcp.exe`。构建固定依赖版本记录在 `go.mod` 和 `go.sum`。
