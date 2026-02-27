// string_byte_index verifies that s[i] returns the byte at byte position i,
// not a rune at rune position i (#73).
//
// Expected: 0 violations.
package main

func main() {
	// ASCII: byte indexing and rune indexing are equivalent.
	s := "Hello"
	b0 := s[0] // 'H' = 72
	b1 := s[1] // 'e' = 101
	_ = b0
	_ = b1

	// Verify len() is byte count.
	n := len(s)
	_ = n // 5

	// Simple iteration over bytes.
	for i := 0; i < len(s); i++ {
		_ = s[i]
	}

	// Multi-character string byte access.
	s2 := "abcdef"
	last := s2[len(s2)-1] // 'f' = 102
	_ = last
}
