// select_timeout verifies that a blocking select with a time.After arm does
// not deadlock. The time.After channel is pre-populated by the interpreter so
// the timeout arm fires immediately without blocking.
//
// Without the time.After intercept: the select blocks forever, goroutine leak.
// With the intercept: timeout arm fires, no violation.
//
// Expected: 0 violations.
package main

import "time"

func waitOrTimeout(ch <-chan int) {
	select {
	case <-ch:
	case <-time.After(1 * time.Second):
	}
}

func main() {
	ch := make(chan int)
	waitOrTimeout(ch) // should take the time.After arm
}
