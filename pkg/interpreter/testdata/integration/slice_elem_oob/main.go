// slice_elem_oob verifies that accessing a slice element beyond its declared
// length is reported as an "out-of-bounds" violation, even when the index
// is within the slice's capacity.
//
// At runtime Go panics: "runtime error: index out of range [7] with length 3".
// Expected: 1 violation, category "out-of-bounds".
package main

func bigIndex() int { return 7 }

func main() {
	s := make([]int, 3, 10) // len=3, cap=10
	_ = s[bigIndex()]       // index 7 is within cap but beyond len=3: panic
}
