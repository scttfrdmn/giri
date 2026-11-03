// errgroup_wait: errgroup.Group.Wait waits for its goroutines.
// Native Go: n is 2 after Wait, so trap indexes out of range.
package main

import (
	"sync"

	"golang.org/x/sync/errgroup"
)

func trap(n int) {
	xs := []int{0, 0}
	_ = xs[n]
}

func main() {
	var g errgroup.Group
	var mu sync.Mutex
	n := 0
	for i := 0; i < 2; i++ {
		g.Go(func() error {
			mu.Lock()
			n++
			mu.Unlock()
			return nil
		})
	}
	_ = g.Wait()
	trap(n)
}
