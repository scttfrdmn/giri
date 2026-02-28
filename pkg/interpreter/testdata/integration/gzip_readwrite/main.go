// gzip_readwrite verifies that compress/gzip intercepts work (#91).
//
// Expected: 0 violations.
package main

import (
	"bytes"
	"compress/gzip"
)

func main() {
	// gzip.NewWriter wraps an io.Writer.
	var buf bytes.Buffer
	w := gzip.NewWriter(&buf)
	_ = w

	// Write compressed data.
	n, err := w.Write([]byte("hello, world"))
	_ = n
	_ = err

	// Flush ensures buffered data is written.
	_ = w.Flush()

	// Close finalises the gzip stream.
	_ = w.Close()

	// gzip.NewWriterLevel returns (*Writer, error).
	w2, err2 := gzip.NewWriterLevel(&buf, gzip.BestCompression)
	_ = w2
	_ = err2

	// gzip.NewReader wraps an io.Reader.
	r, err3 := gzip.NewReader(&buf)
	_ = err3 // nil
	_ = r

	// Read decompresses data.
	decompressed := make([]byte, 64)
	n2, _ := r.Read(decompressed)
	_ = n2

	// Close the reader.
	_ = r.Close()
}
