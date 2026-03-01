// cmp_compare verifies that cmp.* functions are correctly intercepted.
//
// Exercises Compare (concrete int comparison), Less, and Or.
// (cmp.Equal does not exist in the standard library; use cmp.Compare for
// equality checks.)
//
// Expected: 0 violations.
package main

import "cmp"

func main() {
	// Compare: concrete integer comparison should work correctly.
	r := cmp.Compare(1, 2)
	if r != -1 {
		var x []int
		_ = x[0] // canary: fires only if Compare(1,2) != -1
	}

	r2 := cmp.Compare(5, 5)
	if r2 != 0 {
		var x []int
		_ = x[0] // canary: fires only if Compare(5,5) != 0
	}

	// Less: concrete comparison.
	_ = cmp.Less(1, 2)

	// Or: returns first non-zero value.
	v := cmp.Or(0, 0, 42)
	_ = v
}
