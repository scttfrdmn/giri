// select_default verifies that a non-blocking select with a default clause
// does not produce violations when no channel is ready.
//
// Expected: 0 violations.
package main

func tryRecv(ch <-chan int) int {
	select {
	case v := <-ch:
		return v
	default:
		return -1
	}
}

func main() {
	ch := make(chan int, 1)
	tryRecv(ch) // no pending value, should take default
}
