// bufio_scanner verifies that bufio package intercepts work (#78).
//
// Expected: 0 violations.
package main

import (
	"bufio"
	"strings"
)

func main() {
	// bufio.NewScanner wraps a reader.
	sc := bufio.NewScanner(strings.NewReader("line1\nline2\n"))
	for sc.Scan() {
		_ = sc.Text()
	}
	_ = sc.Err()

	// bufio.NewReader
	br := bufio.NewReader(strings.NewReader("hello"))
	_ = br

	// bufio.NewWriter
	bw := bufio.NewWriter(io_discard{})
	n, err := bw.WriteString("world")
	_ = n
	_ = err
	_ = bw.Flush()
}

// io_discard is a minimal io.Writer to avoid importing io (keeps deps minimal).
type io_discard struct{}

func (io_discard) Write(p []byte) (int, error) { return len(p), nil }
