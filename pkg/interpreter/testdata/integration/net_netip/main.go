// net_netip verifies that net/netip.* functions are correctly intercepted
// (Go 1.18+).
//
// Exercises ParseAddr, MustParseAddr, AddrPortFrom, ParsePrefix, IsValid,
// and various predicate methods.
//
// Expected: 0 violations.
package main

import "net/netip"

func main() {
	// ParseAddr returns (Addr, error).
	addr, err := netip.ParseAddr("192.0.2.1")
	_ = err
	_ = addr

	// MustParseAddr returns Addr (panics on invalid input — valid here).
	addr2 := netip.MustParseAddr("::1")
	_ = addr2

	// IPv4Unspecified, IPv6Unspecified constructors.
	_ = netip.IPv4Unspecified()
	_ = netip.IPv6Unspecified()

	// AddrFrom4: build from 4-byte array.
	addr3 := netip.AddrFrom4([4]byte{10, 0, 0, 1})
	_ = addr3

	// AddrPortFrom: combine address and port.
	ap := netip.AddrPortFrom(netip.MustParseAddr("10.0.0.1"), 8080)
	_ = ap

	// ParsePrefix.
	prefix, err2 := netip.ParsePrefix("10.0.0.0/8")
	_ = prefix
	_ = err2

	// PrefixFrom.
	p2 := netip.PrefixFrom(netip.MustParseAddr("192.168.0.0"), 16)
	_ = p2

	// ParseAddrPort.
	addrPort, err3 := netip.ParseAddrPort("192.0.2.1:443")
	_ = addrPort
	_ = err3
}
