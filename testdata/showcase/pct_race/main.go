// This program compiles cleanly, passes go vet, and passes go test -race.
//
// With Giri's default round-robin scheduler, no violation is reported:
// setup() (GID 3, higher ID) always runs before work() (GID 2, lower ID),
// so Add(1) happens before Done() and the counter stays non-negative.
//
// With Giri's PCT multi-run scheduler (RunN), the bug is discovered: PCT
// randomizes priorities, sometimes running work() before setup(). In that
// ordering Done() is called before Add(1), driving the counter to -1 and
// triggering a WaitGroup negative counter violation.
package main

import "sync"

// wg is a WaitGroup used to coordinate setup and work goroutines.
// The contract: setup() must call Add(1) before work() calls Done().
// There is no synchronization to enforce this ordering.
var wg sync.WaitGroup

// setup performs initialization and signals completion via Add.
// It must run before work() for the WaitGroup counter to remain non-negative.
func setup() {
	wg.Add(1) // counter: 0 → 1
}

// work performs the actual task and decrements the WaitGroup counter.
// BUG: if work() runs before setup(), it calls Done() on a counter of 0,
// causing the counter to go negative: "sync: negative WaitGroup counter".
func work() {
	wg.Done() // should decrement from 1 to 0, but decrements from 0 to -1 if setup hasn't run
}

func main() {
	go work()  // GID 2 — runs SECOND with round-robin (lower GID)
	go setup() // GID 3 — runs FIRST with round-robin (higher GID)

	// With round-robin:
	//   setup() (GID 3) runs first → Add(1) → counter=1
	//   work()  (GID 2) runs next  → Done() → counter=0 → OK
	//   Giri finds: no violations.
	//
	// With PCT (some runs):
	//   work() (GID 2) runs first → Done() → counter=-1 → WaitGroupNegativeError
	//   Giri + RunN finds: waitgroup negative counter violation.

	wg.Wait()
}
