// complex_conv verifies that complex64 ↔ complex128 type conversions are
// handled correctly by the interpreter (#144).
//
// Expected: 0 violations.
package main

func main() {
	// complex64 → complex128 (widening).
	var c64 complex64 = complex(1.5, 2.5)
	c128 := complex128(c64)
	if real(c128) != float64(real(c64)) {
		var s []int
		_ = s[0] // false positive: only reached if conversion is wrong
	}

	// complex128 → complex64 (narrowing).
	orig := complex(3.0, 4.0) // complex128
	narrow := complex64(orig)
	if real(narrow) != float32(3.0) {
		var s []int
		_ = s[0]
	}

	// Untyped constant complex128 value is directly usable.
	const c = complex(1.0, 0.0) // untyped complex constant
	var cc complex128 = c
	if cc != 1.0+0i {
		var s []int
		_ = s[0]
	}
}
