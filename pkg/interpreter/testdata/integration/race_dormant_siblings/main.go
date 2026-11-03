// race_dormant_siblings checks that the race detector, dormant while only
// main exists (#272), wakes at the first go statement: main writes x before
// spawning, then two sibling goroutines read and write it unsynchronized.
// Native go run -race reports this race.
// Expected: 1 violation, data-race.
package main

import "sync"

var x int

func reader(wg *sync.WaitGroup) { _ = x; wg.Done() }
func writer(wg *sync.WaitGroup) { x = 3; wg.Done() }

func main() {
	x = 1 // before any goroutine exists
	var wg sync.WaitGroup
	wg.Add(2)
	go reader(&wg)
	go writer(&wg) // sibling: races with reader
	wg.Wait()
}
