// sync_cond verifies that sync.Cond Wait/Signal/Broadcast intercepts work (#92).
//
// The test avoids closure-captured loop variables because the interpreter cannot
// propagate value changes through store/load cycles across goroutines.
//
// Expected: 0 violations.
package main

import "sync"

func main() {
	var mu sync.Mutex
	cond := sync.NewCond(&mu)

	// Signal: wake a waiter.
	mu.Lock()
	cond.Signal()
	mu.Unlock()

	// Wait: acquires and releases mu internally.
	mu.Lock()
	cond.Wait()
	mu.Unlock()

	// Broadcast wakes all waiters.
	var mu2 sync.Mutex
	cond2 := sync.NewCond(&mu2)
	cond2.Broadcast()

	// sync.Map.Range now probes the callback.
	var m sync.Map
	m.Store("key", "value")
	m.Range(func(k, v interface{}) bool {
		_ = k
		_ = v
		return true
	})
}
