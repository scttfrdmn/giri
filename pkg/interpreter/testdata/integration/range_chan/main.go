// range_chan verifies that range-over-channel iterates the correct number of
// times (#143).
//
// Without the fix, ssa.Next immediately returns ok=false for a channel
// iterator and the loop body never executes (count stays 0).
// False-positive canary: if count != 3, the "!= 3" comparison fires a
// nil-slice OOB.
//
// Expected: 0 violations.
package main

func main() {
	// Pre-populate a buffered channel and close it.
	ch := make(chan int, 3)
	ch <- 1
	ch <- 2
	ch <- 3
	close(ch)

	// Range over the channel: should iterate exactly 3 times.
	count := 0
	for range ch {
		count++
	}

	if count != 3 {
		var s []int
		_ = s[0] // false positive: only reached if range-over-channel is broken
	}
}
