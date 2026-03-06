// atomic_generics exercises the Go 1.19+ concrete atomic types added in v0.69.0:
// atomic.Int32, Int64, Uint32, Uint64, Uintptr, Bool — Load, Store, Add, Swap,
// CompareAndSwap methods; also bufio.ReadBytes.
// Expected: 0 violations.
package main

import (
	"bufio"
	"strings"
	"sync/atomic"
)

func main() {
	// atomic.Int64 — Load/Store/Add/Swap/CompareAndSwap.
	var counter atomic.Int64
	counter.Store(10)
	val := counter.Load()
	_ = val
	newVal := counter.Add(5)
	_ = newVal
	old := counter.Swap(100)
	_ = old
	ok := counter.CompareAndSwap(100, 200)
	_ = ok

	// atomic.Int32
	var i32 atomic.Int32
	i32.Store(1)
	_ = i32.Load()
	_ = i32.Add(1)

	// atomic.Uint64
	var u64 atomic.Uint64
	u64.Store(42)
	_ = u64.Load()
	_ = u64.Add(1)

	// atomic.Uint32
	var u32 atomic.Uint32
	u32.Store(7)
	_ = u32.Load()

	// atomic.Uintptr
	var up atomic.Uintptr
	up.Store(0)
	_ = up.Load()

	// atomic.Bool — no Add method.
	var flag atomic.Bool
	flag.Store(true)
	_ = flag.Load()
	_ = flag.Swap(false)
	_ = flag.CompareAndSwap(false, true)

	// bufio.ReadBytes (v0.69.0).
	r := bufio.NewReader(strings.NewReader("hello\nworld\n"))
	line, _ := r.ReadBytes('\n')
	_ = line
}
