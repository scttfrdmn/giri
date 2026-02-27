// atomic_cas verifies that sync/atomic CompareAndSwap and Swap intercepts work (#77).
//
// Expected: 0 violations.
package main

import "sync/atomic"

func main() {
	var val int64
	atomic.StoreInt64(&val, 42)

	// CompareAndSwap: old matches → swaps, returns true.
	ok := atomic.CompareAndSwapInt64(&val, 42, 100)
	_ = ok // true

	// CompareAndSwap: old does not match → no-op, returns false.
	ok2 := atomic.CompareAndSwapInt64(&val, 42, 200)
	_ = ok2 // false

	// Swap returns the old value.
	prev := atomic.SwapInt64(&val, 999)
	_ = prev // 100

	// uint64 variants
	var u uint64
	atomic.StoreUint64(&u, 7)
	old := atomic.SwapUint64(&u, 8)
	_ = old // 7
	v := atomic.LoadUint64(&u)
	_ = v // 8
}
