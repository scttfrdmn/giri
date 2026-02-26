package detector

import (
	"testing"

	"github.com/scttfrdmn/giri/pkg/shadow"
)

// makeAlloc creates an allocation and returns a pointer to it.
func makeAlloc(t *testing.T, mem *shadow.Memory, kind shadow.AllocKind, size int) *shadow.Pointer {
	t.Helper()
	id := mem.Allocate(kind, size, "T", "test:1")
	return &shadow.Pointer{Alloc: id, Offset: 0}
}

func makeArenaAlloc(t *testing.T, mem *shadow.Memory, arenaID shadow.ArenaID, size int) *shadow.Pointer {
	t.Helper()
	id := mem.AllocateInArena(arenaID, size, "T", "test:1")
	return &shadow.Pointer{Alloc: id, Offset: 0}
}

// --- ArenaDetector ---

func TestArenaDetector_UAF(t *testing.T) {
	mem := shadow.NewMemory()
	d := &ArenaDetector{}
	arenaID := mem.CreateArena("create:1")
	ptr := makeArenaAlloc(t, mem, arenaID, 16)
	mem.FreeArena(arenaID, "free:1")

	err := d.CheckAccess(mem, ptr, 8, shadow.AccessRead, "access:1", 1)
	if err == nil {
		t.Fatal("expected UseAfterFreeError for arena allocation after arena free")
	}
	if _, ok := err.(*shadow.UseAfterFreeError); !ok {
		t.Fatalf("expected *UseAfterFreeError, got %T", err)
	}
}

func TestArenaDetector_CleanOnHeap(t *testing.T) {
	mem := shadow.NewMemory()
	d := &ArenaDetector{}
	ptr := makeAlloc(t, mem, shadow.AllocHeap, 16)

	err := d.CheckAccess(mem, ptr, 8, shadow.AccessRead, "access:1", 1)
	if err != nil {
		t.Errorf("unexpected error for heap allocation: %v", err)
	}
}

func TestArenaDetector_Finalize_Leak(t *testing.T) {
	mem := shadow.NewMemory()
	d := &ArenaDetector{}
	mem.CreateArena("create:1")

	errs := d.CheckFinalize(mem)
	if len(errs) == 0 {
		t.Fatal("expected arena leak error, got none")
	}
}

func TestArenaDetector_Finalize_NoLeak(t *testing.T) {
	mem := shadow.NewMemory()
	d := &ArenaDetector{}
	arenaID := mem.CreateArena("create:1")
	mem.FreeArena(arenaID, "free:1")

	errs := d.CheckFinalize(mem)
	if len(errs) != 0 {
		t.Errorf("unexpected errors: %v", errs)
	}
}

// --- BoundsDetector ---

func TestBoundsDetector_InBounds(t *testing.T) {
	mem := shadow.NewMemory()
	d := &BoundsDetector{}
	ptr := makeAlloc(t, mem, shadow.AllocHeap, 16)

	err := d.CheckAccess(mem, ptr, 8, shadow.AccessRead, "access:1", 1)
	if err != nil {
		t.Errorf("unexpected error for in-bounds access: %v", err)
	}
}

func TestBoundsDetector_OutOfBounds(t *testing.T) {
	mem := shadow.NewMemory()
	d := &BoundsDetector{}
	id := mem.Allocate(shadow.AllocHeap, 8, "T", "alloc:1")
	ptr := &shadow.Pointer{Alloc: id, Offset: 6}

	err := d.CheckAccess(mem, ptr, 4, shadow.AccessRead, "access:1", 1) // 6+4=10 > 8
	if err == nil {
		t.Fatal("expected OutOfBoundsError, got nil")
	}
	if _, ok := err.(*shadow.OutOfBoundsError); !ok {
		t.Fatalf("expected *OutOfBoundsError, got %T", err)
	}
}

// --- UnsafeDetector ---

func TestUnsafeDetector_DerivedWithOOB(t *testing.T) {
	mem := shadow.NewMemory()
	d := NewUnsafeDetector()
	id := mem.Allocate(shadow.AllocHeap, 8, "T", "alloc:1")
	base := &shadow.Pointer{Alloc: id, Offset: 0}
	derived := mem.DerivePointer(base, 100) // Way out of bounds

	err := d.CheckAccess(mem, derived, 1, shadow.AccessRead, "access:1", 1)
	if err == nil {
		t.Fatal("expected UnsafePointerViolation for out-of-bounds derived pointer, got nil")
	}
	uviol, ok := err.(*shadow.UnsafePointerViolation)
	if !ok {
		t.Fatalf("expected *UnsafePointerViolation, got %T", err)
	}
	if uviol.Rule != shadow.RuleArithmetic {
		t.Errorf("want RuleArithmetic, got %v", uviol.Rule)
	}
}

