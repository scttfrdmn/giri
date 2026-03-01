// range_chan_valid verifies that ranging over an empty closed channel
// produces 0 iterations without false-positive violations (#143).
//
// Expected: 0 violations.
package main

func main() {
	// Empty buffered channel, immediately closed.
	ch := make(chan int, 4)
	close(ch)

	count := 0
	for range ch {
		count++
	}

	// count must be 0; if range mistakenly iterates, it would enter the loop
	// body and increment count. The anti-canary checks nothing is wrong.
	if count != 0 {
		var s []int
		_ = s[0]
	}
}
