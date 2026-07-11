// integer_truncation verifies that a narrowing integer conversion which
// silently discards significant bits is detected when the truncation detector
// is enabled (#223).
//
// int8(300) wraps to 44 in Go's well-defined arithmetic — a common source of
// logic bugs. Expected: 1 violation (integer-truncation).
package main

func narrow(v int) int8 {
	return int8(v)
}

func main() {
	// 300 does not fit in an int8 (range -128..127); the conversion wraps.
	_ = narrow(300)
}
