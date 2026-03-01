// unsafe_string_nil verifies that unsafe.String with a nil pointer and non-zero
// length is reported as an "unsafe-slice" violation.
//
// At runtime Go panics: "unsafe.String: ptr is nil".
// Expected: 1 violation, category "unsafe-slice".
package main

import "unsafe"

func main() {
	var p *byte // nil pointer
	_ = unsafe.String(p, 4) // nil ptr + non-zero len: runtime panic
}
