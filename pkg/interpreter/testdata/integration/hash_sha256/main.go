// hash_sha256 verifies that crypto/sha256 (and crypto/md5) intercepts work (#82).
//
// Expected: 0 violations.
package main

import (
	"crypto/md5"
	"crypto/sha256"
)

func main() {
	// sha256.New returns a hash.Hash.
	h := sha256.New()
	_ = h

	// Package-level sha256.Sum256(data) [32]byte.
	digest := sha256.Sum256([]byte("hello"))
	_ = digest

	// md5.New returns a hash.Hash.
	h2 := md5.New()
	_ = h2

	// Package-level md5.Sum(data) [16]byte.
	digest2 := md5.Sum([]byte("hello"))
	_ = digest2

	// Hash size constants.
	_ = sha256.Size   // 32
	_ = sha256.Size224 // 28
	_ = md5.Size      // 16
}
