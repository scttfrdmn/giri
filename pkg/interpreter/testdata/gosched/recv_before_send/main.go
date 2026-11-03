// recv_before_send: on an unbuffered channel the receive happens before the
// send completes, so main's read of x after its send is ordered after the
// goroutine's write. Native Go (-race): no race; x is 1, so trap indexes out
// of range.
package main

func trap(n int) {
	xs := []int{0}
	_ = xs[n]
}

func main() {
	x := 0
	c := make(chan int)
	go func() {
		x = 1
		<-c
	}()
	c <- 0
	trap(x)
}
