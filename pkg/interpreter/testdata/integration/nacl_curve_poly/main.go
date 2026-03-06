// nacl_curve_poly exercises golang.org/x/crypto/nacl/box, nacl/secretbox,
// curve25519, and poly1305 intercepts (issue #197). Expected: 0 violations.
package main

import (
	"crypto/rand"

	"golang.org/x/crypto/curve25519"
	"golang.org/x/crypto/nacl/box"
	"golang.org/x/crypto/nacl/secretbox"
	"golang.org/x/crypto/poly1305"
)

func main() {
	// nacl/box: GenerateKey
	pub, priv, err := box.GenerateKey(rand.Reader)
	_ = err

	// nacl/box: Seal — appends sealed message to nil
	var nonce24 [24]byte
	sealed := box.Seal(nil, []byte("hello"), &nonce24, pub, priv)
	_ = sealed

	// nacl/box: Open
	opened, ok := box.Open(nil, sealed, &nonce24, pub, priv)
	_, _ = opened, ok

	// nacl/box: Precompute
	var shared [32]byte
	box.Precompute(&shared, pub, priv)

	// nacl/secretbox: Seal / Open
	var key32 [32]byte
	ssealed := secretbox.Seal(nil, []byte("world"), &nonce24, &key32)
	sopened, sok := secretbox.Open(nil, ssealed, &nonce24, &key32)
	_, _ = sopened, sok

	// curve25519: X25519
	scalar := make([]byte, 32)
	point := make([]byte, 32)
	result, err2 := curve25519.X25519(scalar, point)
	_, _ = result, err2

	// poly1305: Sum (package-level noop)
	var mac16 [16]byte
	var key32b [32]byte
	poly1305.Sum(&mac16, []byte("msg"), &key32b)

	// poly1305: Verify
	v := poly1305.Verify(&mac16, []byte("msg"), &key32b)
	_ = v

	// poly1305: New → *MAC
	m := poly1305.New(&key32b)
	m.Write([]byte("data"))
	_ = m.Size()
	_ = m.Sum(nil)
}
