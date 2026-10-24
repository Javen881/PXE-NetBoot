//go:build windows

package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"
)

type Config struct {
	ListenInterface string   `json:"listenInterface"`
	ServerIP        string   `json:"serverIP"`
	SubnetMask      string   `json:"subnetMask"`
	PoolStart       string   `json:"poolStart"`
	PoolEnd         string   `json:"poolEnd"`
	Router          string   `json:"router"`
	DNSServers      []string `json:"dnsServers"`
	LeaseTime       uint32   `json:"leaseTimeSeconds"`
	TFTPServer      string   `json:"tftpServer"`
	BIOSBootFile    string   `json:"biosBootFile"`
	UEFIX64BootFile string   `json:"uefiX64BootFile"`
	IPXEMenuURL     string   `json:"ipxeMenuURL"`
	LeaseFile       string   `json:"leaseFile"`
	LogFile         string   `json:"logFile"`
}

var (
	mw                                                         *walk.MainWindow
	adapter                                                    *walk.ComboBox
	serverIP, mask, poolStart, poolEnd, router, dns, leaseTime *walk.LineEdit
	tftp, bios, uefi, menuURL, leaseFile, logFile              *walk.LineEdit
	logView                                                    *walk.TextEdit
	statusLabel                                                *walk.Label
	startButton, stopButton, saveButton, checkButton           *walk.PushButton
	process                                                    *exec.Cmd
	processMu                                                  sync.Mutex
	interfaceOptions                                           []string
)

func main() {
	defer func() {
		if recovered := recover(); recovered != nil {
			fatalGUI(fmt.Errorf("程序异常：%v", recovered))
		}
	}()
	interfaceOptions = interfaceNames()
	err := MainWindow{
		AssignTo: &mw,
		Title:    "Windows PXE DHCP 管理器",
		MinSize:  Size{Width: 820, Height: 680},
		Size:     Size{Width: 920, Height: 760},
		Layout:   VBox{MarginsZero: false},
		Children: []Widget{
			GroupBox{Title: "现场 LAN 与 DHCP 地址池", Layout: Grid{Columns: 4}, Children: []Widget{
				Label{Text: "监听网卡"}, ComboBox{AssignTo: &adapter, Model: interfaceOptions},
				Label{Text: "本机 LAN 地址"}, LineEdit{AssignTo: &serverIP},
				Label{Text: "子网掩码"}, LineEdit{AssignTo: &mask},
				Label{Text: "默认网关"}, LineEdit{AssignTo: &router},
				Label{Text: "地址池开始"}, LineEdit{AssignTo: &poolStart},
				Label{Text: "地址池结束"}, LineEdit{AssignTo: &poolEnd},
				Label{Text: "DNS（逗号分隔）"}, LineEdit{AssignTo: &dns},
				Label{Text: "租期（秒）"}, LineEdit{AssignTo: &leaseTime},
			}},
			GroupBox{Title: "PXE 启动资源", Layout: Grid{Columns: 4}, Children: []Widget{
				Label{Text: "TFTP 服务器"}, LineEdit{AssignTo: &tftp},
				Label{Text: "BIOS 启动文件"}, LineEdit{AssignTo: &bios},
				Label{Text: "UEFI x64 启动文件"}, LineEdit{AssignTo: &uefi},
				Label{Text: "iPXE 菜单 URL"}, LineEdit{AssignTo: &menuURL},
				Label{Text: "租约文件"}, LineEdit{AssignTo: &leaseFile},
				Label{Text: "日志文件"}, LineEdit{AssignTo: &logFile},
			}},
			Composite{Layout: HBox{}, Children: []Widget{
				PushButton{AssignTo: &saveButton, Text: "保存配置", OnClicked: saveConfig},
				PushButton{Text: "重新载入", OnClicked: func() {
					if err := loadIntoForm(); err != nil {
						showError(err)
					} else {
						appendLog("已重新载入配置。")
					}
				}},
				PushButton{AssignTo: &checkButton, Text: "检查配置", OnClicked: checkConfig},
				PushButton{AssignTo: &startButton, Text: "启动 DHCP", OnClicked: startServer},
				PushButton{AssignTo: &stopButton, Text: "停止", Enabled: false, OnClicked: stopServer},
				PushButton{Text: "安装防火墙规则", OnClicked: installFirewall},
				PushButton{Text: "打开程序目录", OnClicked: openDirectory},
			}},
			Label{Text: "运行日志（启动需要管理员权限；LAN 内不能有其他 DHCP 服务）"},
			TextEdit{AssignTo: &logView, ReadOnly: true, VScroll: true},
			Label{AssignTo: &statusLabel, Text: "状态：未启动"},
		},
	}.Create()
	if err != nil {
		fatalGUI(fmt.Errorf("创建窗口失败：%w", err))
		return
	}

	mw.Closing().Attach(func(canceled *bool, reason walk.CloseReason) { stopServer() })
	if err := loadIntoForm(); err != nil {
		appendLog("未加载 config.json：" + err.Error() + "，已显示默认参数。")
		setDefaults()
	}
	mw.Run()
}

