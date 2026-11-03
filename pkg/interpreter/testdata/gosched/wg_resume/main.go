// wg_resume: main waits for three workers, then checks their count.
// Native Go: the count is 3, so trap indexes out of range. Before #294 main
// stopped at wg.Wait and never reached the check.
package main

import "sync"

func trap(n int) {
	xs := []int{0, 0, 0}
	_ = xs[n]
}

func main() {
	var wg sync.WaitGroup
	var mu sync.Mutex
	n := 0
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			mu.Lock()
			n++
			mu.Unlock()
		}()
	}
	wg.Wait()
	trap(n)
}
