// Showcase: unsafe out-of-bounds memory access
//
// A minimal "frame parser" reads a protocol header using unsafe pointer
// arithmetic, but the offset calculation is wrong — it reads 4 bytes past the
// end of the 4-byte allocation.
//
// What each tool reports:
//
//	go vet:        PASS — vet has no check for unsafe arithmetic bounds
//	go test -race: PASS — single goroutine, no race condition
//	giri:          FAIL — unsafe-pointer-violation: pointer arithmetic exceeds allocation bounds
//
// Why this matters: on most systems this silently reads adjacent stack or heap
// memory without panicking. There is no runtime error and -race is silent.
// The only signal is reading garbage data — which may not be noticed until
// production.
package main

import "unsafe"

// parseFrameType reads the 1-byte frame type from a fixed-size frame header.
// Bug: the type field is at byte 4, but the header allocation is only 4 bytes,
// so offset 4 points one byte past the end of the allocation.
func parseFrameType(hdr *[4]byte) byte {
	// hdr layout (intended): [version:1][flags:1][length:2][type:1]
	// Bug: the allocation is [4]byte but we try to read index 4 (past the end).
	return *(*byte)(unsafe.Add(unsafe.Pointer(hdr), 4))
}

func main() {
	hdr := [4]byte{0x01, 0x00, 0x00, 0x10}
	frameType := parseFrameType(&hdr)
	_ = frameType
}
