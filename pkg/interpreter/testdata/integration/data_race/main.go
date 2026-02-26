package main

// data_race exercises the RaceDetector's vector-clock race detection (#23).
// Main writes to *x; then a goroutine also writes to *x with no synchronization.
// The two goroutines have independent vector clocks (no channel/mutex sync),
// so neither clock causally precedes the other → data race reported.
func main() {
	x := new(int)
	*x = 1 // write from main goroutine (GID 1)
	go func(p *int) {
		*p = 2 // write from goroutine 2 — races with main's write
	}(x)
}
