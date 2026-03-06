// crypto_sha3_hpke verifies that crypto/sha3, crypto/hpke, crypto/mlkem, and
// crypto/fips140 are correctly intercepted.
//
// Expected: 0 violations.
package main

import (
	"crypto/fips140"
	"crypto/hpke"
	"crypto/mlkem"
	"crypto/sha3"
)

func main() {
	// crypto/sha3: New256 constructor and Write/Sum methods.
	h := sha3.New256()
	if h == nil {
		var s []int
		_ = s[0] // canary: hash must be non-nil
	}

	// crypto/sha3: SumSHAKE256 convenience function.
	out := sha3.SumSHAKE256([]byte("hello"), 32)
	_ = out

	// crypto/sha3: SHAKE constructor.
	shake := sha3.NewSHAKE256()
	if shake == nil {
		var s []int
		_ = s[0] // canary: shake must be non-nil
	}

	// crypto/hpke: KEM/KDF/AEAD constructors.
	kem := hpke.DHKEM(nil)
	if kem == nil {
		var s []int
		_ = s[0] // canary: kem must be non-nil
	}

	kdf := hpke.HKDFSHA256()
	_ = kdf

	aead := hpke.AES256GCM()
	_ = aead

	// crypto/hpke: Seal.
	ct, err := hpke.Seal(nil, kdf, aead, []byte("info"), []byte("plaintext"))
	_ = ct
	_ = err

	// crypto/mlkem: GenerateKey768.
	dk, err2 := mlkem.GenerateKey768()
	_ = err2
	if dk == nil {
		var s []int
		_ = s[0] // canary: decap key must be non-nil
	}

	// crypto/fips140: Enabled / Enforced.
	enabled := fips140.Enabled()
	_ = enabled

	enforced := fips140.Enforced()
	_ = enforced
}
