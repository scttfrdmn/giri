// complex_builtins verifies that the real(), imag(), and complex() built-in
// functions are handled correctly by the interpreter (#141).
//
// False-positive canary: if real(complex(3.0, 4.0)) != 3.0, the builtin
// returned Value{} or the wrong value; the != comparison returns Value{} (not
// bool false) and ssa.If defaults to the TRUE branch, triggering nil-slice OOB.
//
// Expected: 0 violations.
package main

func main() {
	// complex() constructs a complex128 from real and imaginary parts.
	c := complex(3.0, 4.0)

	// real() extracts the real part.
	if real(c) != 3.0 {
		var s []int
		_ = s[0] // false positive: only reached if real() is wrong
	}

	// imag() extracts the imaginary part.
	if imag(c) != 4.0 {
		var s []int
		_ = s[0] // false positive: only reached if imag() is wrong
	}

	// Round-trip: complex(real(c), imag(c)) == c
	c2 := complex(real(c), imag(c))
	if c2 != c {
		var s []int
		_ = s[0]
	}

	// Zero complex value.
	var z complex128
	if real(z) != 0.0 || imag(z) != 0.0 {
		var s []int
		_ = s[0]
	}
}
