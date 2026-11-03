// pingpong: 1000 round trips between main and a goroutine.
// Native Go: n is 1000 at the end, so trap indexes out of range.
package main

func trap(n int) {
	xs := make([]int, 1000)
	_ = xs[n]
}

func main() {
	ping, pong := make(chan int), make(chan int)
	n := 0
	go func() {
		for range ping {
			n++
			pong <- 0
		}
	}()
	for i := 0; i < 1000; i++ {
		ping <- i
		<-pong
	}
	close(ping)
	trap(n)
}
