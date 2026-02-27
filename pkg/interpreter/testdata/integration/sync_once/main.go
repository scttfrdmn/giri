// sync_once verifies that sync.Once.Do(f) calls f exactly once (#61).
//
// Three calls to once.Do are made. Only the first should execute increment().
// If the interpreter fires f more than once, count would be 3 and a
// subsequent use might differ from expected. No unsafe operations, so the
// test is purely a correctness/no-crash regression guard.
//
// Expected: 0 violations.
package main

import "sync"

var count int

func increment() { count++ }

func main() {
	var once sync.Once
	once.Do(increment) // fires: count = 1
	once.Do(increment) // noop
	once.Do(increment) // noop
	_ = count
}
