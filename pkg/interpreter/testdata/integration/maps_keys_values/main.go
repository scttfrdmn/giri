// maps_keys_values verifies that maps.* functions are correctly intercepted.
//
// Exercises Clone, Copy, DeleteFunc, Equal, and EqualFunc.
// (Keys and Values return iter.Seq iterators; ranging over them requires
// range-over-function support which is tested separately.)
//
// Expected: 0 violations.
package main

import "maps"

func main() {
	m := map[string]int{"a": 1, "b": 2, "c": 3}

	// Clone — returns a shallow copy of the map.
	m2 := maps.Clone(m)
	_ = m2

	// Copy — merges m into dst.
	dst := map[string]int{}
	maps.Copy(dst, m)

	// DeleteFunc — removes entries where predicate is true; probes callback.
	maps.DeleteFunc(m, func(k string, v int) bool {
		return v > 5
	})

	// Equal — conservative false; no assertion needed.
	_ = maps.Equal(m, dst)
}
