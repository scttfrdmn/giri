// int_truncate_valid verifies that integer conversions that do NOT truncate
// produce no violations (#139).
//
// Expected: 0 violations.
package main

func main() {
	// Small values fit in any target type without truncation.
	var x int64 = 42
	_ = int8(x)   // 42 fits in int8
	_ = uint8(x)  // 42 fits in uint8
	_ = int16(x)  // 42 fits in int16
	_ = uint16(x) // 42 fits in uint16
	_ = int32(x)  // 42 fits in int32
	_ = uint32(x) // 42 fits in uint32

	// Round-trips: int32 → int64 → int32 = same value for small numbers.
	var y int32 = 1000
	z := int64(y)
	_ = int32(z) // back to 1000, no truncation

	// Widening conversions are always exact.
	var b int8 = 127
	_ = int16(b) // 127 widened to int16: exact
	_ = int32(b) // 127 widened to int32: exact
	_ = int64(b) // 127 widened to int64: exact
}
