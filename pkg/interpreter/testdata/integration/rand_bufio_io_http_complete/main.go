// rand_bufio_io_http_complete exercises additions from v0.82.0:
// crypto/rand.Text (Go 1.24 random base32 text);
// bufio split functions: ScanLines, ScanWords, ScanBytes, ScanRunes;
// io.SectionReader methods (Read/ReadAt/Seek/Size);
// io.OffsetWriter methods (Write/WriteAt/Seek);
// io.Pipe + PipeReader/PipeWriter Read/Write/Close;
// net/http: FileServer, (*Server).Close, (*Server).Shutdown, (*Server).Serve.
// Expected: 0 violations.
package main

import (
	"bufio"
	"bytes"
	cryptorand "crypto/rand"
	"context"
	"io"
	"net"
	"net/http"
)

// writerAt adapts /dev/null to io.WriterAt for io.NewOffsetWriter.
type writerAt struct{}

func (writerAt) WriteAt(p []byte, _ int64) (int, error) { return len(p), nil }

func main() {
	// crypto/rand.Text (Go 1.24).
	_ = cryptorand.Text()

	// bufio split functions (data []byte, atEOF bool) → (advance, token, err).
	adv, tok, err := bufio.ScanLines(nil, true)
	_, _, _ = adv, tok, err
	_, _, _ = bufio.ScanWords(nil, true)
	_, _, _ = bufio.ScanBytes(nil, true)
	_, _, _ = bufio.ScanRunes(nil, true)

	// io.SectionReader methods.
	sr := io.NewSectionReader(bytes.NewReader([]byte("hello world")), 0, 11)
	buf := make([]byte, 4)
	_, _ = sr.Read(buf)
	_, _ = sr.ReadAt(buf, 0)
	_, _ = sr.Seek(0, io.SeekStart)
	_ = sr.Size()

	// io.OffsetWriter methods.
	ow := io.NewOffsetWriter(writerAt{}, 0)
	_, _ = ow.Write(buf)
	_, _ = ow.WriteAt(buf, 0)
	_, _ = ow.Seek(0, io.SeekStart)

	// io.Pipe: PipeReader.Read/Close, PipeWriter.Write/Close.
	pr, pw := io.Pipe()
	_, _ = pr.Read(buf)
	_ = pr.Close()
	_, _ = pw.Write(buf)
	_ = pw.Close()

	// net/http FileServer.
	_ = http.FileServer(http.Dir("."))

	// net/http *Server lifecycle.
	srv := &http.Server{}
	l, _ := net.Listen("tcp", ":0")
	_ = srv.Serve(l)
	_ = srv.Shutdown(context.Background())
	_ = srv.Close()
}
