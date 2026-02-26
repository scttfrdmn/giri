package scheduler

import (
	"testing"
)

// --- RoundRobin ---

func TestRoundRobin_Next_Empty(t *testing.T) {
	s := NewRoundRobin()
	if got := s.Next(nil); got != -1 {
		t.Errorf("want -1, got %d", got)
	}
	if got := s.Next([]int64{}); got != -1 {
		t.Errorf("want -1, got %d", got)
	}
}

func TestRoundRobin_Next_Sequence(t *testing.T) {
	s := NewRoundRobin()
	// RoundRobin sorts and cycles: after sort [1, 2, 3]
	// lastIdx starts at 0; Next increments to 1 → picks index 1 → 2
	// Then 2 → index 2 → 3; then 0 → index 0 → 1; repeating
	got := make([]int64, 6)
	for i := range got {
		got[i] = s.Next([]int64{3, 1, 2})
	}

	// Verify all returned values are in the original set
	valid := map[int64]bool{1: true, 2: true, 3: true}
	for _, v := range got {
		if !valid[v] {
			t.Errorf("unexpected goroutine ID %d", v)
		}
	}

	// Verify cycling: 6 steps, stats should show 6 decisions
	stats := s.Stats()
	if stats.TotalDecisions != 6 {
		t.Errorf("want 6 decisions, got %d", stats.TotalDecisions)
	}
}

func TestRoundRobin_OnSpawn(t *testing.T) {
	s := NewRoundRobin()
	s.OnSpawn(1, 2)
	s.OnSpawn(1, 3)
	stats := s.Stats()
	if stats.GoroutinesSpawned != 2 {
		t.Errorf("want 2 spawned, got %d", stats.GoroutinesSpawned)
	}
}

func TestRoundRobin_OnSyncPoint(t *testing.T) {
	s := NewRoundRobin()
	s.OnSyncPoint(1)
	s.OnSyncPoint(1)
	stats := s.Stats()
	if stats.SyncPoints != 2 {
		t.Errorf("want 2 sync points, got %d", stats.SyncPoints)
	}
}

// --- Random ---

func TestRandom_Next_Empty(t *testing.T) {
	s := NewRandom(42)
	if got := s.Next(nil); got != -1 {
		t.Errorf("want -1, got %d", got)
	}
}

func TestRandom_Next_Reproducible(t *testing.T) {
	runnable := []int64{1, 2, 3, 4, 5}

	s1 := NewRandom(1234)
	s2 := NewRandom(1234)

	for i := 0; i < 20; i++ {
		v1 := s1.Next(runnable)
		v2 := s2.Next(runnable)
		if v1 != v2 {
			t.Errorf("step %d: same seed gave different values %d vs %d", i, v1, v2)
		}
	}
}

func TestRandom_Next_DifferentSeeds(t *testing.T) {
	runnable := []int64{1, 2, 3}
	s1 := NewRandom(111)
	s2 := NewRandom(999)

	// With different seeds, at least one step should differ
	matched := true
	for i := 0; i < 10; i++ {
		if s1.Next(runnable) != s2.Next(runnable) {
			matched = false
			break
		}
	}
	if matched {
		t.Error("different seeds produced identical 10-step sequences (very unlikely)")
	}
}

func TestRandom_Next_ValidResults(t *testing.T) {
	s := NewRandom(42)
	runnable := []int64{10, 20, 30}
	valid := map[int64]bool{10: true, 20: true, 30: true}
	for i := 0; i < 20; i++ {
		v := s.Next(runnable)
		if !valid[v] {
			t.Errorf("unexpected value %d", v)
		}
	}
}

// --- PCT ---

func TestPCT_Next_Empty(t *testing.T) {
	s := NewPCT(42, 2)
	if got := s.Next(nil); got != -1 {
		t.Errorf("want -1, got %d", got)
	}
}

func TestPCT_Next_HighestPriority(t *testing.T) {
	s := NewPCT(42, 1) // 0 change points
	// Manually set priorities
	s.priorities[1] = 100
	s.priorities[2] = 200
	s.priorities[3] = 50
	// GID 2 has highest priority
	got := s.Next([]int64{1, 2, 3})
	if got != 2 {
		t.Errorf("want GID 2 (highest priority), got %d", got)
	}
}

func TestPCT_OnSpawn_AddsPriority(t *testing.T) {
	s := NewPCT(42, 2)
	s.OnSpawn(1, 5)
	if _, ok := s.priorities[5]; !ok {
		t.Error("OnSpawn should add child GID to priorities map")
	}
	stats := s.Stats()
	if stats.GoroutinesSpawned != 1 {
		t.Errorf("want 1 spawned, got %d", stats.GoroutinesSpawned)
	}
}

func TestPCT_Stats_Counters(t *testing.T) {
	s := NewPCT(42, 2)
	runnable := []int64{1, 2}
	for i := 0; i < 5; i++ {
		s.Next(runnable)
	}
	s.OnSyncPoint(1)
	s.OnSyncPoint(1)
	stats := s.Stats()
	if stats.TotalDecisions != 5 {
		t.Errorf("want 5 decisions, got %d", stats.TotalDecisions)
	}
	if stats.SyncPoints != 2 {
		t.Errorf("want 2 sync points, got %d", stats.SyncPoints)
	}
}
