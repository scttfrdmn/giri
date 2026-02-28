// Fuzz tests for the shadow memory system (issue #106).
//
// Run seed corpus only (CI):
//
//	go test -run=FuzzXxx ./pkg/shadow/
//
// Run full fuzzer (local):
//
//	go test -fuzz=FuzzAllocateCheckAccess -fuzztime=30s ./pkg/shadow/
package shadow

import (
	"testing"
)

// FuzzAllocateCheckAccess fuzzes Allocate → CheckAccess → Free sequences
// with random sizes, offsets, and access widths. The invariant is that no
// panic should occur; the function returns nil or a typed error.
func FuzzAllocateCheckAccess(f *testing.F) {
	// Seed corpus: representative (size, offset, accessSize) triples.
	seeds := [][3]int{
		{8, 0, 8},    // exact-fit read
		{64, 0, 8},   // normal heap read
		{64, 56, 8},  // last 8 bytes
		{64, 57, 8},  // one byte OOB
		{1, 0, 1},    // minimal allocation
		{100, 99, 2}, // straddles boundary
		{0, 0, 1},    // zero-size allocation
	}
	for _, s := range seeds {
		f.Add(s[0], s[1], s[2])
	}

	f.Fuzz(func(t *testing.T, size, offset, accessSize int) {
		// Clamp inputs to reasonable ranges to avoid allocation explosions.
		if size < 0 {
			size = -size
		}
		if size > 1<<20 {
			size = 1 << 20
		}
		if accessSize <= 0 {
			accessSize = 1
		}
		if accessSize > 256 {
			accessSize = 256
		}

		m := NewMemory()
		id := m.Allocate(AllocHeap, size, "fuzz.T", "fuzz:1")

		ptr := &Pointer{Alloc: id, Offset: offset}

		// Must not panic; may return nil or an error.
		_ = m.CheckAccess(ptr, accessSize, AccessRead, "fuzz:read", 1)
		_ = m.CheckAccess(ptr, accessSize, AccessWrite, "fuzz:write", 1)

		// Free should not panic either.
		_ = m.Free(id, "fuzz:free")

		// Access after free — must not panic, should return UseAfterFreeError.
		_ = m.CheckAccess(ptr, accessSize, AccessRead, "fuzz:uaf", 1)
	})
}

// FuzzMarkInitializedCheckAccess fuzzes the initialization tracking path.
// Invariant: no panic; MarkInitialized followed by CheckAccess is always safe.
func FuzzMarkInitializedCheckAccess(f *testing.F) {
	// Seed corpus: (allocSize, markOffset, markSize, accessOffset, accessSize).
	f.Add(64, 0, 8, 0, 8)
	f.Add(64, 8, 16, 0, 8)   // read uninitialized prefix
	f.Add(64, 0, 64, 32, 16) // fully initialized, partial read
	f.Add(1, 0, 1, 0, 1)     // minimal
	f.Add(32, 0, 0, 0, 8)    // zero-size mark

	f.Fuzz(func(t *testing.T, allocSize, markOff, markSz, accOff, accSz int) {
		if allocSize <= 0 || allocSize > 1<<16 {
			allocSize = 64
		}
		if markSz < 0 {
			markSz = 0
		}
		if markSz > allocSize {
			markSz = allocSize
		}
		if accSz <= 0 {
			accSz = 1
		}
		if accSz > 256 {
			accSz = 256
		}

		m := NewMemory(WithInitTracking())
		id := m.Allocate(AllocHeap, allocSize, "fuzz.T", "fuzz:1")
		ptr := &Pointer{Alloc: id, Offset: accOff}

		m.MarkInitialized(id, markOff, markSz)

		// Must not panic.
		_ = m.CheckAccess(ptr, accSz, AccessRead, "fuzz:read", 1)
	})
}

// FuzzDerivePointer fuzzes pointer arithmetic: base + offset must not panic.
func FuzzDerivePointer(f *testing.F) {
	f.Add(64, 0, 8)
	f.Add(64, 32, 8)
	f.Add(64, -1, 8)
	f.Add(64, 64, 1)

	f.Fuzz(func(t *testing.T, allocSize, deriveOffset, checkOffset int) {
		if allocSize <= 0 || allocSize > 1<<16 {
			allocSize = 64
		}

		m := NewMemory()
		id := m.Allocate(AllocHeap, allocSize, "fuzz.T", "fuzz:1")
		base := &Pointer{Alloc: id, Offset: 0}

		// DerivePointer must not panic.
		derived := m.DerivePointer(base, deriveOffset)
		if derived == nil {
			return
		}

		ptr := &Pointer{Alloc: derived.Alloc, Offset: checkOffset}
		_ = m.CheckAccess(ptr, 1, AccessRead, "fuzz:read", 1)
	})
}
