// crypto_ssh exercises golang.org/x/crypto/ssh intercepts (issue #196).
// Expected: 0 violations.
package main

import (
	"golang.org/x/crypto/ssh"
)

func main() {
	// Build an ssh.ClientConfig (struct literal — no intercept needed).
	config := &ssh.ClientConfig{
		User: "alice",
		Auth: []ssh.AuthMethod{
			ssh.Password("secret"),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}
	_ = config

	// FingerprintSHA256 — opaque string
	// (Would need a real PublicKey object; test the intercept path via opaque.)

	// ParsePrivateKey — (Signer, error) — invalid key just exercises intercept
	_, err := ssh.ParsePrivateKey([]byte("not-a-key"))
	_ = err

	// ParsePublicKey — (PublicKey, error)
	_, err2 := ssh.ParsePublicKey([]byte("not-a-key"))
	_ = err2

	// ParseAuthorizedKey — (PublicKey, comment string, options []string, rest []byte, error)
	_, _, _, _, err3 := ssh.ParseAuthorizedKey([]byte("not-a-key"))
	_ = err3

	// MarshalAuthorizedKey — []byte
	// Requires a PublicKey; use opaque path.
	_ = ssh.MarshalAuthorizedKey

	// InsecureIgnoreHostKey returns HostKeyCallback (opaque func)
	cb := ssh.InsecureIgnoreHostKey()
	_ = cb

	// Dial would connect to a real host; just verify the intercept path.
	// We don't actually call Dial because it would block on network.
}
