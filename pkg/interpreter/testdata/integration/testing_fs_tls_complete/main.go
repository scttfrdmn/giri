// testing_fs_tls_complete exercises additions from v0.89.0:
// testing: (*T).Context (Go 1.21+), Deadline (Go 1.21+), Chdir (Go 1.24+),
//          ArtifactDir (Go 1.25+), Attr (Go 1.25+), Output (Go 1.21+);
// io/fs: FormatDirEntry (Go 1.21+), FormatFileInfo (Go 1.21+),
//        ReadLink (Go 1.25+);
// crypto/tls: (*Conn).CloseWrite, CipherSuiteName, VersionName (Go 1.23+);
// net/url: (*URL).JoinPath (Go 1.19+) method form returning *URL.
// Expected: 0 violations.
package main

import (
	"crypto/tls"
	"io/fs"
	"net/url"
	"testing"
)

func main() {
	// testing: newer T methods — use a sentinel *testing.T value.
	t := &testing.T{}

	// Context (Go 1.21+).
	ctx := t.Context()
	_ = ctx

	// Deadline (Go 1.21+).
	deadline, ok := t.Deadline()
	_, _ = deadline, ok

	// Chdir (Go 1.24+) — noop in tests.
	t.Chdir("/tmp")

	// ArtifactDir (Go 1.25+).
	dir := t.ArtifactDir()
	_ = dir

	// Attr (Go 1.25+).
	t.Attr("key", "value")

	// Output (Go 1.21+) — returns io.Writer.
	w := t.Output()
	_ = w

	// io/fs: FormatDirEntry and FormatFileInfo (Go 1.21+).
	// These operate on interface values — pass nil (the functions handle that).
	var de fs.DirEntry
	s1 := fs.FormatDirEntry(de)
	_ = s1

	var fi fs.FileInfo
	s2 := fs.FormatFileInfo(fi)
	_ = s2

	// io/fs: ReadLink (Go 1.25+).
	target, err := fs.ReadLink(nil, "symlink")
	_, _ = target, err

	// crypto/tls: CloseWrite on a *Conn value.
	conn := &tls.Conn{}
	_ = conn.CloseWrite()

	// crypto/tls: CipherSuiteName and VersionName.
	name := tls.CipherSuiteName(tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256)
	_ = name
	ver := tls.VersionName(tls.VersionTLS13)
	_ = ver

	// net/url: (*URL).JoinPath method form (Go 1.19+) — returns *URL.
	base, _ := url.Parse("https://example.com/base")
	u2 := base.JoinPath("foo", "bar")
	_ = u2.String()
}
