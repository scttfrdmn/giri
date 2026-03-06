// crypto_stream exercises x/crypto/scrypt, chacha20, xts, and salsa20
// intercepts (issue #201). Expected: 0 violations.
package main

import (
	"crypto/aes"

	"golang.org/x/crypto/chacha20"
	"golang.org/x/crypto/salsa20"
	"golang.org/x/crypto/scrypt"
	"golang.org/x/crypto/xts"
)

func main() {
	// scrypt: Key derivation
	dk, err := scrypt.Key([]byte("password"), []byte("saltsalt"), 1024, 8, 1, 32)
	_ = dk
	_ = err

	// chacha20: NewUnauthenticatedCipher
	key := make([]byte, 32)
	nonce := make([]byte, 12)
	c, err2 := chacha20.NewUnauthenticatedCipher(key, nonce)
	_ = err2
	if c != nil {
		src := []byte("hello world")
		dst := make([]byte, len(src))
		c.XORKeyStream(dst, src)
		c.SetCounter(0)
	}

	// chacha20: HChaCha20
	nonce16 := make([]byte, 16)
	sub, err3 := chacha20.HChaCha20(key, nonce16)
	_, _ = sub, err3

	// xts: NewCipher
	xtsKey := make([]byte, 64) // AES-256-XTS needs 64-byte key
	xc, err4 := xts.NewCipher(aes.NewCipher, xtsKey)
	_ = err4
	if xc != nil {
		pt := make([]byte, 16)
		ct := make([]byte, 16)
		xc.Encrypt(ct, pt, 0)
		xc.Decrypt(pt, ct, 0)
	}

	// salsa20: XORKeyStream
	var salsakey [32]byte
	salsa20.XORKeyStream(key, key, []byte("nonce678"), &salsakey)
}
