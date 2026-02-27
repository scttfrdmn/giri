// multi_recover verifies that panic/recover works correctly across call frames:
// an inner function panics, a deferred recover() in the outer function catches
// it, and execution continues normally in main (#48).
//
// Without the fix, ssa.Panic cleared the call stack eagerly before deferred
// recover() could fire, leaving the goroutine in a permanently panicked state.
// With the fix, unwinding is lazy (per-frame) and recover() can intercept it.
//
// Expected: 0 violations.
package main

func inner() {
	panic("intentional")
}

func outer() {
	defer func() {
		recover()
	}()
	inner()
}

func main() {
	outer()
}
