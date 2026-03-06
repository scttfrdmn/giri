//go:build tools

// Package tools anchors golang.org/x dependencies used only in testdata
// integration-test programs. Without this file, `go mod tidy` removes these
// packages from go.mod because no main-module Go source directly imports them.
// The build tag "tools" prevents this file from being compiled during normal
// builds while still satisfying go mod tidy's dependency analysis.
package tools

import (
	// x/text packages used in testdata integration tests (v0.63.0+)
	_ "golang.org/x/text/cases"
	_ "golang.org/x/text/language"
	_ "golang.org/x/text/runes"
	_ "golang.org/x/text/transform"
	_ "golang.org/x/text/unicode/norm"
	_ "golang.org/x/text/width"
	// x/text additional packages (v0.64.0+)
	_ "golang.org/x/text/collate"
	_ "golang.org/x/text/encoding"
	_ "golang.org/x/text/encoding/charmap"
	_ "golang.org/x/text/encoding/unicode"
	_ "golang.org/x/text/search"
	// x/text additional packages (v0.66.0+)
	_ "golang.org/x/text/encoding/ianaindex"
	_ "golang.org/x/text/encoding/korean"
	_ "golang.org/x/text/encoding/simplifiedchinese"
	_ "golang.org/x/text/encoding/traditionalchinese"
	// x/text additional packages (v0.65.0+)
	_ "golang.org/x/text/currency"
	_ "golang.org/x/text/encoding/htmlindex"
	_ "golang.org/x/text/encoding/japanese"
	_ "golang.org/x/text/message"
	_ "golang.org/x/text/number"
	_ "golang.org/x/text/secure/bidirule"
	_ "golang.org/x/text/secure/precis"
	_ "golang.org/x/text/unicode/bidi"
	_ "golang.org/x/text/unicode/runenames"
	// x/crypto packages used in testdata integration tests (v0.63.0+)
	_ "golang.org/x/crypto/chacha20"
	_ "golang.org/x/crypto/salsa20"
	_ "golang.org/x/crypto/scrypt"
	_ "golang.org/x/crypto/xts"
	_ "golang.org/x/crypto/argon2"
	_ "golang.org/x/crypto/bcrypt"
	_ "golang.org/x/crypto/blake2b"
	_ "golang.org/x/crypto/blake2s"
	_ "golang.org/x/crypto/chacha20poly1305"
	_ "golang.org/x/crypto/curve25519"
	_ "golang.org/x/crypto/ed25519"
	_ "golang.org/x/crypto/nacl/box"
	_ "golang.org/x/crypto/nacl/secretbox"
	_ "golang.org/x/crypto/poly1305"
	_ "golang.org/x/crypto/ssh"
	// x/term used in testdata integration tests (v0.63.0+)
	_ "golang.org/x/term"
	// x/net additional packages (v0.66.0+)
	_ "golang.org/x/net/dns/dnsmessage"
	_ "golang.org/x/net/http2/hpack"
	_ "golang.org/x/net/trace"
	// x/net packages used in testdata integration tests (v0.61.0+)
	_ "golang.org/x/net/html"
	_ "golang.org/x/net/html/charset"
	_ "golang.org/x/net/http/httpguts"
	_ "golang.org/x/net/idna"
	_ "golang.org/x/net/netutil"
	_ "golang.org/x/net/proxy"
	_ "golang.org/x/net/publicsuffix"
	// x/sys used in testdata integration tests (v0.61.0+)
	_ "golang.org/x/sys/unix"
	// x/sync additional packages (v0.66.0+)
	_ "golang.org/x/sync/syncmap"
	// x/mod packages used in testdata integration tests (v0.61.0+)
	_ "golang.org/x/mod/modfile"
	_ "golang.org/x/mod/module"
	_ "golang.org/x/mod/semver"
)
