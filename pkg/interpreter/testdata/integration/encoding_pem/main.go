// encoding_pem verifies that encoding/pem and encoding/asn1 functions are
// correctly intercepted.
//
// Expected: 0 violations.
package main

import (
	"encoding/asn1"
	"encoding/pem"
)

func main() {
	// encoding/pem: Decode a well-formed PEM block.
	const pemData = `-----BEGIN CERTIFICATE-----
MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEA2a2rwplBQLF29amygykE
-----END CERTIFICATE-----`
	block, rest := pem.Decode([]byte(pemData))
	_ = block
	_ = rest

	// EncodeToMemory on opaque input returns a non-empty result.
	out := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: []byte{0x30, 0x00}})
	if len(out) == 0 {
		var x []int
		_ = x[0] // canary: output must be non-empty
	}

	// encoding/asn1: Marshal a simple integer.
	b, err := asn1.Marshal(42)
	_ = err
	if len(b) == 0 {
		var x []int
		_ = x[0]
	}

	// Unmarshal — no canary needed, just ensure no crash.
	der := []byte{0x02, 0x01, 0x2a} // ASN.1 INTEGER 42
	var val int
	rest2, err2 := asn1.Unmarshal(der, &val)
	_ = rest2
	_ = err2
}
