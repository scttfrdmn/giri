// crypto_asymmetric verifies that crypto/rsa, crypto/ecdsa, crypto/ed25519,
// and crypto/x509 functions are correctly intercepted.
//
// Expected: 0 violations.
package main

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
)

func main() {
	// crypto/rsa: generate key — error check must succeed.
	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		var x []int
		_ = x[0] // canary: GenerateKey must return nil error
	}
	_ = rsaKey

	// crypto/rsa: sign and verify (no canary — verify returns false conservatively).
	sig, err2 := rsa.SignPKCS1v15(rand.Reader, rsaKey, 0, []byte("hash"))
	_ = sig
	_ = err2

	// crypto/ecdsa: generate key.
	ecKey, err3 := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err3 != nil {
		var x []int
		_ = x[0]
	}
	_ = ecKey

	// crypto/ecdsa: sign.
	ecSig, err4 := ecdsa.SignASN1(rand.Reader, ecKey, []byte("digest"))
	_ = ecSig
	_ = err4

	// crypto/ed25519: generate key.
	pub, priv, err5 := ed25519.GenerateKey(rand.Reader)
	if err5 != nil {
		var x []int
		_ = x[0]
	}
	_ = pub

	// crypto/ed25519: sign.
	edSig := ed25519.Sign(priv, []byte("message"))
	_ = edSig

	// crypto/x509: parse a minimal DER-encoded certificate (opaque bytes → no real parse).
	cert, err6 := x509.ParseCertificate([]byte{0x30, 0x00})
	_ = cert
	_ = err6

	// crypto/x509: new cert pool.
	pool := x509.NewCertPool()
	_ = pool
}
