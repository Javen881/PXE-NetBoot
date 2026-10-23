# 历史脚本（仅供审计，不作为执行入口）

完整保留现有部署的 deploy.py、finalize.py、vpn-enable.py、verify.py 和早期路由器模板。

- deploy.py 会覆盖配置并停用发行版默认服务，包含早期 Windows 启动逻辑。
- finalize.py 依赖 /tmp 中的文件，不能直接用于另一台机器。
- vpn-enable.py 是一次性补丁，会覆盖旧脚本；不会创建 VPN。
- verify.py 未覆盖后加的 Windows 11。
- 早期模板使用 TEST_MAC，当前实际规则见 ../router/dnsmasq-pxe.conf。

请使用 ../scripts/prepare.py 和 ../docs/DEMO.md；不要直接执行历史脚本。


另外保留 Linux initrd、TFTP、Win11 接入及重试安装的一次性补丁。它们包含临时路径或一次性断言，仅供追溯；当前 Win11 脚本见 examples/windows11/netboot.cmd。
