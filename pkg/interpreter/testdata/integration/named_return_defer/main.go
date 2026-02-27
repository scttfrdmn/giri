// named_return_defer verifies that a deferred closure modifying a named return
// variable is reflected in the actual return value (#49).
//
// Without the fix, the deferred closure wrote to the named-return alloc but
// the return value captured at ssa.Return time was already stale. With the fix,
// recomputeNamedReturns re-evaluates the return from valueStore after defers.
//
// Expected: 0 violations.
package main

func double(n int) (result int) {
	result = n
	defer func() {
		result *= 2
	}()
	return
}

func main() {
	_ = double(5) // returns 10 with correct named-return defer semantics
}
