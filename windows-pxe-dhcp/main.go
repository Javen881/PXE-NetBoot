package main

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/ipv4"
)

const (
	bootpFixed = 236
	cookie     = "\x63\x82\x53\x63"
	discover   = 1
	offer      = 2
	request    = 3
	decline    = 4
	ack        = 5
	nak        = 6
	release    = 7
	inform     = 8
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

type Lease struct {
	IP      string    `json:"ip"`
	MAC     string    `json:"mac"`
	Expires time.Time `json:"expires"`
}

type Store struct {
	mu     sync.Mutex
	path   string
	leases map[string]Lease
}

type Packet struct {
	raw     []byte
	xid     []byte
	flags   uint16
	ciaddr  net.IP
	giaddr  net.IP
	chaddr  net.HardwareAddr
	options map[byte][]byte
}

func main() {
	configPath := flag.String("config", "config.json", "path to editable JSON configuration")
	check := flag.Bool("check", false, "validate configuration and exit")
	flag.Parse()

	cfg, base, err := loadConfig(*configPath)
	if err != nil {
		fatal(err)
	}
	iface, err := validate(cfg)
	if err != nil {
		fatal(err)
	}
	if *check {
		fmt.Printf("Configuration OK: interface=%s index=%d server=%s pool=%s-%s\n", iface.Name, iface.Index, cfg.ServerIP, cfg.PoolStart, cfg.PoolEnd)
		return
	}

	logger, closeLog, err := makeLogger(resolve(base, cfg.LogFile))
	if err != nil {
		fatal(err)
	}
	defer closeLog()
	store, err := openStore(resolve(base, cfg.LeaseFile))
	if err != nil {
		logger.Fatal(err)
	}

	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4zero, Port: 67})
	if err != nil {
		logger.Fatalf("bind UDP/67: %v (run as Administrator and stop other DHCP services)", err)
	}
	defer conn.Close()
	pc := ipv4.NewPacketConn(conn)
	if err := pc.SetControlMessage(ipv4.FlagInterface, true); err != nil {
		logger.Fatalf("enable interface filtering: %v", err)
	}
	logger.Printf("started interface=%s index=%d server=%s pool=%s-%s", iface.Name, iface.Index, cfg.ServerIP, cfg.PoolStart, cfg.PoolEnd)

	buf := make([]byte, 1600)
	for {
		n, cm, src, err := pc.ReadFrom(buf)
		if err != nil {
			logger.Printf("read: %v", err)
			continue
		}
		if cm == nil || cm.IfIndex != iface.Index {
			continue
		}
		p, err := parsePacket(buf[:n])
		if err != nil {
			logger.Printf("drop packet from %v: %v", src, err)
			continue
		}
		reply, summary, err := handle(cfg, store, p)
		if err != nil {
			logger.Printf("%s %v", p.chaddr, err)
			continue
		}
		if reply == nil {
			continue
		}
		dst := &net.UDPAddr{IP: net.IPv4bcast, Port: 68}
		if p.giaddr.To4() != nil && !p.giaddr.IsUnspecified() {
			dst = &net.UDPAddr{IP: p.giaddr, Port: 67}
		}
		if _, err := pc.WriteTo(reply, &ipv4.ControlMessage{IfIndex: iface.Index}, dst); err != nil {
			logger.Printf("send %s: %v", summary, err)
		} else {
			logger.Print(summary)
		}
	}
}

func loadConfig(path string) (Config, string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Config{}, "", err
	}
	var c Config
	if err := json.Unmarshal(b, &c); err != nil {
		return c, "", fmt.Errorf("parse config: %w", err)
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return c, "", err
	}
	return c, filepath.Dir(abs), nil
}

func validate(c Config) (*net.Interface, error) {
	if c.ListenInterface == "" {
		return nil, errors.New("listenInterface is required")
	}
	if c.LeaseTime < 60 {
		return nil, errors.New("leaseTimeSeconds must be at least 60")
	}
	server, start, end, mask := ip4(c.ServerIP), ip4(c.PoolStart), ip4(c.PoolEnd), ip4(c.SubnetMask)
	if server == nil || start == nil || end == nil || mask == nil {
		return nil, errors.New("serverIP, subnetMask and pool addresses must be IPv4")
	}
	if ipUint(start) > ipUint(end) {
		return nil, errors.New("poolStart must not exceed poolEnd")
	}
	if ipUint(server)&ipUint(mask) != ipUint(start)&ipUint(mask) || ipUint(start)&ipUint(mask) != ipUint(end)&ipUint(mask) {
		return nil, errors.New("server and pool must share the configured subnet")
	}
	if c.Router != "" && ip4(c.Router) == nil {
		return nil, errors.New("router must be IPv4")
	}
	if ip4(c.TFTPServer) == nil {
		return nil, errors.New("tftpServer must be IPv4")
	}
	for _, d := range c.DNSServers {
		if ip4(d) == nil {
			return nil, fmt.Errorf("invalid DNS server %q", d)
		}
	}
	iface, err := net.InterfaceByName(c.ListenInterface)
	if err != nil {
		return nil, fmt.Errorf("listenInterface %q: %w", c.ListenInterface, err)
	}
	addrs, err := iface.Addrs()
	if err != nil {
		return nil, err
	}
	found := false
	for _, a := range addrs {
		if strings.Split(a.String(), "/")[0] == c.ServerIP {
			found = true
		}
	}
	if !found {
		return nil, fmt.Errorf("interface %q does not own serverIP %s", iface.Name, c.ServerIP)
	}
	return iface, nil
}

