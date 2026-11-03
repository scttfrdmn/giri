// race_dormant_array_prespawn: main fills an array before any goroutine
// exists, which the dormant race detector doesn't record (#272), then a child
// reads it with no further synchronization before the read. The writes happen
// before the child starts, so there is no race (native go run -race agrees).
// From the tumble session's review of #284.
// Expected: 0 violations.
package main

var x, y [16]int

func main() {
	for i := range x {
		x[i] = i
	}
	done := make(chan bool)
	go func() { y[0] = x[3] + x[15]; done <- true }()
	<-done
	println(y[0])
}
