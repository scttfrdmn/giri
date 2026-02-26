// Showcase: uintptr held across a GC safepoint (unsafe.Pointer Rule 2)
//
// Stores a pointer as a uintptr integer, calls a function (a GC safepoint),
// then converts the integer back to a pointer. If the GC moves the object
// between those two points, the uintptr is now a dangling pointer.
//
// unsafe.Pointer Rule 2: conversion of a Pointer to uintptr must appear in
// the same expression as the call/operation that uses it. Storing a uintptr
// in a variable and calling ANY function between the conversion and the use
// is FORBIDDEN by the Go specification.
//
// What each tool reports:
//
//	go vet:        PASS — vet does not track uintptr liveness across calls
//	go test -race: PASS — no concurrent access
//	giri:          FAIL — unsafe-pointer-violation: rule 2: uintptr held across GC point
//
// Why this matters: the current Go GC does not move objects, so this pattern
// appears to work today on amd64. But it is explicitly forbidden by the Go
// specification and will silently break with any future moving or compacting GC.
// This is a latent bug that only surfaces when the runtime implementation changes.
package main

import "unsafe"

//go:noinline
func doWork() { /* simulate work — this call is a GC safepoint */ }

func main() {
	x := new(int)
	*x = 42

	// Convert pointer to integer. While addr is live as a uintptr (not
	// unsafe.Pointer), the GC does not treat it as a pointer — the object
	// x points to may be reclaimed or moved.
	addr := uintptr(unsafe.Pointer(x))

	// Any function call is a potential GC safepoint.
	// After doWork() returns, addr may be a stale integer, not a valid pointer.
	doWork()

	// Converting the stale uintptr back to unsafe.Pointer and dereferencing
	// is undefined behaviour — may silently read freed or reallocated memory.
	_ = *(*int)(unsafe.Pointer(addr))
}
