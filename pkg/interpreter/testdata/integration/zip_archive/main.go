// zip_archive exercises archive/zip and archive/tar intercepts (#110).
//
// Expected: 0 violations.
package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
)

func useZip() {
	var buf bytes.Buffer

	// zip.NewWriter returns *Writer.
	w := zip.NewWriter(&buf)

	// Create an entry.
	_, _ = w.Create("hello.txt")

	// Close flushes the ZIP central directory.
	_ = w.Close()

	// zip.NewReader returns (*Reader, error).
	r, _ := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	_ = r
}

func useTar() {
	var buf bytes.Buffer

	// tar.NewWriter returns *Writer.
	tw := tar.NewWriter(&buf)

	// WriteHeader writes a tar header.
	_ = tw.WriteHeader(&tar.Header{
		Name: "hello.txt",
		Size: 5,
	})

	// Write file contents.
	_, _ = tw.Write([]byte("hello"))

	// Flush and Close.
	_ = tw.Flush()
	_ = tw.Close()

	// tar.NewReader returns *Reader.
	tr := tar.NewReader(bytes.NewReader(buf.Bytes()))

	// Next reads the next header.
	hdr, _ := tr.Next()
	_ = hdr

	// Read file contents.
	out := make([]byte, 5)
	_, _ = tr.Read(out)
}

func main() {
	useZip()
	useTar()
}
