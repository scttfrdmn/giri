package shadow

import (
	"sync"
	"testing"
)

// --- Helpers ---

func newTestMemory() *Memory {
	return NewMemory()
}

func newVerboseMemory() *Memory {
	return NewMemory(WithVerbose())
}

func newInitMemory() *Memory {
	return NewMemory(WithInitTracking())
}

// --- Allocate ---

func TestAllocate_Basic(t *testing.T) {
	m := newTestMemory()
	id := m.Allocate(AllocHeap, 64, "*int", "test:1")
	if id == 0 {
		t.Fatal("expected non-zero AllocID")
	}
	alloc, ok := m.GetAllocation(id)
	if !ok {
		t.Fatal("allocation not found after Allocate")
	}
	if alloc.Size != 64 {
		t.Errorf("want size 64, got %d", alloc.Size)
	}
	if alloc.Kind != AllocHeap {
		t.Errorf("want AllocHeap, got %v", alloc.Kind)
	}
	if alloc.TypeName != "*int" {
		t.Errorf("want *int, got %q", alloc.TypeName)
	}
}

func TestAllocate_Kinds(t *testing.T) {
	m := newTestMemory()
	kinds := []AllocKind{AllocHeap, AllocStack, AllocArena, AllocGlobal, AllocUnsafe}
	for _, k := range kinds {
		id := m.Allocate(k, 8, "T", "site")
		alloc, ok := m.GetAllocation(id)
		if !ok {
			t.Fatalf("allocation %v not found", k)
		}
		if alloc.Kind != k {
			t.Errorf("want %v, got %v", k, alloc.Kind)
		}
	}
}

func TestAllocate_MonotonicallyIncreasing(t *testing.T) {
	m := newTestMemory()
	var prev AllocID
	for i := 0; i < 10; i++ {
		id := m.Allocate(AllocHeap, 8, "T", "site")
		if id <= prev {
			t.Errorf("non-monotone IDs: %d then %d", prev, id)
		}
		prev = id
	}
}

// --- Free ---

