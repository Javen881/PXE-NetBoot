package main

import (
	"encoding/binary"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func packet(msg byte, arch uint16, user string) Packet {
	p := Packet{raw: make([]byte, 300), xid: []byte{1, 2, 3, 4}, chaddr: net.HardwareAddr{0, 1, 2, 3, 4, 5}, ciaddr: net.IPv4zero, giaddr: net.IPv4zero, options: map[byte][]byte{53: []byte{msg}, 60: []byte("PXEClient")}}
	a := make([]byte, 2)
	binary.BigEndian.PutUint16(a, arch)
	p.options[93] = a
	if user != "" {
		p.options[77] = []byte(user)
	}
	return p
}

func TestLeaseIsStableAndPersisted(t *testing.T) {
	path := filepath.Join(t.TempDir(), "leases.json")
	s, err := openStore(path)
	if err != nil {
		t.Fatal(err)
	}
	c := Config{PoolStart: "192.168.50.100", PoolEnd: "192.168.50.101", LeaseTime: 3600}
	a, err := s.allocate(c, "00:01:02:03:04:05", nil, false)
	if err != nil {
		t.Fatal(err)
	}
	b, err := s.allocate(c, "00:01:02:03:04:05", nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if a.IP != "192.168.50.100" || b.IP != a.IP {
		t.Fatalf("unstable lease: %#v %#v", a, b)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
	if !b.Expires.After(time.Now()) {
		t.Fatal("lease not renewed")
	}
}

func TestStrictRequestedAddress(t *testing.T) {
	s, err := openStore(filepath.Join(t.TempDir(), "leases.json"))
	if err != nil {
		t.Fatal(err)
	}
	c := Config{PoolStart: "192.168.50.100", PoolEnd: "192.168.50.101", LeaseTime: 3600}
	if _, err := s.allocate(c, "00:00:00:00:00:01", net.ParseIP("192.168.60.1"), true); err == nil {
		t.Fatal("expected out-of-pool request to fail")
	}
}

func TestReplyContainsPXEOptions(t *testing.T) {
	c := Config{ServerIP: "192.168.50.1", SubnetMask: "255.255.255.0", Router: "192.168.50.1", DNSServers: []string{"1.1.1.1"}, LeaseTime: 3600, TFTPServer: "192.168.10.220", UEFIX64BootFile: "ipxe.efi"}
	p := packet(discover, 7, "")
	b := buildReply(c, p, net.ParseIP("192.168.50.100"), offer, "")
	if len(b) < bootpFixed+4 || b[0] != 2 {
		t.Fatal("invalid BOOTP reply")
	}
	parsed := optionMap(t, b)
	if string(parsed[67]) != "ipxe.efi" {
		t.Fatalf("boot file=%q", parsed[67])
	}
	if !net.IP(parsed[54]).Equal(net.ParseIP("192.168.50.1")) {
		t.Fatalf("server id=%v", parsed[54])
	}
}

func optionMap(t *testing.T, b []byte) map[byte][]byte {
	t.Helper()
	out := map[byte][]byte{}
	for i := bootpFixed + 4; i < len(b); {
		code := b[i]
		i++
		if code == 255 {
			break
		}
		if code == 0 {
			continue
		}
		if i >= len(b) {
			t.Fatal("bad option")
		}
		n := int(b[i])
		i++
		if i+n > len(b) {
			t.Fatal("bad option length")
		}
		out[code] = b[i : i+n]
		i += n
	}
	return out
}
func TestBootSelection(t *testing.T) {
	c := Config{BIOSBootFile: "undionly.kpxe", UEFIX64BootFile: "ipxe.efi", IPXEMenuURL: "http://server/menu"}
	cases := []struct {
		p Packet
		w string
	}{
		{packet(discover, 0, ""), "undionly.kpxe"},
		{packet(discover, 7, ""), "ipxe.efi"},
		{packet(discover, 9, ""), "ipxe.efi"},
		{packet(discover, 7, "iPXE"), "http://server/menu"},
	}
	for _, x := range cases {
		if g := bootSelection(c, x.p); g != x.w {
			t.Fatalf("got %q want %q", g, x.w)
		}
	}
}

func TestParseRejectsTruncated(t *testing.T) {
	if _, err := parsePacket([]byte{1}); err == nil {
		t.Fatal("expected error")
	}
}
