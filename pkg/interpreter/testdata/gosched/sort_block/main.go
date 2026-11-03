// sort_block: a sort comparator receives from a channel that a goroutine
// fills, one value at a time. Native Go: the sort completes, so trap indexes
// out of range.
package main

import "sort"

func trap(n int) {
	xs := []int{0}
	_ = xs[n]
}

func main() {
	tokens := make(chan int, 1)
	stop := make(chan struct{})
	go func() {
		for {
			select {
			case tokens <- 1:
			case <-stop:
				return
			}
		}
	}()
	xs := []int{3, 1, 2}
	sort.Slice(xs, func(i, j int) bool {
		<-tokens
		return xs[i] < xs[j]
	})
	close(stop)
	trap(1)
}
