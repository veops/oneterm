package utils

import "net"

func IpFromNetAddr(addr net.Addr) string {
	switch t := addr.(type) {
	case *net.UDPAddr:
		return t.IP.String()
	case *net.TCPAddr:
		return t.IP.String()
	}
	return ""
}
