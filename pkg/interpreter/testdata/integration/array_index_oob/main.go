// array_index_oob verifies that indexing a pointer-to-array out of bounds is
// reported as an "out-of-bounds" violation.
//
// At runtime Go panics: "runtime error: index out of range [5] with length 3".
// The index must be a variable to avoid compile-time rejection.
// Expected: 1 violation, category "out-of-bounds".
package main

func badIndex() int { return 5 }

func main() {
	var arr [3]int
	p := &arr
	_ = p[badIndex()] // index 5 >= array length 3: runtime panic
}
