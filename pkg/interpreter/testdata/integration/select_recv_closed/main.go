// select_recv_closed verifies that a select receive case on a closed empty
// channel returns ok=false (#145).
//
// False-positive canary: if recvOk is wrong (true instead of false), the
// ok branch fires the nil-slice OOB.
//
// Expected: 0 violations.
package main

func main() {
	ch := make(chan int, 4)
	close(ch) // closed immediately, no items

	// Select on a closed empty channel → ok must be false.
	select {
	case _, ok := <-ch:
		if ok {
			var s []int
			_ = s[0] // false positive: ok should be false (channel closed+empty)
		}
	}
}
