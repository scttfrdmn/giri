// range_array_race verifies that range-over-array executes its body, enabling
// the race detector to fire on concurrent accesses inside the loop (#137).
//
// Two sibling goroutines (both spawned from main) range over separate [3]int
// value arrays, but both write to the same shared global counter without
// synchronisation.  Without the range-over-array fix the loop body never runs
// and the race is missed.  With the fix, both bodies execute and the
// write/write race on counter is detected.
//
// Uses sibling goroutines per the data_race test pattern: parent→child spawn
// establishes happens-before and would suppress the race.
//
// Expected: 1 violation, category "data race".
package main

var counter int

func writeArr() {
	arr := [3]int{1, 2, 3}
	for range arr {
		counter++ // unsynchronised write
	}
}

func main() {
	go writeArr() // goroutine A writes
	go writeArr() // goroutine B writes — races with A
}
