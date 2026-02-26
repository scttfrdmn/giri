// Package scheduler implements goroutine scheduling strategies for the
// Giri interpreter. The scheduler controls the order in which goroutines
// execute, which is critical for finding concurrency bugs.
//
// Unlike Go's runtime scheduler (which is preemptive and non-deterministic),
// Giri's scheduler is cooperative and deterministic — the interpreter yields
// control at synchronization points, and the scheduler picks the next
// goroutine to run according to its strategy.
//
// Strategies range from simple (round-robin) to sophisticated (PCT) and
// can be seeded for reproducible bug reports.
package scheduler

import (
	"math/rand"
	"sort"
)

// Scheduler picks which goroutine to run next.
type Scheduler interface {
	// Next returns the ID of the next goroutine to run from the runnable set.
	// Returns -1 if no goroutines are runnable.
	Next(runnable []int64) int64

	// OnSyncPoint is called when a goroutine hits a synchronization point
	// (channel op, mutex op, atomic). This is the "preemption point" where
	// the scheduler may switch to a different goroutine.
	OnSyncPoint(currentGID int64)

	// OnSpawn is called when a new goroutine is created.
	OnSpawn(parentGID, childGID int64)

	// Stats returns scheduling statistics.
	Stats() ScheduleStats
}

// ScheduleStats tracks scheduling decisions for analysis.
type ScheduleStats struct {
	TotalDecisions    int
	ContextSwitches   int
	GoroutinesSpawned int
	SyncPoints        int
}

// --- Round Robin ---

// RoundRobin runs goroutines in deterministic order.
// Simple and fast, but won't find order-dependent bugs.
type RoundRobin struct {
	lastIdx int
	stats   ScheduleStats
}

func NewRoundRobin() *RoundRobin {
	return &RoundRobin{}
}

func (s *RoundRobin) Next(runnable []int64) int64 {
	if len(runnable) == 0 {
		return -1
	}
	sort.Slice(runnable, func(i, j int) bool { return runnable[i] < runnable[j] })
	s.lastIdx = (s.lastIdx + 1) % len(runnable)
	s.stats.TotalDecisions++
	return runnable[s.lastIdx]
}

func (s *RoundRobin) OnSyncPoint(gid int64) { s.stats.SyncPoints++ }
func (s *RoundRobin) OnSpawn(_, _ int64)    { s.stats.GoroutinesSpawned++ }
func (s *RoundRobin) Stats() ScheduleStats  { return s.stats }

// --- Random ---

// Random picks a random runnable goroutine at each decision point.
// Seeded for reproducibility — if you find a bug, you can replay it.
type Random struct {
	rng   *rand.Rand
	stats ScheduleStats
}

func NewRandom(seed int64) *Random {
	return &Random{
		rng: rand.New(rand.NewSource(seed)),
	}
}

func (s *Random) Next(runnable []int64) int64 {
	if len(runnable) == 0 {
		return -1
	}
	s.stats.TotalDecisions++
	return runnable[s.rng.Intn(len(runnable))]
}

func (s *Random) OnSyncPoint(gid int64) { s.stats.SyncPoints++ }
func (s *Random) OnSpawn(_, _ int64)    { s.stats.GoroutinesSpawned++ }
func (s *Random) Stats() ScheduleStats  { return s.stats }

// --- PCT (Probabilistic Concurrency Testing) ---

// PCT implements the Probabilistic Concurrency Testing algorithm.
// It provides mathematical guarantees about the probability of finding bugs
// of a given depth (number of scheduling decisions needed to trigger the bug).
//
// For a bug that requires d scheduling decisions to trigger, PCT finds it
// with probability at least 1/n^(d-1) where n is the number of threads.
// Most real-world concurrency bugs have depth 1-2, so PCT is very effective.
//
// Reference: Burckhardt et al., "A Randomized Scheduler with Probabilistic
// Guarantees of Finding Bugs", ASPLOS 2010.
type PCT struct {
	rng          *rand.Rand
	priorities   map[int64]int // goroutine → priority
	changePoints []uint64      // instruction counts where priority inversion happens
	instructionN uint64
	bugDepth     int // target bug depth
	stats        ScheduleStats
}

func NewPCT(seed int64, bugDepth int) *PCT {
	rng := rand.New(rand.NewSource(seed))

	// Pre-generate d-1 priority change points
	changePoints := make([]uint64, bugDepth-1)
	for i := range changePoints {
		changePoints[i] = uint64(rng.Int63n(1_000_000)) // Within first 1M instructions
	}
	sort.Slice(changePoints, func(i, j int) bool { return changePoints[i] < changePoints[j] })

	return &PCT{
		rng:          rng,
		priorities:   make(map[int64]int),
		changePoints: changePoints,
		bugDepth:     bugDepth,
	}
}

func (s *PCT) Next(runnable []int64) int64 {
	if len(runnable) == 0 {
		return -1
	}

	s.stats.TotalDecisions++
	s.instructionN++

	// Assign initial priorities to new goroutines
	for _, gid := range runnable {
		if _, ok := s.priorities[gid]; !ok {
			s.priorities[gid] = s.rng.Int()
		}
	}

	// Check if we've hit a priority change point
	for _, cp := range s.changePoints {
		if s.instructionN == cp {
			// Randomly lower the priority of a random goroutine
			target := runnable[s.rng.Intn(len(runnable))]
			s.priorities[target] = -s.rng.Int()
			break
		}
	}

	// Pick the highest-priority runnable goroutine
	best := runnable[0]
	bestPri := s.priorities[best]
	for _, gid := range runnable[1:] {
		if s.priorities[gid] > bestPri {
			best = gid
			bestPri = s.priorities[gid]
		}
	}

	return best
}

func (s *PCT) OnSyncPoint(gid int64) { s.stats.SyncPoints++ }
func (s *PCT) OnSpawn(_, child int64) {
	s.stats.GoroutinesSpawned++
	s.priorities[child] = s.rng.Int()
}
func (s *PCT) Stats() ScheduleStats { return s.stats }
