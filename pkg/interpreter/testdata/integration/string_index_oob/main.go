// string_index_oob verifies that an out-of-bounds string index is detected.
// In Go, s[i] where i >= len(s) panics: "index out of range [N] with length M".
// Expected: 1 violation (out-of-bounds).
package main

func byteAt(s string, i int) byte {
	return s[i]
}

func main() {
	s := "hello"
	_ = byteAt(s, 10) // index 10 >= len("hello") == 5
}
