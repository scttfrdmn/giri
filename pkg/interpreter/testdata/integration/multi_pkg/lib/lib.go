// Package lib is a helper package for the multi_pkg integration test.
// It contains an unsafe misaligned pointer access that Giri should detect
// when interpreting code that crosses package boundaries (#53).
package lib

import "unsafe"

// UnsafeRead reads a uint32 from a byte slice at offset 1 (misaligned).
// This violates unsafe.Pointer Rule 1 and Giri should report it when
// called from a different package.
func UnsafeRead(b []byte) uint32 {
	return *(*uint32)(unsafe.Pointer(&b[1])) // offset 1 mod 4 != 0 → Rule 1
}
