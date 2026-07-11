// integer_truncation_valid verifies that value-preserving integer conversions
// do NOT trigger the truncation detector even when it is enabled (#223).
//
// 100 fits in an int8, and widening int8→int64 always round-trips.
// Expected: 0 violations.
package main

func narrow(v int) int8 {
	return int8(v)
}

func widen(v int8) int64 {
	return int64(v)
}

func main() {
	x := narrow(100) // 100 fits in int8, no truncation
	_ = widen(x)     // widening never truncates
}
