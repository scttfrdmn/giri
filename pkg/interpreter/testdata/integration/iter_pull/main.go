// iter_pull verifies that iter.Pull and iter.Pull2 are correctly intercepted.
//
// iter.Pull converts an iter.Seq into a pull-based (next, stop) pair.
// iter.Pull2 converts an iter.Seq2 into a pull-based (next, stop) pair.
//
// The intercept returns opaque non-nil values for both; calling them is safe.
//
// Expected: 0 violations.
package main

import (
	"iter"
	"slices"
)

func main() {
	// Build a simple Seq from slices.Values.
	s := []int{1, 2, 3}
	seq := slices.Values(s)

	// iter.Pull returns (next func() (int, bool), stop func()).
	next, stop := iter.Pull(seq)
	defer stop()

	// Call next() once to consume one element.
	_, _ = next()

	// iter.Pull2 with a two-value sequence.
	seq2 := slices.All(s)
	next2, stop2 := iter.Pull2(seq2)
	defer stop2()

	_, _, _ = next2()
}
