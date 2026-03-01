// and_not verifies that the bit-clear (AND NOT, &^) operator is evaluated
// correctly (#140).
//
// False-positive canary: if a &^ b != 0xF0, the interpreter returned Value{}
// for the &^ operation; the comparison produces Value{} (not bool false), and
// ssa.If defaults to the TRUE branch, triggering the nil-slice OOB.
//
// Expected: 0 violations.
package main

func main() {
	var a int64 = 0xFF  // 1111_1111
	var b int64 = 0x0F  // 0000_1111
	result := a &^ b    // clear low nibble → 1111_0000 = 0xF0
	if result != 0xF0 {
		var s []int
		_ = s[0] // false positive: only reached if &^ wrongly returns Value{}
	}

	// Additional &^ patterns
	var x int64 = 0b1010_1010
	var mask int64 = 0b1111_0000
	cleared := x &^ mask // clear upper nibble → 0b0000_1010 = 0x0A
	if cleared != 0x0A {
		var s []int
		_ = s[0]
	}

	// &^ with zero mask is identity
	var n int64 = 42
	if n&^0 != 42 {
		var s []int
		_ = s[0]
	}
}
