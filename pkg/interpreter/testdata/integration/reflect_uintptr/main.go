package main

import (
	"reflect"
	"unsafe"
)

func noop() {}

func main() {
	x := 42
	v := reflect.ValueOf(&x)
	// Rule 5: reflect.Value.Pointer() returns a uintptr.
	// The uintptr must be converted back to unsafe.Pointer before any GC point.
	uptr := v.Pointer()
	noop() // GC safepoint: uptr is still a uintptr — this is UB!
	p := (*int)(unsafe.Pointer(uptr))
	_ = *p
}
