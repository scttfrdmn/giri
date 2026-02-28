// make_invalid_len verifies that make() with a negative length is detected.
// In Go, make([]T, -1) panics at runtime: "makeslice: len out of range".
// Expected: 1 violation (make-invalid).
package main

func makeSlice(n int) []int {
	return make([]int, n)
}

func main() {
	_ = makeSlice(-1)
}