func TestFree_Basic(t *testing.T) {
	m := newTestMemory()
	id := m.Allocate(AllocHeap, 8, "T", "site:1")
	if err := m.Free(id, "site:2"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	alloc, ok := m.GetAllocation(id)
	if !ok {
		t.Fatal("allocation should still exist after free")
	}
	if !alloc.Freed {
		t.Error("allocation should be marked freed")
	}
	if alloc.FreeSite != "site:2" {
		t.Errorf("want FreeSite site:2, got %q", alloc.FreeSite)
	}
}

func TestFree_DoubleFree(t *testing.T) {
	m := newTestMemory()
	id := m.Allocate(AllocHeap, 8, "T", "alloc:1")
	if err := m.Free(id, "free:1"); err != nil {
		t.Fatalf("first free unexpected error: %v", err)
	}
	err := m.Free(id, "free:2")
	if err == nil {
		t.Fatal("expected DoubleFreeError, got nil")
	}
	dfe, ok := err.(*DoubleFreeError)
	if !ok {
		t.Fatalf("expected *DoubleFreeError, got %T: %v", err, err)
	}
	if dfe.AllocID != id {
		t.Errorf("want AllocID %d, got %d", id, dfe.AllocID)
	}
	if dfe.FirstFree != "free:1" {
		t.Errorf("want FirstFree free:1, got %q", dfe.FirstFree)
	}
}

// --- CheckAccess ---

func TestCheckAccess_Valid(t *testing.T) {
	m := newTestMemory()
	id := m.Allocate(AllocHeap, 64, "T", "alloc:1")
	ptr := &Pointer{Alloc: id, Offset: 0}
	if err := m.CheckAccess(ptr, 8, AccessRead, "read:1", 1); err != nil {
		t.Errorf("expected nil, got %v", err)
	}
}

func TestCheckAccess_UseAfterFree(t *testing.T) {
	m := newTestMemory()
	id := m.Allocate(AllocHeap, 64, "T", "alloc:1")
	_ = m.Free(id, "free:1")
	ptr := &Pointer{Alloc: id, Offset: 0}
	err := m.CheckAccess(ptr, 8, AccessRead, "access:1", 1)
	if err == nil {
		t.Fatal("expected UseAfterFreeError, got nil")
	}
	if _, ok := err.(*UseAfterFreeError); !ok {
		t.Fatalf("expected *UseAfterFreeError, got %T", err)
	}
}

func TestCheckAccess_OutOfBounds_Overflow(t *testing.T) {
	m := newTestMemory()
	id := m.Allocate(AllocHeap, 16, "T", "alloc:1")
	ptr := &Pointer{Alloc: id, Offset: 12}
	err := m.CheckAccess(ptr, 8, AccessRead, "access:1", 1) // 12+8=20 > 16
	if err == nil {
		t.Fatal("expected OutOfBoundsError, got nil")
	}
	if _, ok := err.(*OutOfBoundsError); !ok {
		t.Fatalf("expected *OutOfBoundsError, got %T", err)
	}
}

func TestCheckAccess_OutOfBounds_Negative(t *testing.T) {
	m := newTestMemory()
	id := m.Allocate(AllocHeap, 16, "T", "alloc:1")
	ptr := &Pointer{Alloc: id, Offset: -1}
	err := m.CheckAccess(ptr, 1, AccessRead, "access:1", 1)
	if err == nil {
		t.Fatal("expected OutOfBoundsError, got nil")
	}
	if _, ok := err.(*OutOfBoundsError); !ok {
		t.Fatalf("expected *OutOfBoundsError, got %T", err)
	}
}

// --- Initialization Tracking ---

func TestCheckAccess_UninitRead(t *testing.T) {
	m := newInitMemory()
	id := m.Allocate(AllocHeap, 8, "T", "alloc:1")
	ptr := &Pointer{Alloc: id, Offset: 0}
	err := m.CheckAccess(ptr, 1, AccessRead, "read:1", 1)
	if err == nil {
		t.Fatal("expected UninitializedReadError, got nil")
	}
	if _, ok := err.(*UninitializedReadError); !ok {
		t.Fatalf("expected *UninitializedReadError, got %T", err)
	}
}

func TestMarkInitialized_ClearsError(t *testing.T) {
	m := newInitMemory()
	id := m.Allocate(AllocHeap, 8, "T", "alloc:1")
	ptr := &Pointer{Alloc: id, Offset: 0}
	m.MarkInitialized(id, 0, 8)
	if err := m.CheckAccess(ptr, 8, AccessRead, "read:1", 1); err != nil {
		t.Errorf("after MarkInitialized: expected nil, got %v", err)
	}
}

// --- Arena Lifecycle ---

func TestFreeArena_PoisonsAllocations(t *testing.T) {
	m := newTestMemory()
	arenaID := m.CreateArena("arena:1")
	allocID := m.AllocateInArena(arenaID, 16, "T", "alloc:1")
	errs := m.FreeArena(arenaID, "free:1")
	if len(errs) != 0 {
		t.Fatalf("unexpected errors freeing arena: %v", errs)
	}
	alloc, ok := m.GetAllocation(allocID)
	if !ok {
		t.Fatal("allocation not found")
	}
	if !alloc.Freed {
		t.Error("arena allocation should be freed after FreeArena")
	}
}

func TestFreeArena_DoubleFree(t *testing.T) {
	m := newTestMemory()
	arenaID := m.CreateArena("create:1")
	m.FreeArena(arenaID, "free:1")
	errs := m.FreeArena(arenaID, "free:2")
	if len(errs) == 0 {
		t.Fatal("expected ArenaDoubleFreeError, got none")
	}
	if _, ok := errs[0].(*ArenaDoubleFreeError); !ok {
		t.Fatalf("expected *ArenaDoubleFreeError, got %T", errs[0])
	}
}

// --- DerivePointer ---

func TestDerivePointer(t *testing.T) {
	m := newTestMemory()
	id := m.Allocate(AllocHeap, 64, "T", "alloc:1")
	base := &Pointer{Alloc: id, Offset: 0}
	derived := m.DerivePointer(base, 16)
	if derived.Alloc != id {
		t.Errorf("want AllocID %d, got %d", id, derived.Alloc)
	}
	if derived.Offset != 16 {
		t.Errorf("want Offset 16, got %d", derived.Offset)
	}
	if derived.DerivedFrom != base {
		t.Error("DerivedFrom should point to base pointer")
	}
}

// --- Stats ---

func TestStats(t *testing.T) {
	m := newTestMemory()
	id1 := m.Allocate(AllocHeap, 32, "A", "s:1")
	m.Allocate(AllocHeap, 64, "B", "s:2")
	m.Free(id1, "s:3")

	stats := m.Stats()
	if stats.TotalAllocations != 2 {
		t.Errorf("want 2 total, got %d", stats.TotalAllocations)
	}
	if stats.LiveAllocations != 1 {
		t.Errorf("want 1 live, got %d", stats.LiveAllocations)
	}
	if stats.FreedAllocations != 1 {
		t.Errorf("want 1 freed, got %d", stats.FreedAllocations)
	}
	if stats.LiveBytes != 64 {
		t.Errorf("want 64 live bytes, got %d", stats.LiveBytes)
	}
}

// --- Verbose Access Logging ---

func TestVerbose_AccessLog(t *testing.T) {
	m := newVerboseMemory()
	id := m.Allocate(AllocHeap, 64, "T", "alloc:1")
	alloc, ok := m.GetAllocation(id)
	if !ok {
		t.Fatal("allocation not found")
	}
	if alloc.AccessLog == nil {
		t.Fatal("AccessLog should be initialized in verbose mode")
	}

	ptr := &Pointer{Alloc: id, Offset: 0}
	m.CheckAccess(ptr, 8, AccessRead, "read:1", 1)
	m.CheckAccess(ptr, 8, AccessWrite, "write:1", 1)

	// Re-fetch to get updated log
	alloc, _ = m.GetAllocation(id)
	if len(alloc.AccessLog) != 2 {
		t.Errorf("want 2 access records, got %d", len(alloc.AccessLog))
	}
}

// --- Race detector: concurrent reads should not race ---

func TestConcurrentReads_NoRace(t *testing.T) {
	m := newTestMemory()
	id := m.Allocate(AllocHeap, 64, "T", "alloc:1")
	ptr := &Pointer{Alloc: id, Offset: 0}

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			m.CheckAccess(ptr, 8, AccessRead, "read:concurrent", int64(i))
		}()
	}
	wg.Wait()
}
