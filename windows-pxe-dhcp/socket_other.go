//go:build !windows

package main

import "net"

func setOutgoingInterface(_ *net.UDPConn, _ int) error { return nil }
