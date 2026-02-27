// wg_negative verifies that a negative WaitGroup counter is detected (#57).
//
// Done() is called without a prior Add(), driving the counter from 0 to -1.
// At runtime Go panics: "sync: negative WaitGroup counter".
// go vet: pass, go test -race: pass (single goroutine, no concurrent access).
// Giri: intercepts the Done() call and checks the counter.
//
// Expected: 1 violation, "waitgroup".
package main

import "sync"

func main() {
	var wg sync.WaitGroup
	wg.Done() // no prior Add — counter goes from 0 to -1
}
