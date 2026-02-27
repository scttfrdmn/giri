// os_exit verifies that os.Exit(0) halts interpretation cleanly (#62).
//
// The program calls os.Exit(0) before any unsafe operation. With the intercept,
// all goroutines are marked Panicked and execution stops at the Exit call.
// Code after os.Exit is unreachable, so no violations should be reported.
//
// Expected: 0 violations.
package main

import (
	"os"
	"unsafe"
)

func main() {
	os.Exit(0)
	// Unreachable: misaligned load that would be a Rule 1 violation if reached.
	var b [5]byte
	_ = *(*uint32)(unsafe.Pointer(&b[1]))
}
