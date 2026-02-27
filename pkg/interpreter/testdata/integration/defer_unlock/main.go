// defer_unlock verifies that a deferred sync.Mutex.Unlock() call is
// properly executed when the frame pops (#47).
//
// Without the fix, executeDeferred silently dropped the Unlock call, so the
// mutex was never released. With the fix, the sync package dispatch fires and
// the vector-clock state is updated correctly.
//
// Expected: 0 violations.
package main

import "sync"

func withLock(mu *sync.Mutex) {
	mu.Lock()
	defer mu.Unlock()
}

func main() {
	var mu sync.Mutex
	withLock(&mu)
}
