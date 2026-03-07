// io_bufio_context_http_complete exercises additions from v0.86.0:
// io: (*SectionReader).Outer (Go 1.22);
// bufio: (*Writer).ReadFrom, (*Reader).WriteTo;
// context: WithoutCancel (Go 1.21);
// net/http: (*Request).PathValue/SetPathValue (Go 1.22),
//           (*ResponseController).EnableFullDuplex (Go 1.23);
// regexp: LiteralPrefix;
// os: (*File).ReadFrom.
// Expected: 0 violations.
package main

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
)

// noopRW implements http.ResponseWriter for ResponseController tests.
type noopRW struct{}

func (noopRW) Header() http.Header       { return http.Header{} }
func (noopRW) Write([]byte) (int, error) { return 0, nil }
func (noopRW) WriteHeader(int)           {}

func main() {
	// io: (*SectionReader).Outer (Go 1.22) — returns underlying ReaderAt + offsets.
	sr := io.NewSectionReader(bytes.NewReader([]byte("hello world")), 2, 5)
	r, off, n := sr.Outer()
	_, _, _ = r, off, n

	// bufio: (*Writer).ReadFrom — implements io.ReaderFrom.
	var dst bytes.Buffer
	bw := bufio.NewWriter(&dst)
	_, _ = bw.ReadFrom(strings.NewReader("content"))

	// bufio: (*Reader).WriteTo — implements io.WriterTo.
	br := bufio.NewReader(strings.NewReader("content"))
	var wDst bytes.Buffer
	_, _ = br.WriteTo(&wDst)

	// context: WithoutCancel (Go 1.21).
	ctx := context.WithoutCancel(context.Background())
	_ = ctx

	// net/http: (*Request).PathValue and SetPathValue (Go 1.22).
	req, _ := http.NewRequest("GET", "http://example.com/users/42", nil)
	id := req.PathValue("id")
	_ = id
	req.SetPathValue("id", "42")

	// net/http: (*ResponseController).EnableFullDuplex (Go 1.23).
	rc := http.NewResponseController(noopRW{})
	_ = rc.EnableFullDuplex()

	// regexp: LiteralPrefix.
	re := regexp.MustCompile(`hello`)
	prefix, complete := re.LiteralPrefix()
	_, _ = prefix, complete

	// os: (*File).ReadFrom (sendfile, Go 1.16+).
	f, err := os.CreateTemp("", "giri-test-*")
	if err == nil {
		_, _ = f.ReadFrom(strings.NewReader("data"))
		f.Close()
		os.Remove(f.Name())
	}
}
