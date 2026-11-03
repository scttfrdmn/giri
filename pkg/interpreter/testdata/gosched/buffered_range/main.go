// buffered_range: a producer fills a 2-slot buffer (blocking when it is full)
// and closes it; main ranges over it. Native Go: main receives 5 values, so
// trap indexes out of range.
package main

func trap(n int) {
	xs := make([]int, 5)
	_ = xs[n]
}

func main() {
	ch := make(chan int, 2)
	go func() {
		for i := 0; i < 5; i++ {
			ch <- i
		}
		close(ch)
	}()
	n := 0
	for range ch {
		n++
	}
	trap(n)
}
