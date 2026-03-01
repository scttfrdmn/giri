// unsafe_slice_neg verifies that unsafe.Slice with a negative length argument
// is reported as an "unsafe-slice" violation.
//
// At runtime Go panics: "unsafe.Slice: len out of range".
// The compiler rejects constant negative lengths, so we use a function to
// produce a runtime-negative value that bypasses compile-time checks.
// Expected: 1 violation, category "unsafe-slice".
package main

import "unsafe"

func negLen() int { return -1 }

func main() {
	var x int
	p := (*byte)(unsafe.Pointer(&x))
	n := negLen()    // -1, not a compile-time constant at the call site
	_ = unsafe.Slice(p, n)
}
