// zlib_compress verifies that compress/zlib intercepts work (#91).
//
// Expected: 0 violations.
package main

import (
	"bytes"
	"compress/zlib"
)

func main() {
	var buf bytes.Buffer

	// zlib.NewWriter returns *Writer (single value).
	w := zlib.NewWriter(&buf)

	// Write compresses data.
	n, err2 := w.Write([]byte("hello, zlib"))
	_ = n
	_ = err2

	// Flush and Close.
	_ = w.Flush()
	_ = w.Close()

	// zlib.NewWriterLevel returns (*Writer, error).
	w2, err3 := zlib.NewWriterLevel(&buf, zlib.BestSpeed)
	_ = w2
	_ = err3

	// zlib.NewReader returns (io.ReadCloser, error).
	r, err4 := zlib.NewReader(&buf)
	_ = err4 // nil

	// Read decompressed data.
	out := make([]byte, 64)
	n2, _ := r.Read(out)
	_ = n2

	// Close the reader.
	_ = r.Close()

	// zlib.NewReaderDict returns (io.ReadCloser, error).
	r2, err5 := zlib.NewReaderDict(&buf, []byte("dict"))
	_ = r2
	_ = err5
}
