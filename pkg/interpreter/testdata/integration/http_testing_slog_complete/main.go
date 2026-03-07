// http_testing_slog_complete exercises additions from v0.83.0:
// net/http: FS, NewFileTransport, NewFileTransportFS, AllowQuerySemicolons,
//           TimeoutHandler, MaxBytesHandler;
// testing: Benchmark (package-level runner);
// log/slog: NewLogLogger.
// Expected: 0 violations.
package main

import (
	"io/fs"
	"log/slog"
	"net/http"
	"testing"
	"time"
)

func main() {
	// net/http: filesystem helpers (Go 1.16 / 1.22).
	var fsys fs.FS
	httpFS := http.FS(fsys)
	_ = http.NewFileTransport(httpFS)
	_ = http.NewFileTransportFS(fsys)

	// net/http: middleware factories.
	h := http.NotFoundHandler()
	_ = http.AllowQuerySemicolons(h)
	_ = http.TimeoutHandler(h, time.Second, "timeout")
	_ = http.MaxBytesHandler(h, 1<<20)

	// testing: package-level Benchmark runner.
	// Avoid b.N field access — the interpreter passes an opaque sentinel for *testing.B;
	// field loads on opaque values are safe only for method calls, not FieldAddr.
	result := testing.Benchmark(func(b *testing.B) {
		_ = b
	})
	_ = result

	// log/slog: NewLogLogger bridges a slog Handler to *log.Logger.
	handler := slog.NewTextHandler(nil, nil)
	logger := slog.NewLogLogger(handler, slog.LevelInfo)
	_ = logger
}
