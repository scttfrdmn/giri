// log_fatal verifies that log.Fatal terminates all goroutines cleanly (#80).
//
// log.Fatal simulates os.Exit(1): the interpreter marks all goroutines as
// Panicked and stops execution. No violations should be reported.
//
// Expected: 0 violations.
package main

import "log"

func mayFail(ok bool) {
	if !ok {
		log.Fatal("fatal error")
	}
}

func main() {
	// Call with ok=true; log.Fatal is not reached.
	mayFail(true)

	// Call with ok=false; log.Fatal fires → all goroutines marked Panicked.
	mayFail(false)
}
