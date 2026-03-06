// crypto_subtle_maphash verifies that crypto/subtle, hash/maphash,
// regexp/syntax, and unique are correctly intercepted.
//
// Expected: 0 violations.
package main

import (
	"crypto/subtle"
	"hash/maphash"
	"regexp/syntax"
)

func main() {
	// crypto/subtle: ConstantTimeCompare.
	a := []byte("hello")
	b := []byte("world")
	eq := subtle.ConstantTimeCompare(a, b)
	_ = eq

	// crypto/subtle: XORBytes.
	dst := make([]byte, 5)
	n := subtle.XORBytes(dst, a, b)
	if n < 0 {
		var s []int
		_ = s[0] // canary: XORBytes result must be >= 0
	}

	// crypto/subtle: ConstantTimeLessOrEq.
	lt := subtle.ConstantTimeLessOrEq(1, 2)
	_ = lt

	// hash/maphash: MakeSeed + Bytes/String.
	seed := maphash.MakeSeed()
	h1 := maphash.Bytes(seed, []byte("test"))
	h2 := maphash.String(seed, "test")
	_ = h1
	_ = h2

	// hash/maphash: Hash methods.
	var h maphash.Hash
	h.SetSeed(seed)
	h.WriteString("hello")
	s64 := h.Sum64()
	_ = s64
	h.Reset()

	// regexp/syntax: Parse.
	re, err := syntax.Parse(`\d+`, syntax.Perl)
	_ = err
	if re == nil {
		var s []int
		_ = s[0] // canary: parsed regexp must be non-nil
	}
}
