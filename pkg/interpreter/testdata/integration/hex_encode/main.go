// hex_encode verifies that encoding/hex intercepts work (#81).
//
// Expected: 0 violations.
package main

import "encoding/hex"

func main() {
	// EncodeToString produces a lowercase hex string.
	data := []byte{0xde, 0xad, 0xbe, 0xef}
	s := hex.EncodeToString(data)
	_ = s // "deadbeef"

	// DecodeString parses a hex string back to bytes.
	b, err := hex.DecodeString("deadbeef")
	_ = b   // {0xde, 0xad, 0xbe, 0xef}
	_ = err // nil

	// DecodeString with an invalid hex string returns an error.
	_, err2 := hex.DecodeString("zz")
	_ = err2 // non-nil

	// EncodedLen / DecodedLen.
	n := hex.EncodedLen(4)
	_ = n // 8

	m := hex.DecodedLen(8)
	_ = m // 4
}
