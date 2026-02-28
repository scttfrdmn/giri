// tls_dial verifies that crypto/tls intercepts work (#101).
//
// Expected: 0 violations.
package main

import "crypto/tls"

func main() {
	// tls.Dial returns (*Conn, error).
	conn, err := tls.Dial("tcp", "example.com:443", nil)
	_ = err

	// *tls.Conn methods.
	buf := make([]byte, 64)
	n, err2 := conn.Read(buf)
	_ = n
	_ = err2

	msg := []byte("hello tls")
	n2, err3 := conn.Write(msg)
	_ = n2
	_ = err3

	_ = conn.Handshake()
	_ = conn.ConnectionState()
	_ = conn.RemoteAddr()
	_ = conn.LocalAddr()
	_ = conn.Close()

	// tls.LoadX509KeyPair returns (Certificate, error).
	cert, err4 := tls.LoadX509KeyPair("cert.pem", "key.pem")
	_ = cert
	_ = err4

	// tls.NewListener returns (net.Listener, error).
	ln, err5 := tls.Listen("tcp", ":8443", &tls.Config{})
	_ = ln
	_ = err5
}