func executableDir() string { p, _ := os.Executable(); return filepath.Dir(p) }
func configPath() string    { return filepath.Join(executableDir(), "config.json") }
func serverPath() string    { return filepath.Join(executableDir(), "windows-pxe-dhcp.exe") }

func interfaceNames() []string {
	list, _ := net.Interfaces()
	names := make([]string, 0, len(list))
	for _, i := range list {
		names = append(names, i.Name)
	}
	return names
}

func setDefaults() {
	setForm(Config{ListenInterface: "Ethernet", ServerIP: "192.168.50.1", SubnetMask: "255.255.255.0", PoolStart: "192.168.50.100", PoolEnd: "192.168.50.199", Router: "192.168.50.1", DNSServers: []string{"223.5.5.5", "1.1.1.1"}, LeaseTime: 3600, TFTPServer: "192.168.10.220", BIOSBootFile: "undionly.kpxe", UEFIX64BootFile: "ipxe.efi", IPXEMenuURL: "http://192.168.10.220:8088/boot.ipxe", LeaseFile: "data/leases.json", LogFile: "logs/server.log"})
}

func loadIntoForm() error {
	b, err := os.ReadFile(configPath())
	if os.IsNotExist(err) {
		b, err = os.ReadFile(filepath.Join(executableDir(), "config.example.json"))
	}
	if err != nil {
		return err
	}
	var c Config
	if err := json.Unmarshal(b, &c); err != nil {
		return err
	}
	setForm(c)
	return nil
}

func setForm(c Config) {
	adapter.SetCurrentIndex(-1)
	for i, name := range interfaceOptions {
		if name == c.ListenInterface {
			adapter.SetCurrentIndex(i)
			break
		}
	}
	if adapter.CurrentIndex() < 0 {
		adapter.SetText(c.ListenInterface)
	}
	serverIP.SetText(c.ServerIP)
	mask.SetText(c.SubnetMask)
	poolStart.SetText(c.PoolStart)
	poolEnd.SetText(c.PoolEnd)
	router.SetText(c.Router)
	dns.SetText(strings.Join(c.DNSServers, ", "))
	leaseTime.SetText(strconv.FormatUint(uint64(c.LeaseTime), 10))
	tftp.SetText(c.TFTPServer)
	bios.SetText(c.BIOSBootFile)
	uefi.SetText(c.UEFIX64BootFile)
	menuURL.SetText(c.IPXEMenuURL)
	leaseFile.SetText(c.LeaseFile)
	logFile.SetText(c.LogFile)
}

func formConfig() (Config, error) {
	seconds, err := strconv.ParseUint(strings.TrimSpace(leaseTime.Text()), 10, 32)
	if err != nil {
		return Config{}, fmt.Errorf("租期必须是整数")
	}
	var dnsList []string
	for _, s := range strings.Split(dns.Text(), ",") {
		if s = strings.TrimSpace(s); s != "" {
			dnsList = append(dnsList, s)
		}
	}
	return Config{ListenInterface: strings.TrimSpace(adapter.Text()), ServerIP: strings.TrimSpace(serverIP.Text()), SubnetMask: strings.TrimSpace(mask.Text()), PoolStart: strings.TrimSpace(poolStart.Text()), PoolEnd: strings.TrimSpace(poolEnd.Text()), Router: strings.TrimSpace(router.Text()), DNSServers: dnsList, LeaseTime: uint32(seconds), TFTPServer: strings.TrimSpace(tftp.Text()), BIOSBootFile: strings.TrimSpace(bios.Text()), UEFIX64BootFile: strings.TrimSpace(uefi.Text()), IPXEMenuURL: strings.TrimSpace(menuURL.Text()), LeaseFile: strings.TrimSpace(leaseFile.Text()), LogFile: strings.TrimSpace(logFile.Text())}, nil
}

