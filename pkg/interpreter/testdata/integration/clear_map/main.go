// clear_map verifies that the Go 1.21+ clear() builtin is handled
// correctly for maps (#69).
//
// Expected: 0 violations.
package main

func main() {
	// Build a map with several entries.
	m := map[string]int{
		"a": 1,
		"b": 2,
		"c": 3,
	}

	// Clear it — the interpreter must process the builtin without panicking.
	clear(m)

	// After clear, iterating should see 0 entries.
	count := 0
	for range m {
		count++
	}
	_ = count

	// clear on an already-empty map: noop.
	empty := make(map[int]string)
	clear(empty)

	// clear on a slice: noop in the model (elements not individually tracked).
	s := []int{1, 2, 3, 4, 5}
	clear(s)
	_ = s
}
