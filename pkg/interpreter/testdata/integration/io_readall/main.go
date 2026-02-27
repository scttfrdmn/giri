// io_readall verifies that io package intercepts work (#78).
//
// Expected: 0 violations.
package main

import (
	"io"
	"strings"
)

func main() {
	// io.ReadAll consumes a reader.
	r := strings.NewReader("hello world")
	data, err := io.ReadAll(r)
	_ = data
	_ = err

	// io.WriteString writes to a writer.
	n, err2 := io.WriteString(io.Discard, "hello")
	_ = n
	_ = err2

	// io.NopCloser wraps a reader.
	rc := io.NopCloser(strings.NewReader("test"))
	_ = rc

	// io.LimitReader wraps a reader.
	lr := io.LimitReader(strings.NewReader("test"), 3)
	_ = lr

	// io.MultiReader combines readers.
	mr := io.MultiReader(strings.NewReader("a"), strings.NewReader("b"))
	_ = mr

	// io.Copy reads from one and writes to another.
	var written int64
	written, _ = io.Copy(io.Discard, strings.NewReader("abc"))
	_ = written
}
