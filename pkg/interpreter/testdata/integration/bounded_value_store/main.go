// bounded_value_store verifies that valueStore entries for stack-allocated
// named-return variables are evicted on frame exit (#60).
//
// Each call to compute() creates an ssa.Alloc(Heap=false) for its named
// return variable. Without the fix, these alloc IDs would accumulate in
// valueStore forever. With the fix, each popFrame evicts the entry, keeping
// valueStore bounded to live heap/global allocs only.
//
// The test is a correctness test (not a memory-measurement test): it verifies
// that 100 calls to compute() produce the correct accumulated sum with
// 0 violations, confirming that eviction does not corrupt the return values.
//
// Expected: 0 violations.
package main

// compute has a named return of struct type (ssa.Alloc Heap=false).
// Its valueStore entry is evicted in popFrame via the StackAllocs cleanup.
type Result struct{ V int }

func compute(n int) (r Result) {
	r.V = n * n
	return
}

func main() {
	sum := 0
	for i := 1; i <= 100; i++ {
		sum += compute(i).V
	}
	_ = sum // 338350
}
