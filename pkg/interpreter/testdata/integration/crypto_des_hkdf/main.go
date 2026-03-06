// crypto_des_hkdf verifies that crypto/des, crypto/rc4, crypto/pbkdf2, and
// crypto/hkdf are correctly intercepted.
//
// Expected: 0 violations.
package main

import (
	"crypto/des"
	"crypto/hkdf"
	"crypto/pbkdf2"
	"crypto/rc4"
	"crypto/sha256"
)

func main() {
	// crypto/des: NewCipher.
	block, err := des.NewCipher(make([]byte, 8))
	_ = err
	if block == nil {
		var s []int
		_ = s[0] // canary: block must be non-nil
	}

	// crypto/des: NewTripleDESCipher.
	block3, err2 := des.NewTripleDESCipher(make([]byte, 24))
	_ = err2
	_ = block3

	// crypto/rc4: NewCipher.
	cipher, err3 := rc4.NewCipher(make([]byte, 16))
	_ = err3
	if cipher == nil {
		var s []int
		_ = s[0] // canary: cipher must be non-nil
	}

	// crypto/hkdf: Extract + Expand.
	prk, err4 := hkdf.Extract(sha256.New, []byte("secret"), []byte("salt"))
	_ = err4
	_ = prk

	key, err5 := hkdf.Key(sha256.New, []byte("secret"), []byte("salt"), "info", 32)
	_ = err5
	_ = key

	// crypto/pbkdf2: Key.
	dk, err6 := pbkdf2.Key(sha256.New, "password", []byte("salt"), 4096, 32)
	_ = err6
	_ = dk
}