func TestUnsafeDetector_NoDerivedInBounds(t *testing.T) {
	mem := shadow.NewMemory()
	d := NewUnsafeDetector()
	id := mem.Allocate(shadow.AllocHeap, 64, "T", "alloc:1")
	base := &shadow.Pointer{Alloc: id, Offset: 0}
	derived := mem.DerivePointer(base, 8)

	err := d.CheckAccess(mem, derived, 4, shadow.AccessRead, "access:1", 1)
	if err != nil {
		t.Errorf("unexpected error for in-bounds derived pointer: %v", err)
	}
}

func TestUnsafeDetector_CheckGCPoint_PendingUintptr(t *testing.T) {
	mem := shadow.NewMemory()
	d := NewUnsafeDetector()
	id := mem.Allocate(shadow.AllocHeap, 8, "T", "alloc:1")
	ptr := &shadow.Pointer{Alloc: id, Offset: 0}
	d.RecordUintptrConversion("val1", "conv:1", ptr)

	errs := d.CheckGCPoint("gc:1")
	if len(errs) == 0 {
		t.Fatal("expected uintptr violation at GC point, got none")
	}
	uviol, ok := errs[0].(*shadow.UnsafePointerViolation)
	if !ok {
		t.Fatalf("expected *UnsafePointerViolation, got %T", errs[0])
	}
	if uviol.Rule != shadow.RuleUintptr {
		t.Errorf("want RuleUintptr, got %v", uviol.Rule)
	}
}

func TestUnsafeDetector_CheckGCPoint_AfterClear(t *testing.T) {
	d := NewUnsafeDetector()
	d.RecordUintptrConversion("val1", "conv:1", nil)
	d.ClearUintptrConversion("val1")

	errs := d.CheckGCPoint("gc:1")
	if len(errs) != 0 {
		t.Errorf("expected no errors after clearing, got %v", errs)
	}
}

// --- RaceDetector ---

func TestRaceDetector_SameGoroutine_NoRace(t *testing.T) {
	mem := shadow.NewMemory()
	d := NewRaceDetector()
	id := mem.Allocate(shadow.AllocHeap, 8, "T", "alloc:1")
	ptr := &shadow.Pointer{Alloc: id, Offset: 0}

	d.CheckAccess(mem, ptr, 8, shadow.AccessWrite, "write:1", 1)
	err := d.CheckAccess(mem, ptr, 8, shadow.AccessRead, "read:1", 1)
	if err != nil {
		t.Errorf("unexpected race error for same goroutine: %v", err)
	}
}

func TestRaceDetector_DifferentGoroutines_Race(t *testing.T) {
	mem := shadow.NewMemory()
	d := NewRaceDetector()
	id := mem.Allocate(shadow.AllocHeap, 8, "T", "alloc:1")
	ptr := &shadow.Pointer{Alloc: id, Offset: 0}

	d.CheckAccess(mem, ptr, 8, shadow.AccessWrite, "write:1", 1)
	err := d.CheckAccess(mem, ptr, 8, shadow.AccessRead, "read:1", 2)
	if err == nil {
		t.Fatal("expected DataRaceError for concurrent read-write, got nil")
	}
	if _, ok := err.(*shadow.DataRaceError); !ok {
		t.Fatalf("expected *DataRaceError, got %T", err)
	}
}

// --- Registry ---

func TestRegistry_CheckAccess_CollectsAll(t *testing.T) {
	mem := shadow.NewMemory()
	// Set up: arena allocation freed (triggers ArenaDetector), also out of bounds (BoundsDetector)
	arenaID := mem.CreateArena("create:1")
	id := mem.AllocateInArena(arenaID, 4, "T", "alloc:1")
	mem.FreeArena(arenaID, "free:1")
	ptr := &shadow.Pointer{Alloc: id, Offset: 8} // OOB by 4 bytes

	r := NewRegistry(&ArenaDetector{}, &BoundsDetector{})
	errs := r.CheckAccess(mem, ptr, 1, shadow.AccessRead, "access:1", 1)
	// Expect at least 2 errors (UAF + OOB)
	if len(errs) < 2 {
		t.Errorf("expected >= 2 errors, got %d: %v", len(errs), errs)
	}
}

func TestRegistry_Finalize_CallsAll(t *testing.T) {
	mem := shadow.NewMemory()
	mem.CreateArena("leak:1") // Not freed → leak

	r := NewRegistry(&ArenaDetector{})
	errs := r.Finalize(mem)
	if len(errs) == 0 {
		t.Fatal("expected arena leak error from Finalize, got none")
	}
}
