// fmt_sprintf verifies that the fmt.Sprintf stdlib intercept returns the
// correct non-empty string for concrete arguments.
//
// fmt.Sprintf("hello=%s", "world") returns "hello=world" (11 bytes), so
// len(s) > 5 is true and the if-body is entered, triggering a Rule 1 access.
//
// Without the intercept: Sprintf returns Value{} (opaque nil) → len(nil) == 0
// → 0 > 5 is false → if-body never entered → 0 violations (wrong).
// With the intercept: len("hello=world") == 11 → 11 > 5 == true →
// Rule 1 violation detected.
// Expected: 1 violation, category "rule 1".
package main

import (
	"fmt"
	"unsafe"
)

func format(name string) {
	s := fmt.Sprintf("hello=%s", name)
	if len(s) > 5 {
		// Reached when Sprintf returns a string longer than 5 bytes.
		// Rule 1: misaligned uint32 read from offset 1 of a [5]byte.
		var b [5]byte
		_ = *(*uint32)(unsafe.Pointer(&b[1]))
	}
}

func main() {
	format("world")
}
