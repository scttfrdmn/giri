// race_dormant_happens_before: main writes x before spawning a reader and
// again after wg.Wait. Both writes are ordered with the read, so there is no
// race (native go run -race agrees) whether or not the detector recorded the
// pre-spawn write (#272).
// Expected: 0 violations.
package main

import "sync"

var x int

func reader(wg *sync.WaitGroup) { _ = x; wg.Done() }

func main() {
	x = 1 // before any goroutine exists: happens before the reader
	var wg sync.WaitGroup
	wg.Add(1)
	go reader(&wg)
	wg.Wait()
	x = 2 // after Wait: happens after the reader
}
