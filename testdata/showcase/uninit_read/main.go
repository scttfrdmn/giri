// Showcase: read from uninitialized memory (Giri --track-init mode)
//
// Allocates a heap value with new() and reads it before any explicit write.
// Go's specification guarantees zero-initialization, so this is not a
// language-level bug. However, Giri's TrackInit mode tracks EXPLICIT writes
// and reports reads on bytes that the program never wrote — useful for:
//
//   - Auditing security-critical code: ensuring secrets are explicitly zeroed
//     rather than relying on allocator behaviour.
//   - Detecting logic bugs where a field is read before initialization in code
//     that does NOT rely on zero values.
//   - Verifying that sensitive buffers are cleared before use.
//
// What each tool reports:
//
//	go vet:              PASS — vet does not track write-before-read
//	go test -race:       PASS — no concurrent access
//	giri --track-init:   FAIL — uninitialized read: byte at offset 0 never written
//
// This mode is opt-in because Go's zero-init guarantee means it is not
// a bug in general — but it is valuable for security audits.
package main

// AuthToken holds a sensitive bearer token.
type AuthToken struct {
	value [32]byte
}

func newToken() *AuthToken {
	// Allocates and zero-initializes per Go spec.
	// Bug: the caller reads value[0] before explicitly setting the token bytes.
	return new(AuthToken)
}

func isNullToken(t *AuthToken) bool {
	return t.value[0] == 0 // read before any explicit write
}

func main() {
	tok := newToken()
	// isNullToken reads tok.value[0] which was never explicitly written.
	// With TrackInit=true, Giri reports this as an uninitialized read.
	if isNullToken(tok) {
		_ = "token is unset"
	}
}
