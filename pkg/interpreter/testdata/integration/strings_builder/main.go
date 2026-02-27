// strings_builder verifies that strings.Builder method intercepts work (#79).
//
// Expected: 0 violations.
package main

import "strings"

func main() {
	var b strings.Builder

	// WriteString returns (n, nil).
	n, err := b.WriteString("Hello")
	_ = n   // 5
	_ = err // nil

	// WriteByte returns nil error.
	_ = b.WriteByte(',')

	// WriteRune returns (1, nil).
	sz, err2 := b.WriteRune(' ')
	_ = sz   // 1
	_ = err2 // nil

	// Write returns (n, nil).
	nn, err3 := b.Write([]byte("World"))
	_ = nn   // 5
	_ = err3 // nil

	// Len returns accumulated length.
	_ = b.Len()

	// Cap reports capacity.
	_ = b.Cap()

	// String returns accumulated content.
	s := b.String()
	_ = s

	// Grow pre-allocates capacity.
	b.Grow(64)

	// Reset clears the builder.
	b.Reset()
	_ = b.Len() // 0
}
