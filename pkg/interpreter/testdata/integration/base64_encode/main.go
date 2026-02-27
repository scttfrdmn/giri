// base64_encode verifies that encoding/base64 intercepts work (#81).
//
// Expected: 0 violations.
package main

import "encoding/base64"

func main() {
	// EncodeToString encodes bytes to base64 string.
	data := []byte("hello world")
	s := base64.StdEncoding.EncodeToString(data)
	_ = s // "aGVsbG8gd29ybGQ="

	// DecodeString decodes a base64 string back to bytes.
	b, err := base64.StdEncoding.DecodeString(s)
	_ = b   // []byte("hello world")
	_ = err // nil

	// Invalid base64 returns an error.
	_, err2 := base64.StdEncoding.DecodeString("not-valid-base64!!!")
	_ = err2

	// URLEncoding variant.
	s2 := base64.URLEncoding.EncodeToString([]byte("test"))
	_ = s2

	// EncodedLen / DecodedLen.
	n := base64.StdEncoding.EncodedLen(9)
	_ = n // 12

	m := base64.StdEncoding.DecodedLen(12)
	_ = m // 9
}
