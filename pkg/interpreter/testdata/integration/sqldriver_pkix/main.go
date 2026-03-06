// sqldriver_pkix verifies that database/sql/driver and crypto/x509/pkix are
// correctly intercepted.
//
// Expected: 0 violations.
package main

import (
	"crypto/x509/pkix"
	"database/sql/driver"
	"math/big"
)

func main() {
	// database/sql/driver: IsValue.
	ok := driver.IsValue(42)
	_ = ok

	ok2 := driver.IsValue("hello")
	_ = ok2

	// crypto/x509/pkix: Name.String.
	name := pkix.Name{
		CommonName:   "example.com",
		Organization: []string{"Example Org"},
	}
	s := name.String()
	_ = s

	// crypto/x509/pkix: Name.ToRDNSequence.
	rdn := name.ToRDNSequence()
	_ = rdn

	// crypto/x509/pkix: AlgorithmIdentifier (struct literal).
	alg := pkix.AlgorithmIdentifier{}
	_ = alg

	// crypto/x509/pkix: Extension.
	ext := pkix.Extension{
		Value: []byte{1, 2, 3},
	}
	_ = ext

	// crypto/x509/pkix: RevokedCertificate.
	rev := pkix.RevokedCertificate{
		SerialNumber: big.NewInt(123),
	}
	_ = rev
}