func parsePacket(b []byte) (Packet, error) {
	if len(b) < bootpFixed+4 || b[0] != 1 || string(b[bootpFixed:bootpFixed+4]) != cookie {
		return Packet{}, errors.New("not a DHCP request")
	}
	hlen := int(b[2])
	if hlen < 1 || hlen > 16 {
		return Packet{}, errors.New("invalid hardware address length")
	}
	p := Packet{raw: append([]byte(nil), b...), xid: append([]byte(nil), b[4:8]...), flags: binary.BigEndian.Uint16(b[10:12]), ciaddr: net.IP(append([]byte(nil), b[12:16]...)), giaddr: net.IP(append([]byte(nil), b[24:28]...)), chaddr: net.HardwareAddr(append([]byte(nil), b[28:28+hlen]...)), options: map[byte][]byte{}}
	for i := bootpFixed + 4; i < len(b); {
		code := b[i]
		i++
		if code == 0 {
			continue
		}
		if code == 255 {
			break
		}
		if i >= len(b) {
			return Packet{}, errors.New("truncated option")
		}
		n := int(b[i])
		i++
		if i+n > len(b) {
			return Packet{}, errors.New("truncated option value")
		}
		p.options[code] = append([]byte(nil), b[i:i+n]...)
		i += n
	}
	if len(p.options[53]) != 1 {
		return Packet{}, errors.New("missing DHCP message type")
	}
	return p, nil
}

func handle(c Config, s *Store, p Packet) ([]byte, string, error) {
	t := p.options[53][0]
	mac := strings.ToLower(p.chaddr.String())
	if t == release || t == decline {
		s.remove(mac)
		return nil, "", nil
	}
	if t != discover && t != request {
		return nil, "", nil
	}
	if sid := p.options[54]; t == request && len(sid) == 4 && !net.IP(sid).Equal(ip4(c.ServerIP)) {
		return nil, "", nil
	}
	requested := net.IP(nil)
	if len(p.options[50]) == 4 {
		requested = net.IP(p.options[50])
	}
	if requested == nil && !p.ciaddr.IsUnspecified() {
		requested = p.ciaddr
	}
	lease, err := s.allocate(c, mac, requested, t == request && requested != nil)
	if err != nil {
		return buildReply(c, p, nil, nak, "no available address"), fmt.Sprintf("NAK mac=%s reason=%v", mac, err), nil
	}
	responseType := offer
	if t == request || t == inform {
		responseType = ack
	}
	reply := buildReply(c, p, ip4(lease.IP), byte(responseType), "")
	boot := bootSelection(c, p)
	return reply, fmt.Sprintf("%s mac=%s ip=%s arch=%s boot=%s", map[int]string{offer: "OFFER", ack: "ACK"}[responseType], mac, lease.IP, archName(p), boot), nil
}

func buildReply(c Config, p Packet, yiaddr net.IP, msgType byte, message string) []byte {
	b := make([]byte, bootpFixed+4, 1500)
	b[0], b[1], b[2] = 2, p.raw[1], p.raw[2]
	copy(b[4:8], p.xid)
	binary.BigEndian.PutUint16(b[10:12], p.flags)
	if yiaddr != nil {
		copy(b[16:20], yiaddr.To4())
	}
	copy(b[20:24], ip4(c.TFTPServer))
	copy(b[24:28], p.giaddr.To4())
	copy(b[28:44], p.raw[28:44])
	copy(b[bootpFixed:], []byte(cookie))
	add := func(code byte, value []byte) {
		if len(value) > 255 {
			return
		}
		b = append(b, code, byte(len(value)))
		b = append(b, value...)
	}
	add(53, []byte{msgType})
	add(54, ip4(c.ServerIP))
	if msgType != nak {
		add(1, ip4(c.SubnetMask))
		if c.Router != "" {
			add(3, ip4(c.Router))
		}
		var dns []byte
		for _, d := range c.DNSServers {
			dns = append(dns, ip4(d)...)
		}
		if len(dns) > 0 {
			add(6, dns)
		}
		v := make([]byte, 4)
		binary.BigEndian.PutUint32(v, c.LeaseTime)
		add(51, v)
		binary.BigEndian.PutUint32(v, c.LeaseTime/2)
		add(58, v)
		binary.BigEndian.PutUint32(v, c.LeaseTime*7/8)
		add(59, v)
		if boot := bootSelection(c, p); boot != "" {
			add(66, []byte(c.TFTPServer))
			add(67, []byte(boot))
			copy(b[108:236], []byte(boot))
		}
	} else if message != "" {
		add(56, []byte(message))
	}
	b = append(b, 255)
	return b
}

