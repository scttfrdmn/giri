// safe_delete verifies that delete(map, key) correctly removes a key (#63).
//
// After delete, a lookup of the deleted key should return the zero value.
// No concurrent access; this is a single-goroutine correctness test.
//
// Expected: 0 violations.
package main

func main() {
	m := map[string]int{"a": 1, "b": 2}
	delete(m, "a")
	v, ok := m["a"] // Should be (0, false) after deletion
	_ = v
	_ = ok
	_ = m["b"] // Still present
}