func saveConfig() { _ = saveConfigFile() }
func saveConfigFile() bool {
	c, err := formConfig()
	if err != nil {
		showError(err)
		return false
	}
	b, _ := json.MarshalIndent(c, "", "  ")
	if err := os.WriteFile(configPath(), append(b, '\n'), 0644); err != nil {
		showError(err)
		return false
	}
	appendLog("配置已保存：" + configPath())
	return true
}

func checkConfig() {
	if saveConfigFile() {
		runOnce("--check")
	}
}

func runOnce(arg string) {
	cmd := exec.Command(serverPath(), "--config", configPath(), arg)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	out, err := cmd.CombinedOutput()
	appendLog(strings.TrimSpace(string(out)))
	if err != nil {
		showError(fmt.Errorf("检查失败：%w", err))
		return
	}
	walk.MsgBox(mw, "配置检查", "配置检查通过。", walk.MsgBoxIconInformation)
}

func startServer() {
	processMu.Lock()
	if process != nil {
		processMu.Unlock()
		return
	}
	processMu.Unlock()
	if !saveConfigFile() {
		return
	}
	cmd := exec.Command(serverPath(), "--config", configPath())
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		showError(err)
		return
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		showError(err)
		return
	}
	if err := cmd.Start(); err != nil {
		showError(fmt.Errorf("启动失败（请以管理员身份运行 GUI）：%w", err))
		return
	}
	processMu.Lock()
	process = cmd
	processMu.Unlock()
	setRunning(true)
	appendLog("DHCP 服务正在启动……")
	go scanOutput(stdout)
	go scanOutput(stderr)
	go func() {
		err := cmd.Wait()
		mw.Synchronize(func() {
			processMu.Lock()
			if process == cmd {
				process = nil
			}
			processMu.Unlock()
			setRunning(false)
			if err != nil {
				appendLog("服务已退出：" + err.Error())
			} else {
				appendLog("服务已停止。")
			}
		})
	}()
}

func scanOutput(r interface{ Read([]byte) (int, error) }) {
	s := bufio.NewScanner(r)
	for s.Scan() {
		line := s.Text()
		mw.Synchronize(func() { appendLog(line) })
	}
}

func stopServer() {
	processMu.Lock()
	cmd := process
	processMu.Unlock()
	if cmd != nil && cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
}

func setRunning(running bool) {
	startButton.SetEnabled(!running)
	stopButton.SetEnabled(running)
	saveButton.SetEnabled(!running)
	checkButton.SetEnabled(!running)
	status := "状态：未启动"
	if running {
		status = "状态：运行中"
	}
	statusLabel.SetText(status)
}

func appendLog(s string) {
	if strings.TrimSpace(s) == "" {
		return
	}
	logView.AppendText(time.Now().Format("15:04:05") + "  " + s + "\r\n")
}

func showError(err error) {
	appendLog(err.Error())
	walk.MsgBox(mw, "错误", err.Error(), walk.MsgBoxIconError)
}

func fatalGUI(err error) {
	path := filepath.Join(executableDir(), "gui-crash.log")
	message := time.Now().Format(time.RFC3339) + "  " + err.Error() + "\r\n"
	file, openErr := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if openErr == nil {
		_, _ = file.WriteString(message)
		_ = file.Close()
	}
	walk.MsgBox(nil, "Windows PXE DHCP 启动失败", err.Error()+"\n\n错误日志："+path, walk.MsgBoxIconError)
}
func openDirectory() { _ = exec.Command("explorer.exe", executableDir()).Start() }
func installFirewall() {
	script := filepath.Join(executableDir(), "firewall-install.ps1")
	command := fmt.Sprintf("Start-Process PowerShell -Verb RunAs -ArgumentList '-NoProfile -ExecutionPolicy Bypass -File \"%s\"'", strings.ReplaceAll(script, "'", "''"))
	cmd := exec.Command("powershell.exe", "-NoProfile", "-Command", command)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if err := cmd.Start(); err != nil {
		showError(err)
		return
	}
	appendLog("已请求管理员权限安装防火墙规则。")
}
