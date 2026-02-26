// This program compiles cleanly, passes go vet, and passes go test -race.
// Giri detects the reflect.Value.Pointer() uintptr escaping past a GC safepoint.
package main

import (
	"reflect"
	"unsafe"
)

// processValue extracts a pointer from a reflect.Value and does "work" before
// converting it back. The bug: v.Pointer() returns a uintptr (not a safe pointer),
// and calling doWork() is a GC safepoint. If the GC runs, the original allocation
// could theoretically be reclaimed or moved, making the uintptr stale.
//
// The Go spec requires that a uintptr from reflect.Value.Pointer() be converted
// to unsafe.Pointer *in the same expression*, with no intervening calls.
//
// go vet cannot detect this because the types are correct.
// go test -race does not flag it because there is no concurrent access.
// Giri catches it by tracking the uintptr across GC safepoints.
func processValue(v reflect.Value) *int {
	uptr := v.Pointer() // Rule 5: returns uintptr, not a tracked pointer
	doWork()            // GC safepoint — uptr may now be stale!
	return (*int)(unsafe.Pointer(uptr))
}

func doWork() {
	// Simulates work that constitutes a GC safepoint.
}

func main() {
	x := 42
	v := reflect.ValueOf(&x)
	p := processValue(v)
	_ = *p
}
