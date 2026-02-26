// This file contains example programs that exhibit undefined behavior detectable
// by Giri. It is intentionally broken code used as integration test fixtures.
//
// Files in testdata/ are not compiled as part of any package.

//go:build ignore

package main

// Each function below demonstrates one class of UB that Giri should detect.
// Run with: giri ./testdata/...

// --- Arena Use-After-Free ---

// arenaUseAfterFree allocates inside an arena, frees the arena, then reads
// the value. This is a use-after-free via arena.Free().
func arenaUseAfterFree() {
	// a := arena.NewArena()
	// defer a.Free()
	// p := arena.New[int](a)
	// *p = 42
	// a.Free()   // double-free: defer will call Free again
	// _ = *p     // use-after-free: arena already freed
}

// arenaEscapeReturn allocates in an arena and returns the pointer, which
// will outlive the arena's lifetime.
func arenaEscapeReturn() *int {
	// a := arena.NewArena()
	// defer a.Free()
	// p := arena.New[int](a)
	// *p = 99
	// return p  // arena pointer escape via return
	return nil
}

// arenaLeak creates an arena but never frees it.
func arenaLeak() {
	// a := arena.NewArena()
	// p := arena.New[int](a)
	// _ = p
	// (no a.Free() call — arena leak)
}

// --- unsafe.Pointer Violations ---

// unsafeOutOfBounds uses unsafe.Add to move a pointer past the end of its
// allocation, violating Rule 3 (arithmetic must stay within allocation).
func unsafeOutOfBounds() {
	// x := new([4]byte)
	// p := unsafe.Pointer(x)
	// past := unsafe.Add(p, 1<<20) // 1MB past a 4-byte allocation
	// _ = (*byte)(past)
}

// --- Data Races ---

// dataRace writes to a shared variable from two goroutines without
// synchronization.
func dataRace() {
	// var shared int
	// go func() { shared = 1 }()
	// go func() { shared = 2 }()
	// _ = shared
}

// --- Uninitialized Reads ---

// uninitializedRead reads from a heap allocation before writing to it.
// Only detected when -init flag is enabled.
func uninitializedRead() {
	// type T struct{ x, y int }
	// p := (*T)(unsafe.Pointer(new(T)))
	// _ = p.x  // uninitialized read (with -init)
}
