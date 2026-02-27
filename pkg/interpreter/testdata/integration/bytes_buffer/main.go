// bytes_buffer verifies that bytes.Buffer method intercepts work (#79).
//
// Expected: 0 violations.
package main

import "bytes"

func main() {
	var buf bytes.Buffer

	// Write returns (n, nil).
	n, err := buf.Write([]byte("Hello"))
	_ = n   // 5
	_ = err // nil

	// WriteString returns (n, nil).
	n2, err2 := buf.WriteString(", World")
	_ = n2   // 7
	_ = err2 // nil

	// WriteByte returns nil error.
	_ = buf.WriteByte('!')

	// WriteRune returns (size, nil).
	sz, err3 := buf.WriteRune('A')
	_ = sz   // 1
	_ = err3 // nil

	// Len returns accumulated length.
	_ = buf.Len()

	// Cap reports capacity.
	_ = buf.Cap()

	// String returns content as string.
	s := buf.String()
	_ = s

	// Bytes returns content as []byte.
	b := buf.Bytes()
	_ = b

	// Grow pre-allocates capacity.
	buf.Grow(64)

	// Reset clears the buffer.
	buf.Reset()
	_ = buf.Len() // 0

	// bytes.NewBuffer convenience constructor.
	buf2 := bytes.NewBuffer([]byte("init"))
	_ = buf2

	// bytes.NewBufferString convenience constructor.
	buf3 := bytes.NewBufferString("hello")
	_ = buf3
}
