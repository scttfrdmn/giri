// suppress_ignore verifies that //giri:ignore comments suppress violations
// at the annotated line (#58).
//
// readU32 performs a misaligned uint32 load (Rule 1 violation). The
// //giri:ignore comment on the line immediately before the offending
// instruction causes Giri to drop the violation.
//
// Expected: 0 violations.
package main

import "unsafe"

// readU32 reads a uint32 from a byte slice at a non-aligned offset.
// The //giri:ignore comment suppresses the Rule 1 violation that Giri
// would otherwise report for this misaligned unsafe.Pointer conversion.
func readU32(b []byte) uint32 {
	//giri:ignore rule 1
	return *(*uint32)(unsafe.Pointer(&b[1])) // offset 1 mod 4 != 0
}

func main() {
	buf := []byte{0x00, 0x01, 0x02, 0x03, 0x04}
	_ = readU32(buf)
}
