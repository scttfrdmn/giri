// strings_reader verifies that strings.NewReader, bytes.NewReader, and
// bytes.NewBuffer intercepts work (#103).
//
// Expected: 0 violations.
package main

import (
	"bytes"
	"strings"
)

func main() {
	// strings.NewReader returns an opaque *strings.Reader.
	r := strings.NewReader("hello, world")

	buf := make([]byte, 8)
	n, err := r.Read(buf)
	_ = n
	_ = err

	b, err2 := r.ReadByte()
	_ = b
	_ = err2

	ch, sz, err3 := r.ReadRune()
	_ = ch
	_ = sz
	_ = err3

	off, err4 := r.Seek(0, 0)
	_ = off
	_ = err4

	_ = r.Len()
	_ = r.Size()

	// bytes.NewReader.
	br := bytes.NewReader([]byte("hello bytes"))
	n2, err5 := br.Read(buf)
	_ = n2
	_ = err5

	off2, err6 := br.Seek(0, 0)
	_ = off2
	_ = err6

	_ = br.Len()
	_ = br.Size()

	// bytes.NewBuffer.
	bb := bytes.NewBuffer([]byte("initial"))
	bb.WriteString(" more")
	_ = bb.Len()
	_ = bb.String()

	// bytes.NewBufferString.
	bb2 := bytes.NewBufferString("hello")
	_ = bb2.Len()
}
