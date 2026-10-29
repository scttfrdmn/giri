// math_bits_widths checks that the math/bits intercepts honor the operand
// width (#250). Each function used to be served by its 64-bit counterpart,
// so LeadingZeros32(1) returned 63 and RotateLeft8 rotated through 64 bits.
//
// Every check that fails calls wrong(), which indexes an empty slice: one
// out-of-bounds violation per wrong result.
// Expected: 0 violations.
package main

import "math/bits"

func wrong() {
	var t []int
	_ = t[0]
}

func main() {
	if bits.LeadingZeros8(1) != 7 {
		wrong()
	}
	if bits.LeadingZeros16(1) != 15 {
		wrong()
	}
	if bits.LeadingZeros32(1) != 31 {
		wrong()
	}
	if bits.TrailingZeros8(0) != 8 {
		wrong()
	}
	if bits.TrailingZeros16(0) != 16 {
		wrong()
	}
	if bits.TrailingZeros32(0) != 32 {
		wrong()
	}
	if bits.RotateLeft8(0x81, 1) != 0x03 {
		wrong()
	}
	if bits.RotateLeft16(0x8001, 1) != 0x0003 {
		wrong()
	}
	if bits.RotateLeft32(0x80000001, 1) != 0x00000003 {
		wrong()
	}
	if bits.Reverse8(1) != 0x80 {
		wrong()
	}
	if bits.Reverse32(1) != 0x80000000 {
		wrong()
	}
	if bits.ReverseBytes16(0x0102) != 0x0201 {
		wrong()
	}
	if bits.ReverseBytes32(0x01020304) != 0x04030201 {
		wrong()
	}
	if bits.Len8(0xff) != 8 || bits.Len32(1<<31) != 32 {
		wrong()
	}
	if sum, carry := bits.Add32(0xffffffff, 1, 0); sum != 0 || carry != 1 {
		wrong()
	}
	if diff, borrow := bits.Sub32(0, 1, 0); diff != 0xffffffff || borrow != 1 {
		wrong()
	}
	if hi, lo := bits.Mul32(0xffffffff, 2); hi != 1 || lo != 0xfffffffe {
		wrong()
	}
	if q, r := bits.Div32(1, 0, 3); q != 0x55555555 || r != 1 {
		wrong()
	}
	if bits.OnesCount8(0xff) != 8 || bits.OnesCount64(0xff00ff) != 16 {
		wrong()
	}
}
