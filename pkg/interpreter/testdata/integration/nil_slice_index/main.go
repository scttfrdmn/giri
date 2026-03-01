// nil_slice_index verifies that indexing into a nil slice is detected.
// In Go, s[0] where s is nil panics: "index out of range [0] with length 0".
// Expected: 1 violation (out-of-bounds).
package main

func getElement(s []int, i int) int {
	return s[i]
}

func main() {
	var s []int // nil slice
	_ = getElement(s, 0)
}
