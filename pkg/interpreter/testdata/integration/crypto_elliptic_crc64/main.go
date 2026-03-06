// crypto_elliptic_crc64 verifies that crypto/dsa, crypto/elliptic, and
// hash/crc64 are correctly intercepted.
//
// Expected: 0 violations.
package main

import (
	"crypto/dsa"
	"crypto/elliptic"
	"crypto/rand"
	"hash/crc64"
)

func main() {
	// crypto/elliptic: P256 curve.
	curve := elliptic.P256()
	if curve == nil {
		var s []int
		_ = s[0] // canary: curve must be non-nil
	}

	// crypto/elliptic: GenerateKey.
	_, _, _, err := elliptic.GenerateKey(curve, rand.Reader)
	_ = err

	// crypto/elliptic: Marshal.
	marshaled := elliptic.Marshal(curve, nil, nil)
	_ = marshaled

	// crypto/dsa: GenerateParameters.
	var params dsa.Parameters
	err2 := dsa.GenerateParameters(&params, rand.Reader, dsa.L1024N160)
	_ = err2

	// hash/crc64: MakeTable + New + Checksum.
	tab := crc64.MakeTable(crc64.ECMA)
	if tab == nil {
		var s []int
		_ = s[0] // canary: table must be non-nil
	}

	h := crc64.New(tab)
	if h == nil {
		var s []int
		_ = s[0] // canary: hash must be non-nil
	}

	sum := crc64.Checksum([]byte("hello"), tab)
	_ = sum
}
