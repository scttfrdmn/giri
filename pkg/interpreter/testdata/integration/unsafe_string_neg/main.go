// unsafe_string_neg verifies that unsafe.String with a negative length argument
// is reported as an "unsafe-slice" violation.
//
// At runtime Go panics: "unsafe.String: len out of range".
// The compiler rejects constant negative lengths, so we use a helper function
// to produce a runtime-negative value.
// Expected: 1 violation, category "unsafe-slice".
package main

import "unsafe"

func negLen() int { return -1 }

func main() {
	var b byte = 'a'
	n := negLen()                  // -1, not a compile-time constant at the call site
	_ = unsafe.String(&b, n)      // negative length: runtime panic
}
