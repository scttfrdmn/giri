// This program compiles cleanly, passes go vet, and often passes go test -race
// under the default round-robin scheduler. Giri detects the data race reliably.
package main

// counter is a shared map accessed concurrently without synchronization.
// In Go, concurrent map writes are undefined behavior: they may silently
// corrupt the map's internal structure, trigger a panic, or appear to work
// depending on CPU scheduling.
//
// go vet cannot detect this: map operations are type-correct.
// go test -race may miss it under round-robin scheduling (goroutines often
// run sequentially rather than truly in parallel in the test environment).
// Giri detects it by tracking vector-clock happens-before relationships:
// the two writes to m["count"] have no synchronization between them.
var counter = make(map[string]int)

func increment(m map[string]int) {
	m["count"]++ // writes to the map
}

func main() {
	// Two sibling goroutines both write to the same map key.
	// Neither holds a lock. Neither synchronizes with the other.
	// BUG: this is a data race — concurrent unsynchronized map writes.
	go increment(counter) // goroutine A
	go increment(counter) // goroutine B — races with A
}
