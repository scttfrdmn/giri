package main

import "unsafe"

func main() {
	x := new(int)
	// Convert to uintptr and immediately back — no intervening GC point.
	// This is Rule 2 compliant: the uintptr is consumed before any function call.
	q := (*int)(unsafe.Pointer(uintptr(unsafe.Pointer(x))))
	_ = q
}
