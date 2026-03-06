// blake2_ed25519 exercises golang.org/x/crypto/blake2b, blake2s, and ed25519
// intercepts (issue #198). Expected: 0 violations.
package main

import (
	"crypto/rand"

	"golang.org/x/crypto/blake2b"
	"golang.org/x/crypto/blake2s"
	"golang.org/x/crypto/ed25519"
)

func main() {
	// blake2b: constructors
	h256, err := blake2b.New256(nil)
	_ = err
	if h256 != nil {
		h256.Write([]byte("hello"))
		_ = h256.Sum(nil)
		_ = h256.Size()
		h256.Reset()
	}

	h512, err2 := blake2b.New512(nil)
	_ = err2
	_ = h512

	// blake2b: Sum functions return [N]byte arrays
	sum := blake2b.Sum256([]byte("data"))
	_ = sum

	sum512 := blake2b.Sum512([]byte("data"))
	_ = sum512

	// blake2b: NewXOF
	xof, err3 := blake2b.NewXOF(32, nil)
	_ = err3
	_ = xof

	// blake2s: constructors
	hs256, err4 := blake2s.New256(nil)
	_ = err4
	_ = hs256

	ssum := blake2s.Sum256([]byte("data"))
	_ = ssum

	// ed25519: GenerateKey
	pub, priv, err5 := ed25519.GenerateKey(rand.Reader)
	_ = err5

	// ed25519: Sign
	sig := ed25519.Sign(priv, []byte("message"))
	_ = sig

	// ed25519: Verify
	ok := ed25519.Verify(pub, []byte("message"), sig)
	_ = ok
}
