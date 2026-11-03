// chan_resume: main receives from done after a goroutine writes x.
// Native Go: x is 7 after the receive, so trap indexes out of range.
package main

func trap(n int) {
	xs := []int{0}
	_ = xs[n]
}

func main() {
	done := make(chan struct{})
	x := 0
	go func() {
		x = 7
		done <- struct{}{}
	}()
	<-done
	trap(x)
}
