// strconv_atoi verifies that the strconv.Atoi stdlib intercept returns the
// correct integer value for a concrete string argument.
//
// strconv.Atoi("42") returns (42, nil), so n > 0 is true and the if-body is
// entered, triggering a misaligned Rule 1 access.
//
// Without the intercept: Atoi returns Value{} (opaque nil) → n is nil →
// n > 0 is not a valid integer comparison → ssa.If treats condition as false
// → if-body never entered → 0 violations (wrong).
// With the intercept: n == 42 → 42 > 0 == true → Rule 1 violation detected.
// Expected: 1 violation, category "rule 1".
package main

import (
	"strconv"
	"unsafe"
)

func parse(s string) {
	n, _ := strconv.Atoi(s)
	if n > 0 {
		// Reached when s parses to a positive integer.
		// Rule 1: misaligned uint32 read from offset 1 of a [5]byte.
		var b [5]byte
		_ = *(*uint32)(unsafe.Pointer(&b[1]))
	}
}

func main() {
	parse("42")
}
