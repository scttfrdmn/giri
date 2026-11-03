// defer_block: a deferred call waits for a goroutine. Native Go: the deferred
// call completes, so trap indexes out of range.
package main

func trap(n int) {
	xs := []int{0}
	_ = xs[n]
}

func work(done chan struct{}) {
	defer func() {
		<-done
		trap(1)
	}()
	go func() { close(done) }()
}

func main() {
	work(make(chan struct{}))
}
