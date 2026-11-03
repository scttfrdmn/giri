// race_dormant_prespawn: main reads and writes x before any goroutine
// exists, which the dormant race detector doesn't record (#272), then two
// sibling goroutines both write x. Native go run -race reports the race.
// Expected: 1 violation, data-race.
package main

import "sync"

var x int

func writer(wg *sync.WaitGroup) { x = 3; wg.Done() }

func main() {
	x = 1 // pre-spawn write by main
	_ = x
	var wg sync.WaitGroup
	wg.Add(2)
	go writer(&wg)
	go writer(&wg) // two siblings write x: a race
	wg.Wait()
}
