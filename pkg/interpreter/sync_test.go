package interpreter

import (
	"errors"
	"go/token"
	"testing"

	"github.com/scttfrdmn/giri/pkg/shadow"
)

// syncFixture is an interpreter with goroutines 1 (main), 2 and 3.
func syncFixture(t *testing.T) (*Interpreter, [4]*Goroutine) {
	t.Helper()
	interp := New(token.NewFileSet(), DefaultConfig())
	var gs [4]*Goroutine
	for i := 1; i <= 3; i++ {
		gid, err := interp.spawnGoroutine("g", "")
		if err != nil {
			t.Fatal(err)
		}
		gs[i] = interp.goroutine(gid)
	}
	return interp, gs
}

// work advances g's own clock, as accesses between sync operations do, and
// returns the new value.
func work(g *Goroutine) uint64 {
	g.VClock.Tick(g.ID)
	return g.VClock.Clocks[g.ID]
}

func unlockErrors(interp *Interpreter) int {
	n := 0
	for _, v := range interp.violations {
		var e *shadow.MutexUnlockError
		if errors.As(v, &e) {
			n++
		}
	}
	return n
}

// TestWaitGroupJoinsAllDones: Wait must merge every Done, not only the last
// one. Before #293 each Done overwrote the release clock, so the first
// worker's writes looked concurrent with the code after Wait.
func TestWaitGroupJoinsAllDones(t *testing.T) {
	interp, g := syncFixture(t)
	ms := &mutexState{}
	interp.syncWaitGroup(1, g[1], ms, "Add", []Value{{}, {Raw: int64(2)}}, "")

	w2 := work(g[2])
	interp.syncWaitGroup(2, g[2], ms, "Done", nil, "")
	w3 := work(g[3])
	interp.syncWaitGroup(3, g[3], ms, "Done", nil, "")
	interp.syncWaitGroup(1, g[1], ms, "Wait", nil, "")

	if got := g[1].VClock.Clocks[2]; got < w2 {
		t.Errorf("after Wait, main's clock for goroutine 2 = %d, want >= %d (first Done lost)", got, w2)
	}
	if got := g[1].VClock.Clocks[3]; got < w3 {
		t.Errorf("after Wait, main's clock for goroutine 3 = %d, want >= %d", got, w3)
	}
	if ms.wgCounter != 0 {
		t.Errorf("counter = %d, want 0", ms.wgCounter)
	}
}

// TestWaitGroupNegativeAddReleases: Add(-n) is a release, like Done.
func TestWaitGroupNegativeAddReleases(t *testing.T) {
	interp, g := syncFixture(t)
	ms := &mutexState{}
	interp.syncWaitGroup(1, g[1], ms, "Add", []Value{{}, {Raw: int64(1)}}, "")
	w2 := work(g[2])
	interp.syncWaitGroup(2, g[2], ms, "Add", []Value{{}, {Raw: int64(-1)}}, "")
	interp.syncWaitGroup(1, g[1], ms, "Wait", nil, "")
	if got := g[1].VClock.Clocks[2]; got < w2 {
		t.Errorf("after Wait, main's clock for goroutine 2 = %d, want >= %d", got, w2)
	}
}

// TestRWMutexOverlappingReaders: two read locks may be held at once. Before
// #293 RLock and RUnlock shared the write-lock flag, so the second RUnlock
// reported a false MutexUnlockError.
func TestRWMutexOverlappingReaders(t *testing.T) {
	interp, g := syncFixture(t)
	ms := &mutexState{}
	interp.syncMutex(2, g[2], ms, "RWMutex", "RLock", "")
	interp.syncMutex(3, g[3], ms, "RWMutex", "RLock", "")
	w2 := work(g[2])
	interp.syncMutex(2, g[2], ms, "RWMutex", "RUnlock", "")
	w3 := work(g[3])
	interp.syncMutex(3, g[3], ms, "RWMutex", "RUnlock", "")
	if n := unlockErrors(interp); n != 0 {
		t.Fatalf("%d MutexUnlockError(s) for balanced overlapping read locks", n)
	}

	// A writer's Lock happens after every earlier RUnlock.
	interp.syncMutex(1, g[1], ms, "RWMutex", "Lock", "")
	if g[1].VClock.Clocks[2] < w2 || g[1].VClock.Clocks[3] < w3 {
		t.Errorf("Lock didn't merge both RUnlocks: clock %v, want [2]>=%d [3]>=%d", g[1].VClock.Clocks, w2, w3)
	}
}

// TestRWMutexReadersDontSync: an RLock merges the last writer's Unlock but not
// other readers' RUnlocks, since readers don't synchronize with each other.
func TestRWMutexReadersDontSync(t *testing.T) {
	interp, g := syncFixture(t)
	ms := &mutexState{}
	interp.syncMutex(1, g[1], ms, "RWMutex", "Lock", "")
	w1 := work(g[1])
	interp.syncMutex(1, g[1], ms, "RWMutex", "Unlock", "")

	interp.syncMutex(2, g[2], ms, "RWMutex", "RLock", "")
	w2 := work(g[2])
	interp.syncMutex(2, g[2], ms, "RWMutex", "RUnlock", "")

	interp.syncMutex(3, g[3], ms, "RWMutex", "RLock", "")
	if got := g[3].VClock.Clocks[1]; got < w1 {
		t.Errorf("RLock didn't merge the writer's Unlock: clock[1] = %d, want >= %d", got, w1)
	}
	if got := g[3].VClock.Clocks[2]; got >= w2 {
		t.Errorf("RLock merged another reader's RUnlock: clock[2] = %d, want < %d", got, w2)
	}
}

// TestRWMutexMismatchedUnlock: unlocking the write side of a read-locked
// RWMutex, or RUnlock with no reader, is a fatal error in Go.
func TestRWMutexMismatchedUnlock(t *testing.T) {
	interp, g := syncFixture(t)
	ms := &mutexState{}
	interp.syncMutex(2, g[2], ms, "RWMutex", "RLock", "")
	interp.syncMutex(2, g[2], ms, "RWMutex", "Unlock", "")
	if n := unlockErrors(interp); n != 1 {
		t.Errorf("RLock then Unlock: %d MutexUnlockError(s), want 1", n)
	}

	interp2, g2 := syncFixture(t)
	ms2 := &mutexState{}
	interp2.syncMutex(2, g2[2], ms2, "RWMutex", "Lock", "")
	interp2.syncMutex(2, g2[2], ms2, "RWMutex", "RUnlock", "")
	if n := unlockErrors(interp2); n != 1 {
		t.Errorf("Lock then RUnlock: %d MutexUnlockError(s), want 1", n)
	}
}
