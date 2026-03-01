// slices_intercept verifies that slices.* functions are correctly intercepted.
//
// Exercises Contains, Index, Sort, SortFunc, Reverse, Clone, and Equal.
// If any intercept returns a Value{} (nil), downstream branches may take the
// wrong path and trigger false violations.
//
// Expected: 0 violations.
package main

import (
	"cmp"
	"slices"
)

func main() {
	s := []int{3, 1, 4, 1, 5, 9, 2, 6}

	// Contains: 5 is present.
	if !slices.Contains(s, 5) {
		var x []int
		_ = x[0] // canary: only reached if Contains is wrongly false
	}

	// Index: -1 means not found; 42 is not in s.
	idx := slices.Index(s, 42)
	_ = idx // -1 is valid; no assertion needed

	// Sort — in-place, noop in Giri.
	slices.Sort(s)

	// SortFunc — probe callback.
	slices.SortFunc(s, func(a, b int) int {
		return cmp.Compare(a, b)
	})

	// Reverse — noop.
	slices.Reverse(s)

	// Clone — returns the slice.
	c := slices.Clone(s)
	_ = c

	// Equal — returns false conservatively; no assertion.
	_ = slices.Equal(s, c)
}
