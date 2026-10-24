//go:build windows

package main

import (
	"fmt"
	"math/bits"
	"net"

	"golang.org/x/sys/windows"
)

// Windows does not implement ipv4.PacketConn interface control messages.
// IP_UNICAST_IF pins limited-broadcast replies to the configured LAN adapter.
func setOutgoingInterface(conn *net.UDPConn, interfaceIndex int) error {
	raw, err := conn.SyscallConn()
	if err != nil {
		return err
	}
	var socketErr error
	err = raw.Control(func(fd uintptr) {
		const ipUnicastIf = 31
		indexNetworkOrder := int(bits.ReverseBytes32(uint32(interfaceIndex)))
		socketErr = windows.SetsockoptInt(windows.Handle(fd), windows.IPPROTO_IP, ipUnicastIf, indexNetworkOrder)
	})
	if err != nil {
		return err
	}
	if socketErr != nil {
		return fmt.Errorf("IP_UNICAST_IF index %d: %w", interfaceIndex, socketErr)
	}
	return nil
}