func bootSelection(c Config, p Packet) string {
	if isIPXE(p.options[77]) {
		return c.IPXEMenuURL
	}
	if !strings.Contains(string(p.options[60]), "PXEClient") {
		return ""
	}
	arch := uint16(0)
	if len(p.options[93]) >= 2 {
		arch = binary.BigEndian.Uint16(p.options[93][:2])
	}
	if arch == 7 || arch == 9 {
		return c.UEFIX64BootFile
	}
	if arch == 0 {
		return c.BIOSBootFile
	}
	return ""
}

func isIPXE(v []byte) bool { return string(v) == "iPXE" || strings.Contains(string(v), "iPXE") }
func archName(p Packet) string {
	if isIPXE(p.options[77]) {
		return "iPXE"
	}
	if len(p.options[93]) < 2 {
		return "unknown"
	}
	return fmt.Sprint(binary.BigEndian.Uint16(p.options[93][:2]))
}

func openStore(path string) (*Store, error) {
	s := &Store{path: path, leases: map[string]Lease{}}
	b, err := os.ReadFile(path)
	if err == nil {
		var a []Lease
		if json.Unmarshal(b, &a) == nil {
			for _, l := range a {
				s.leases[l.MAC] = l
			}
		}
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	return s, nil
}
func (s *Store) allocate(c Config, mac string, requested net.IP, strict bool) (Lease, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	if l, ok := s.leases[mac]; ok && l.Expires.After(now) {
		l.Expires = now.Add(time.Duration(c.LeaseTime) * time.Second)
		s.leases[mac] = l
		s.save()
		return l, nil
	}
	used := map[uint32]bool{}
	for m, l := range s.leases {
		if m != mac && l.Expires.After(now) {
			used[ipUint(ip4(l.IP))] = true
		}
	}
	start, end := ipUint(ip4(c.PoolStart)), ipUint(ip4(c.PoolEnd))
	candidate := uint32(0)
	if requested != nil {
		v := ipUint(requested.To4())
		if v >= start && v <= end && !used[v] {
			candidate = v
		}
		if strict && candidate == 0 {
			return Lease{}, errors.New("requested address is outside the pool or already in use")
		}
	}
	if candidate == 0 {
		for v := start; v <= end; v++ {
			if !used[v] {
				candidate = v
				break
			}
			if v == ^uint32(0) {
				break
			}
		}
	}
	if candidate == 0 {
		return Lease{}, errors.New("pool exhausted")
	}
	l := Lease{IP: uintIP(candidate).String(), MAC: mac, Expires: now.Add(time.Duration(c.LeaseTime) * time.Second)}
	s.leases[mac] = l
	s.save()
	return l, nil
}
func (s *Store) remove(mac string) { s.mu.Lock(); defer s.mu.Unlock(); delete(s.leases, mac); s.save() }
func (s *Store) save() {
	os.MkdirAll(filepath.Dir(s.path), 0755)
	a := make([]Lease, 0, len(s.leases))
	for _, l := range s.leases {
		a = append(a, l)
	}
	sort.Slice(a, func(i, j int) bool { return a[i].IP < a[j].IP })
	b, _ := json.MarshalIndent(a, "", "  ")
	_ = os.WriteFile(s.path, append(b, '\n'), 0644)
}
func makeLogger(path string) (*log.Logger, func(), error) {
	if path == "" {
		return log.New(os.Stdout, "", log.LstdFlags), func() {}, nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, nil, err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return nil, nil, err
	}
	return log.New(io.MultiWriter(os.Stdout, f), "", log.LstdFlags), func() { f.Close() }, nil
}
func resolve(base, p string) string {
	if p == "" || filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(base, p)
}
func ip4(s string) net.IP { return net.ParseIP(s).To4() }
func ipUint(ip net.IP) uint32 {
	if ip == nil {
		return 0
	}
	return binary.BigEndian.Uint32(ip.To4())
}
func uintIP(v uint32) net.IP {
	b := make([]byte, 4)
	binary.BigEndian.PutUint32(b, v)
	return net.IP(b)
}
func fatal(err error) { fmt.Fprintln(os.Stderr, "ERROR:", err); os.Exit(1) }
