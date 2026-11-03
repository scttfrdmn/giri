// select_pair: a sender and a receiver both use select on the same
// unbuffered channel, each with a second case on its own channel that never
// fires. Native Go: the value is handed over, so trap indexes out of range.
package main

func trap(n int) {
	xs := make([]int, 42)
	_ = xs[n]
}

func main() {
	ch, never1, never2 := make(chan int), make(chan int), make(chan int)
	go func() {
		select {
		case ch <- 42:
		case <-never1:
		}
	}()
	select {
	case v := <-ch:
		trap(v)
	case never2 <- 0:
	}
}
