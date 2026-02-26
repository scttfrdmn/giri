// Showcase: misaligned pointer conversion (unsafe.Pointer Rule 1)
//
// Reads a uint32 from a byte slice at an offset that is not 4-byte aligned.
// unsafe.Pointer Rule 1 states that a pointer may only be converted to a
// pointer type T if the memory is aligned to T's required alignment.
// *uint32 requires 4-byte alignment; offset 1 violates this.
//
// What each tool reports:
//
//	go vet:        PASS — vet does not track pointer alignment
//	go test -race: PASS — no concurrent access
//	giri:          FAIL — unsafe-pointer-violation: rule 1: invalid pointer conversion (alignment)
//
// Why this matters:
//   - ARM, MIPS, RISC-V: causes SIGBUS (bus error crash) at runtime.
//   - x86: may silently return a rotated or torn value on some hardware.
//   - The Go spec explicitly forbids this via the unsafe.Pointer rules.
package main

import "unsafe"

// readU32LE reads a little-endian uint32 from b at the given byte offset.
// Bug: does not verify that offset is 4-byte aligned before performing the cast.
func readU32LE(b []byte, offset int) uint32 {
	return *(*uint32)(unsafe.Pointer(&b[offset]))
}

func main() {
	// Simulated binary packet: 1-byte type prefix followed by a uint32 payload.
	// The payload starts at byte 1, which is not 4-byte aligned.
	packet := []byte{0x02, 0xDE, 0xAD, 0xBE, 0xEF}
	_ = readU32LE(packet, 1) // offset 1 mod 4 != 0 → misaligned
}
