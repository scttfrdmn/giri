// complex_neg verifies that unary negation of complex128 values is handled
// correctly by the interpreter (#144).
//
// False-positive canary: without the fix, -c returns c unchanged, so
// -c == -1-2i is false (c==-1-2i would be comparing 1+2i with -1-2i),
// causing the interpreter to take the wrong branch.
//
// Expected: 0 violations.
package main

func main() {
	c := complex(1.0, 2.0) // 1+2i
	neg := -c              // should be -1-2i

	if neg != complex(-1.0, -2.0) {
		var s []int
		_ = s[0] // false positive: only reached if -c is wrong
	}

	// Double negation must be identity.
	if -(-c) != c {
		var s []int
		_ = s[0]
	}

	// Negation of zero complex is zero.
	var z complex128
	if -z != 0 {
		var s []int
		_ = s[0]
	}
}
