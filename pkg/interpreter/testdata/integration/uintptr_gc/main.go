package main

import "unsafe"

// sink is a non-inline function that acts as a GC safepoint.
func sink(v int) int { return v }

func main() {
	x := new(int)
	// Convert to uintptr without immediately converting back.
	// The call to sink() is a GC point while p is a live uintptr — Rule 2 violation.
	p := uintptr(unsafe.Pointer(x))
	sink(0) // GC point: p is a pending uintptr → violation
	_ = p
}
