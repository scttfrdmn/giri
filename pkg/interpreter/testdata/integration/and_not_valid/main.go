// and_not_valid verifies common bit-clear idioms produce correct results (#140).
//
// Expected: 0 violations.
package main

func main() {
	// Clear specific bits from a flags word.
	const FlagA = 1 << 0
	const FlagB = 1 << 1
	const FlagC = 1 << 2

	var flags int64 = FlagA | FlagB | FlagC // 0b111
	flags = flags &^ FlagB                  // clear FlagB → 0b101
	_ = flags

	// AND NOT is the complement of AND:
	// x & mask  keeps only set bits in mask
	// x &^ mask clears only set bits in mask
	var v int64 = 0xFF
	kept := v & 0x0F   // 0x0F
	cleared := v &^ 0x0F // 0xF0
	if kept+cleared != v {
		var s []int
		_ = s[0] // kept + cleared must equal original (no overlap)
	}
}
