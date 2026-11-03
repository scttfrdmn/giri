// buffered_semaphore: a 1-slot buffered channel used as a lock. The k-th
// receive happens before the (k+1)-th send completes, so the increments are
// ordered. Native Go (-race): no race; n is 2, so trap indexes out of range.
package main

import "sync"

func trap(n int) {
	xs := []int{0, 0}
	_ = xs[n]
}

func main() {
	sem := make(chan struct{}, 1)
	var wg sync.WaitGroup
	n := 0
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			n++
			<-sem
		}()
	}
	wg.Wait()
	trap(n)
}
