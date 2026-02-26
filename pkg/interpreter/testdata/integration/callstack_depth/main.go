package main

import "unsafe"

// callstack_depth exercises call stack capture in violation reports.
// The misalignment violation at level4 should include the full call chain
// level1 → level2 → level3 → level4 in its stack trace.
// Expected: 1 violation with category "rule 1".

func level4(b []byte) uint32 {
	// Converts *byte at offset 1 to *uint32 — violates Rule 1 (alignment).
	return *(*uint32)(unsafe.Pointer(&b[1]))
}

func level3(b []byte) uint32 { return level4(b) }
func level2(b []byte) uint32 { return level3(b) }
func level1(b []byte) uint32 { return level2(b) }

func main() {
	buf := make([]byte, 8)
	_ = level1(buf)
}
