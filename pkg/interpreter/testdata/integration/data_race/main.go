package main

// data_race exercises the RaceDetector's vector-clock race detection (#23, #29).
// Two goroutines (sibling, not parent-child) both write to *x with no synchronization.
// They have independent, causally unrelated vector clocks → data race reported.
//
// Note: a parent writing *x then spawning a single goroutine that writes *x is NOT
// a data race per the Go memory model, because the go statement establishes
// happens-before. This test uses sibling goroutines to avoid that ordering.
func main() {
	x := new(int)
	go func(p *int) { *p = 1 }(x) // goroutine A writes
	go func(p *int) { *p = 2 }(x) // goroutine B writes — races with A
}
