// unsafe_slice_valid verifies that valid unsafe.Slice calls produce no violations.
//
// unsafe.Slice(nil, 0) is valid (zero-length slice from nil pointer is allowed).
// unsafe.Slice(&arr[0], n) with n >= 0 is valid.
// Expected: 0 violations.
package main

import "unsafe"

func main() {
	var arr [4]byte
	s := unsafe.Slice(&arr[0], 4)
	_ = s

	// nil pointer with zero length is valid
	var p *byte
	_ = unsafe.Slice(p, 0)
}
