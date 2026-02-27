// This program compiles cleanly, passes go vet, and passes go test -race.
// Giri detects the extra Done() call that drives the WaitGroup counter negative.
package main

import "sync"

// process does work and signals completion to the WaitGroup.
// Each call to process should be matched by exactly one Add(1).
func process(wg *sync.WaitGroup, id int) {
	defer wg.Done()
	_ = id * 2
}

func main() {
	var wg sync.WaitGroup

	wg.Add(1)
	go process(&wg, 42) // goroutine will call Done() once

	// BUG: caller also calls Done() — there are now two Done() calls for one Add(1).
	// When the goroutine finishes, the counter goes: 1 → 0 (caller's Done) → -1
	// (goroutine's Done) → panic: "sync: negative WaitGroup counter".
	wg.Done()

	wg.Wait()
}
