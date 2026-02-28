// sync_pool verifies that sync.Pool Get/Put intercepts work (#92).
//
// sync.Pool.Get() returns nil in the interpreter (pool always empty), so
// callers should fall through to the allocation branch. Put() is a noop.
//
// Expected: 0 violations.
package main

import "sync"

func main() {
	pool := sync.Pool{
		New: func() interface{} {
			return make([]byte, 64)
		},
	}

	// Get returns nil (pool empty) → New is called to allocate.
	v := pool.Get()
	var buf []byte
	if v == nil {
		buf = make([]byte, 64)
	} else {
		buf = v.([]byte)
	}
	_ = buf

	// Put returns the buffer to the pool.
	pool.Put(buf)

	// Second Get follows the same nil path.
	v2 := pool.Get()
	_ = v2

	// TryLock / TryRLock on RWMutex.
	var mu sync.RWMutex
	if mu.TryLock() {
		mu.Unlock()
	}
	if mu.TryRLock() {
		mu.RUnlock()
	}
}
