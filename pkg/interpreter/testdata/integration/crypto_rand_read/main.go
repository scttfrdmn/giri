// crypto_rand_read verifies that crypto/rand intercepts work (#82).
//
// Expected: 0 violations.
package main

import "crypto/rand"

func main() {
	// rand.Read fills the slice and returns (n, nil).
	buf := make([]byte, 32)
	n, err := rand.Read(buf)
	_ = n   // 32
	_ = err // nil

	// Smaller buffer.
	token := make([]byte, 16)
	n2, err2 := rand.Read(token)
	_ = n2   // 16
	_ = err2 // nil

	// Zero-length buffer is a no-op.
	empty := make([]byte, 0)
	n3, err3 := rand.Read(empty)
	_ = n3   // 0
	_ = err3 // nil
}
