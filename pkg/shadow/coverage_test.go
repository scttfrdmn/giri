// Additional unit tests to raise coverage for previously-untested functions
// in pkg/shadow (issue #108): Poison, TrackPointer, GetProvenance,
// WithPointerArena, GetArena, LiveArenas, LiveAllocations, Stats.String.
package shadow

import (
	"strings"
	"testing"
)

// --- Poison ---

func TestPoison_BlocksAccess(t *testing.T) {
	m := NewMemory()
	id := m.Allocate(AllocStack, 32, "*int", "p:alloc")
	m.Poison(id, "p:poison")

	ptr := &Pointer{Alloc: id, Offset: 0}
	err := m.CheckAccess(ptr, 8, AccessRead, "p:read", 1)
	if err == nil {
		t.Fatal("expected error after Poison, got nil")
	}
}

func TestPoison_UnknownID(t *testing.T) {
	m := NewMemory()
	// Poisoning a non-existent ID should not panic.
	m.Poison(AllocID(9999), "p:no-op")
}

// --- TrackPointer / GetProvenance ---

func TestTrackPointer_GetProvenance(t *testing.T) {
	m := NewMemory()
	id := m.Allocate(AllocHeap, 64, "*T", "tp:alloc")
	ptr := &Pointer{Alloc: id, Offset: 8}

	m.TrackPointer("uintptr:deadbeef", ptr)

	got, ok := m.GetProvenance("uintptr:deadbeef")
	if !ok {
		t.Fatal("GetProvenance returned !ok for tracked pointer")
	}
	if got.Alloc != id || got.Offset != 8 {
		t.Errorf("GetProvenance: got alloc=%d offset=%d, want alloc=%d offset=8",
			got.Alloc, got.Offset, id)
	}
}

func TestGetProvenance_Unknown(t *testing.T) {
	m := NewMemory()
	got, ok := m.GetProvenance("unknown:key")
	if ok || got != nil {
		t.Errorf("GetProvenance of unknown key should be (nil,false), got (%v,%v)", got, ok)
	}
}

// WithPointerArena requires a *safearena.Arena argument (which requires
// GOEXPERIMENT=arenas and non-nil arena), so we test the rest of the
// GetArena / LiveArenas / LiveAllocations surface without it.

// --- GetArena ---

func TestGetArena_Exists(t *testing.T) {
	m := NewMemory()
	arenaID := m.CreateArena("ga:create")

	a, ok := m.GetArena(arenaID)
	if !ok || a == nil {
		t.Fatal("GetArena returned !ok for live arena")
	}
	if a.ID != arenaID {
		t.Errorf("GetArena: got ID %d, want %d", a.ID, arenaID)
	}
}

func TestGetArena_NotExists(t *testing.T) {
	m := NewMemory()
	a, ok := m.GetArena(ArenaID(9999))
	if ok || a != nil {
		t.Errorf("GetArena of unknown ID should be (nil,false), got (%v,%v)", a, ok)
	}
}

// --- LiveArenas ---

func TestLiveArenas(t *testing.T) {
	m := NewMemory()
	if len(m.LiveArenas()) != 0 {
		t.Fatal("expected 0 live arenas initially")
	}

	id1 := m.CreateArena("la:1")
	id2 := m.CreateArena("la:2")

	arenas := m.LiveArenas()
	if len(arenas) != 2 {
		t.Fatalf("expected 2 live arenas, got %d", len(arenas))
	}

	// Free one arena and verify the count drops.
	if err := m.FreeArena(id1, "la:free"); err != nil {
		t.Fatalf("FreeArena: %v", err)
	}
	arenas = m.LiveArenas()
	if len(arenas) != 1 {
		t.Errorf("expected 1 live arena after free, got %d", len(arenas))
	}
	if arenas[0].ID != id2 {
		t.Errorf("expected remaining arena ID %d, got %d", id2, arenas[0].ID)
	}
}

// --- LiveAllocations ---

func TestLiveAllocations(t *testing.T) {
	m := NewMemory()
	if len(m.LiveAllocations()) != 0 {
		t.Fatal("expected 0 live allocations initially")
	}

	id1 := m.Allocate(AllocHeap, 32, "*A", "lv:1")
	id2 := m.Allocate(AllocHeap, 64, "*B", "lv:2")

	live := m.LiveAllocations()
	if len(live) != 2 {
		t.Fatalf("expected 2 live allocations, got %d", len(live))
	}

	if err := m.Free(id1, "lv:free"); err != nil {
		t.Fatalf("Free: %v", err)
	}
	live = m.LiveAllocations()
	if len(live) != 1 {
		t.Errorf("expected 1 live allocation after free, got %d", len(live))
	}
	if live[0].ID != id2 {
		t.Errorf("expected remaining alloc ID %d, got %d", id2, live[0].ID)
	}
}

// --- Stats.String ---

func TestStats_String(t *testing.T) {
	m := NewMemory()
	m.Allocate(AllocHeap, 32, "*A", "ss:1")
	m.Allocate(AllocHeap, 64, "*B", "ss:2")

	s := m.Stats()
	str := s.String()
	if !strings.Contains(str, "2") {
		t.Errorf("Stats.String() should mention allocation count; got %q", str)
	}
}
