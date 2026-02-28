// Benchmark tests for detector hot paths (issue #107).
//
// Run: GOEXPERIMENT=arenas go test -bench=. -benchmem ./pkg/detector/
package detector

import (
	"testing"

	"github.com/scttfrdmn/giri/pkg/shadow"
)

// BenchmarkRaceDetectorNoRace measures the common (no-race) access path.
func BenchmarkRaceDetectorNoRace(b *testing.B) {
	mem := shadow.NewMemory()
	id := mem.Allocate(shadow.AllocHeap, 64, "*int", "bench:alloc")
	ptr := &shadow.Pointer{Alloc: id, Offset: 0}
	d := NewRaceDetector()
	clock := map[int64]uint64{1: 1}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		clock[1] = uint64(i) + 1
		_ = d.CheckAccess(mem, ptr, 8, shadow.AccessRead, "bench:read", 1, clock)
	}
}

// BenchmarkRegistryCheckAccess measures all detectors dispatched via Registry.
func BenchmarkRegistryCheckAccess(b *testing.B) {
	mem := shadow.NewMemory()
	id := mem.Allocate(shadow.AllocHeap, 64, "*int", "bench:alloc")
	ptr := &shadow.Pointer{Alloc: id, Offset: 0}
	reg := DefaultRegistry()
	clock := map[int64]uint64{1: 1}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		clock[1] = uint64(i) + 1
		_ = reg.CheckAccess(mem, ptr, 8, shadow.AccessRead, "bench:read", 1, clock)
	}
}
