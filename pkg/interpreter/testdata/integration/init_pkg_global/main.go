// init_pkg_global verifies that package-level variables with function-call
// initializers are properly set before main() runs.
//
// var ch = make(chan int, 1) is compiled into the package's synthesized init()
// function. Without init() being called before main(), ch would be nil and
// ch<-42 would block forever (goroutine leak).
//
// Expected: 0 violations.
package main

var ch = make(chan int, 1)

func main() {
	ch <- 42
	v := <-ch
	if v != 42 {
		// Should never reach here if init() correctly initialized ch.
		var x []int
		_ = x[0] // canary: index OOB only reached on incorrect dispatch
	}
}
