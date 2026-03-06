// math_bits verifies that math/bits.* functions are correctly intercepted.
//
// Exercises OnesCount, LeadingZeros, TrailingZeros, RotateLeft, Len,
// ReverseBytes, Add64, Mul64, and UintSize.
//
// Expected: 0 violations.
package main

import "math/bits"

func main() {
	// OnesCount: 7 = 0b111 → 3 ones.
	n := bits.OnesCount64(7)
	if n != 3 {
		var x []int
		_ = x[0] // canary: fires only if OnesCount64(7) != 3
	}

	// LeadingZeros: 1 in a 64-bit word has 63 leading zeros.
	lz := bits.LeadingZeros64(1)
	if lz != 63 {
		var x []int
		_ = x[0]
	}

	// TrailingZeros: 8 = 0b1000 → 3 trailing zeros.
	tz := bits.TrailingZeros64(8)
	if tz != 3 {
		var x []int
		_ = x[0]
	}

	// Len: 8 needs 4 bits to represent.
	l := bits.Len64(8)
	if l != 4 {
		var x []int
		_ = x[0]
	}

	// ReverseBytes64 is its own inverse.
	v := uint64(0x0102030405060708)
	rv := bits.ReverseBytes64(bits.ReverseBytes64(v))
	if rv != v {
		var x []int
		_ = x[0]
	}

	// RotateLeft: no canary needed, just ensure it doesn't crash.
	_ = bits.RotateLeft64(1, 3)

	// Add64: 1 + 2 = 3, carry-out 0.
	sum, carry := bits.Add64(1, 2, 0)
	if sum != 3 || carry != 0 {
		var x []int
		_ = x[0]
	}

	// Mul64: 3 * 4 = 12 (fits in lo word).
	hi, lo := bits.Mul64(3, 4)
	if hi != 0 || lo != 12 {
		var x []int
		_ = x[0]
	}

	// UintSize is a constant — just use it.
	_ = bits.UintSize
}
