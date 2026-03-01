// unsafe_string_valid verifies that valid unsafe.String calls produce no violations.
//
// unsafe.String(nil, 0) is valid (zero-length string from nil pointer is allowed).
// unsafe.String(&b, n) with n >= 0 is valid.
// Expected: 0 violations.
package main

import "unsafe"

func main() {
	var b byte = 'h'
	s := unsafe.String(&b, 1) // valid: non-nil ptr, len=1
	_ = s

	// nil pointer with zero length is valid
	var p *byte
	_ = unsafe.String(p, 0)
}
