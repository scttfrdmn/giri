package main

import "unsafe"

func main() {
	x := new([8]byte)
	// unsafe.Add moves the pointer by 1 byte.
	// Converting that pointer to *uint32 violates Rule 1: the offset 1 is not
	// divisible by the alignment of uint32 (4 bytes).
	p := unsafe.Add(unsafe.Pointer(x), 1)
	q := (*uint32)(p) // Rule 1 violation: misaligned access
	_ = q
}
