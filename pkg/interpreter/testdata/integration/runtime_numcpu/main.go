// runtime_numcpu verifies that runtime package intercepts work (#88).
//
// Expected: 0 violations.
package main

import "runtime"

func main() {
	// NumCPU reports the number of available CPUs.
	n := runtime.NumCPU()
	_ = n // >= 1

	// GOMAXPROCS queries and optionally sets the parallelism.
	prev := runtime.GOMAXPROCS(0) // query without changing
	_ = prev

	// NumGoroutine reports the current goroutine count.
	ng := runtime.NumGoroutine()
	_ = ng // >= 1

	// Version returns the Go runtime version string.
	v := runtime.Version()
	_ = v // "go1.x.y"

	// GOOS and GOARCH are string constants.
	_ = runtime.GOOS
	_ = runtime.GOARCH

	// GOROOT returns the installation root.
	_ = runtime.GOROOT()

	// GC triggers a garbage collection cycle (noop in the interpreter).
	runtime.GC()

	// Gosched yields the CPU (noop in the interpreter).
	runtime.Gosched()

	// Caller returns program counter info (conservative).
	_, _, _, ok := runtime.Caller(0)
	_ = ok // false (conservative)

	// Stack writes a stack trace into a buffer.
	buf := make([]byte, 1024)
	n2 := runtime.Stack(buf, false)
	_ = n2 // 0
}
