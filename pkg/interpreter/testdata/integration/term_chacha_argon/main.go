// term_chacha_argon exercises golang.org/x/term, chacha20poly1305, and argon2
// intercepts (issue #195). Expected: 0 violations.
package main

import (
	"golang.org/x/crypto/argon2"
	"golang.org/x/crypto/chacha20poly1305"
	"golang.org/x/term"
)

func main() {
	// term: IsTerminal — should return false in interpreter
	isTerm := term.IsTerminal(0)
	_ = isTerm

	// term: GetSize — returns (80, 24, nil)
	w, h, err := term.GetSize(0)
	_, _ = w, h
	_ = err

	// term: ReadPassword — returns ([]byte{}, nil)
	passwd, err2 := term.ReadPassword(0)
	_ = passwd
	_ = err2

	// chacha20poly1305: New — (cipher.AEAD, error)
	key := make([]byte, chacha20poly1305.KeySize)
	aead, err3 := chacha20poly1305.New(key)
	_ = err3
	_ = aead

	// chacha20poly1305: NewX
	aeadX, err4 := chacha20poly1305.NewX(key)
	_ = err4
	_ = aeadX

	// argon2: Key — returns opaque []byte
	password := []byte("password")
	salt := []byte("saltsalt")
	derived := argon2.Key(password, salt, 1, 64*1024, 4, 32)
	_ = derived

	// argon2: IDKey
	derived2 := argon2.IDKey(password, salt, 1, 64*1024, 4, 32)
	_ = derived2
}
