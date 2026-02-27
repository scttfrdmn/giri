// net_parse verifies that net utility function intercepts work (#84).
//
// Expected: 0 violations.
package main

import "net"

func main() {
	// SplitHostPort splits "host:port" into components.
	host, port, err := net.SplitHostPort("localhost:8080")
	_ = host // "localhost"
	_ = port // "8080"
	_ = err  // nil

	// SplitHostPort on an invalid address returns an error.
	_, _, err2 := net.SplitHostPort("invalid")
	_ = err2 // non-nil

	// JoinHostPort reassembles host and port.
	addr := net.JoinHostPort("localhost", "8080")
	_ = addr // "localhost:8080"

	// ParseIP parses a valid IPv4 address.
	ip := net.ParseIP("127.0.0.1")
	_ = ip // non-nil

	// ParseIP on an invalid string returns nil.
	ip2 := net.ParseIP("not-an-ip")
	_ = ip2 // nil

	// ParseCIDR parses CIDR notation.
	ip3, network, err3 := net.ParseCIDR("192.168.1.1/24")
	_ = ip3
	_ = network
	_ = err3
}
