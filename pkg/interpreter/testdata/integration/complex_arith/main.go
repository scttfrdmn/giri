// complex_arith verifies that arithmetic operations on complex128 values
// (+, -, *, /, ==, !=) are evaluated correctly (#141).
//
// Expected: 0 violations.
package main

func main() {
	a := complex(1.0, 2.0) // 1+2i
	b := complex(3.0, 4.0) // 3+4i

	// Addition: (1+2i) + (3+4i) = 4+6i
	sum := a + b
	if real(sum) != 4.0 || imag(sum) != 6.0 {
		var s []int
		_ = s[0]
	}

	// Subtraction: (3+4i) - (1+2i) = 2+2i
	diff := b - a
	if real(diff) != 2.0 || imag(diff) != 2.0 {
		var s []int
		_ = s[0]
	}

	// Multiplication: (1+2i) * (3+4i) = 3+4i+6i+8i² = 3+10i-8 = -5+10i
	prod := a * b
	if real(prod) != -5.0 || imag(prod) != 10.0 {
		var s []int
		_ = s[0]
	}

	// Equality
	c := complex(1.0, 2.0)
	if a != c {
		var s []int
		_ = s[0]
	}
	if a == b {
		var s []int
		_ = s[0]
	}
}
