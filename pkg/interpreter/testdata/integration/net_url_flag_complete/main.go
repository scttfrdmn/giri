// net_url_flag_complete exercises additions from v0.74.0:
// net.LookupAddr/LookupSRV/ParseMAC/Interfaces/InterfaceAddrs/InterfaceByName,
// net.IP methods (String/Equal/IsLoopback/IsGlobalUnicast/IsMulticast/
// IsLinkLocalUnicast/IsUnspecified/To4/To16/Mask), net.IPNet.Contains,
// net.Conn methods (Read/Write/Close/LocalAddr/RemoteAddr/SetDeadline),
// net/url.URL.Redacted, flag.BoolFunc.
// Expected: 0 violations.
package main

import (
	"flag"
	"net"
	"net/url"
)

func main() {
	// net.LookupAddr.
	names, err := net.LookupAddr("127.0.0.1")
	_, _ = names, err

	// net.LookupSRV.
	cname, addrs, err2 := net.LookupSRV("http", "tcp", "example.com")
	_, _, _ = cname, addrs, err2

	// net.ParseMAC.
	hw, err3 := net.ParseMAC("00:1A:2B:3C:4D:5E")
	_, _ = hw, err3

	// net.Interfaces / InterfaceAddrs / InterfaceByName.
	ifaces, err4 := net.Interfaces()
	_ = ifaces
	_ = err4
	addrs2, err5 := net.InterfaceAddrs()
	_, _ = addrs2, err5
	iface, err6 := net.InterfaceByName("lo")
	_, _ = iface, err6

	// net.IP methods.
	ip := net.ParseIP("127.0.0.1")
	if ip != nil {
		_ = ip.String()
		_ = ip.IsLoopback()
		_ = ip.IsGlobalUnicast()
		_ = ip.IsMulticast()
		_ = ip.IsLinkLocalUnicast()
		_ = ip.IsUnspecified()
		_ = ip.Equal(ip)
		_ = ip.To4()
		_ = ip.To16()
	}

	// net.ParseCIDR + IPNet.Contains.
	_, ipnet, err7 := net.ParseCIDR("192.168.0.0/24")
	_ = err7
	if ipnet != nil {
		_ = ipnet.Contains(net.ParseIP("192.168.0.1"))
	}

	// net.Conn methods via Dial.
	conn, err8 := net.Dial("tcp", "127.0.0.1:80")
	_ = err8
	if conn != nil {
		buf := make([]byte, 8)
		n, _ := conn.Read(buf)
		_ = n
		_, _ = conn.Write([]byte("hello"))
		_ = conn.LocalAddr()
		_ = conn.RemoteAddr()
		_ = conn.Close()
	}

	// net/url.URL.Redacted (Go 1.15).
	u, _ := url.Parse("http://user:secret@example.com/path")
	if u != nil {
		_ = u.Redacted()
	}

	// flag.BoolFunc (Go 1.20).
	flag.BoolFunc("verbose", "enable verbose output", func(s string) error {
		return nil
	})
}
