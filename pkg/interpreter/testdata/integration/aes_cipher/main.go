// aes_cipher exercises crypto/aes, crypto/cipher, and crypto/hmac intercepts (#112).
//
// Expected: 0 violations.
package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/sha256"
)

func useAESGCM(key, nonce, plaintext []byte) {
	// crypto/aes.NewCipher returns (cipher.Block, error).
	block, err := aes.NewCipher(key)
	if err != nil {
		return
	}

	// crypto/cipher.NewGCM returns (cipher.AEAD, error).
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return
	}

	// Seal encrypts.
	ciphertext := gcm.Seal(nil, nonce, plaintext, nil)

	// Open decrypts.
	_, _ = gcm.Open(nil, nonce, ciphertext, nil)

	// NonceSize and Overhead.
	_ = gcm.NonceSize()
	_ = gcm.Overhead()
}

func useAESCTR(key, iv, plaintext []byte) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return
	}

	// crypto/cipher.NewCTR returns a Stream.
	stream := cipher.NewCTR(block, iv)

	// XORKeyStream encrypts in place.
	dst := make([]byte, len(plaintext))
	stream.XORKeyStream(dst, plaintext)
}

func useHMAC(key, message []byte) {
	// crypto/hmac.New returns a hash.Hash.
	mac := hmac.New(sha256.New, key)

	// Write the message.
	_, _ = mac.Write(message)

	// Sum appends the HMAC.
	sum := mac.Sum(nil)

	// hmac.Equal compares two MACs in constant time.
	_ = hmac.Equal(sum, sum)

	mac.Reset()
	_ = mac.Size()
	_ = mac.BlockSize()
}

func main() {
	key := make([]byte, 32)   // AES-256 key
	nonce := make([]byte, 12) // GCM nonce
	iv := make([]byte, 16)    // AES block size
	msg := []byte("hello, giri")

	useAESGCM(key, nonce, msg)
	useAESCTR(key, iv, msg)
	useHMAC(key, msg)
}
