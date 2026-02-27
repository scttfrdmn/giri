// map_race verifies that concurrent unsynchronized writes to the same map
// from two sibling goroutines are detected as a data race.
//
// Two goroutines are spawned by main with no synchronization between them.
// Both write to the same map key — classic map data race.
//
// This mirrors the data_race test pattern (sibling goroutines) to ensure the
// race is visible regardless of scheduling order.
//
// Expected: 1 violation, category "data race".
package main

func main() {
	m := make(map[string]int)
	go func(m map[string]int) { m["key"] = 1 }(m) // goroutine A writes
	go func(m map[string]int) { m["key"] = 2 }(m) // goroutine B writes — races with A
}
