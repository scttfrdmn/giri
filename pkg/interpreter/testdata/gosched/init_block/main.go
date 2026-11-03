// init_block: init waits for a goroutine it starts. Native Go: init
// completes and main runs, so trap indexes out of range.
package main

var v int

func init() {
	done := make(chan struct{})
	go func() {
		v = 1
		close(done)
	}()
	<-done
}

func trap(n int) {
	xs := []int{0}
	_ = xs[n]
}

func main() {
	trap(v)
}
