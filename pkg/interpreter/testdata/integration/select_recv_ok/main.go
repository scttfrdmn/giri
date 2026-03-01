// select_recv_ok verifies that a select receive case on a closed buffered
// channel with pending items returns ok=true (#145).
//
// False-positive canary: if recvOk is wrong (false instead of true), the
// !ok branch fires the nil-slice OOB.
//
// Expected: 0 violations.
package main

func main() {
	ch := make(chan int, 2)
	ch <- 1
	ch <- 2
	close(ch)

	// First select: channel has 2 pending items and is closed → ok must be true.
	select {
	case _, ok := <-ch:
		if !ok {
			var s []int
			_ = s[0] // false positive: ok should be true (item was received)
		}
	}

	// Second select: one item consumed; one still pending → ok still true.
	select {
	case _, ok := <-ch:
		if !ok {
			var s []int
			_ = s[0]
		}
	}
}
