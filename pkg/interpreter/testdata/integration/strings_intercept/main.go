// strings_intercept verifies that the strings.* stdlib intercept returns
// correct values for concrete arguments, enabling proper branch dispatch.
//
// strings.Contains("hello world", "world") == true, so the if-body is entered
// and a misaligned Rule 1 access is detected.
//
// Without the intercept: Contains returns Value{} (opaque nil), treated as
// false by ssa.If → the if-body is never entered → 0 violations (wrong).
// With the intercept: true → if-body entered → Rule 1 violation detected.
// Expected: 1 violation, category "rule 1".
package main

import (
	"strings"
	"unsafe"
)

func check(s, sub string) {
	if strings.Contains(s, sub) {
		// Reached only when s contains sub.
		// Rule 1: misaligned uint32 read from offset 1 of a [5]byte.
		var b [5]byte
		_ = *(*uint32)(unsafe.Pointer(&b[1]))
	}
}

func main() {
	check("hello world", "world")
}
