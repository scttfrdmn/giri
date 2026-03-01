// int_truncate verifies that integer conversions apply correct bit-width
// truncation (#139).
//
// False-positive canary: int8(256) must equal 0 (256 & 0xFF = 0). Without the
// fix, the interpreter returns 256 unchanged; the "!= 0" condition is TRUE and
// a nil-slice OOB fires as a false positive. With the fix, int8(256) = 0,
// the condition is FALSE, and no violation occurs.
//
// Expected: 0 violations.
package main

func main() {
	// int8(256) = 0 (truncation: 0x100 & 0xFF = 0x00)
	var n int64 = 256
	y := int8(n)
	if y != 0 {
		var s []int
		_ = s[0] // false positive: only reached if int8(256) wrongly returns 256
	}

	// uint8(256) = 0 (same truncation)
	z := uint8(n)
	if z != 0 {
		var s []int
		_ = s[0] // false positive: only reached if uint8(256) wrongly returns 256
	}

	// int16(65536) = 0 (0x10000 & 0xFFFF = 0)
	var m int64 = 65536
	w := int16(m)
	if w != 0 {
		var s []int
		_ = s[0] // false positive: only reached if int16(65536) wrongly returns 65536
	}
}
