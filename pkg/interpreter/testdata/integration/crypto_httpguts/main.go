// crypto_httpguts verifies that crypto (top-level), testing/cryptotest,
// and golang.org/x/net/html/charset and golang.org/x/net/http/httpguts
// calls are correctly intercepted.
//
// Expected: 0 violations.
package main

import (
	"crypto"
	_ "crypto/sha256" // register SHA-256

	"golang.org/x/net/http/httpguts"
)

func main() {
	// crypto top-level: Available.
	avail := crypto.SHA256.Available()
	_ = avail

	// crypto top-level: HashFunc string.
	s := crypto.SHA256.String()
	_ = s

	// crypto top-level: New (returns a hash.Hash).
	h := crypto.SHA256.New()
	_ = h

	// httpguts: HeaderValuesContainsToken.
	vals := []string{"keep-alive", "upgrade"}
	ok := httpguts.HeaderValuesContainsToken(vals, "upgrade")
	_ = ok

	// httpguts: ValidHeaderFieldName.
	valid := httpguts.ValidHeaderFieldName("Content-Type")
	_ = valid

	// httpguts: PunycodeHostPort.
	host, err := httpguts.PunycodeHostPort("example.com:80")
	_ = host
	_ = err
}
