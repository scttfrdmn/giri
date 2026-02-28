// Benchmark tests for shadow memory hot paths (issue #107).
//
// Run: GOEXPERIMENT=arenas go test -bench=. -benchmem ./pkg/shadow/
package shadow

import (
	"testing"
)

// BenchmarkAllocate measures the cost of allocating a heap object.
func BenchmarkAllocate(b *testing.B) {
	m := NewMemory()
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = m.Allocate(AllocHeap, 64, "*int", "bench:alloc")
	}
}

// BenchmarkCheckAccessValid measures a successful in-bounds access.
func BenchmarkCheckAccessValid(b *testing.B) {
	m := NewMemory()
	id := m.Allocate(AllocHeap, 64, "*int", "bench:alloc")
	ptr := &Pointer{Alloc: id, Offset: 0}
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = m.CheckAccess(ptr, 8, AccessRead, "bench:read", 1)
	}
}

// BenchmarkCheckAccessOOB measures an out-of-bounds access (violation path).
func BenchmarkCheckAccessOOB(b *testing.B) {
	m := NewMemory()
	id := m.Allocate(AllocHeap, 8, "*int", "bench:alloc")
	ptr := &Pointer{Alloc: id, Offset: 7} // straddles boundary
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = m.CheckAccess(ptr, 8, AccessRead, "bench:oob", 1)
	}
}

// BenchmarkMarkInitialized measures the initialization tracking hot path.
func BenchmarkMarkInitialized(b *testing.B) {
	m := NewMemory(WithInitTracking())
	id := m.Allocate(AllocHeap, 64, "*int", "bench:alloc")
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		m.MarkInitialized(id, 0, 64)
	}
}

// BenchmarkAllocateFree measures the full alloc → free lifecycle.
func BenchmarkAllocateFree(b *testing.B) {
	m := NewMemory()
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		id := m.Allocate(AllocHeap, 64, "*int", "bench:alloc")
		_ = m.Free(id, "bench:free")
	}
}

// BenchmarkCheckAccessContended measures concurrent read access from N goroutines.
func BenchmarkCheckAccessContended(b *testing.B) {
	m := NewMemory()
	id := m.Allocate(AllocHeap, 64, "*int", "bench:alloc")
	ptr := &Pointer{Alloc: id, Offset: 0}

	b.ResetTimer()
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = m.CheckAccess(ptr, 8, AccessRead, "bench:par", 1)
		}
	})
}
