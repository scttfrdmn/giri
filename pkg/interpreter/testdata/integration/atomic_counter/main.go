// atomic_counter verifies that sync/atomic Load/Store/Add intercepts work (#77).
//
// Expected: 0 violations.
package main

import "sync/atomic"

func main() {
	var counter int64

	// StoreInt64
	atomic.StoreInt64(&counter, 10)

	// LoadInt64
	n := atomic.LoadInt64(&counter)
	_ = n // 10

	// AddInt64 returns new value
	n2 := atomic.AddInt64(&counter, 5)
	_ = n2 // 15

	// LoadInt64 again
	n3 := atomic.LoadInt64(&counter)
	_ = n3 // 15

	// int32 variants
	var c32 int32
	atomic.StoreInt32(&c32, 1)
	old := atomic.AddInt32(&c32, 2)
	_ = old // 3
	v := atomic.LoadInt32(&c32)
	_ = v // 3
}
