// panic_block: while a panic unwinds, a deferred call waits for a goroutine
// and then recovers. Native Go: recovery succeeds and main continues, so trap
// indexes out of range.
package main

func trap(n int) {
	xs := []int{0}
	_ = xs[n]
}

func risky(done chan struct{}) (ok int) {
	defer func() {
		<-done
		if recover() != nil {
			ok = 1
		}
	}()
	go func() { close(done) }()
	panic("boom")
}

func main() {
	trap(risky(make(chan struct{})))
}
