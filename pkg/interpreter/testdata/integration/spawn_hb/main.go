package main

// spawn_hb verifies that the go statement establishes happens-before (#29).
// Per the Go memory model: the go statement is synchronized before the
// start of the spawned goroutine. So main's write to *x happens-before
// the goroutine's read — no race even without explicit channel sync.
// Expected: 0 violations.
func main() {
	x := new(int)
	*x = 1 // write from main (parent) before spawn
	go func(p *int) {
		_ = *p // read in spawned goroutine — safe via spawn HB
	}(x)
}
