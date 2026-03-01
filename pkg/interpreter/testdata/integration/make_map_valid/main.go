// make_map_valid verifies that make(map[K]V) and make(map[K]V, n) where n >= 0
// produce no violations.
//
// Expected: 0 violations.
package main

func main() {
	_ = make(map[string]int)     // no hint: valid
	_ = make(map[string]int, 0)  // zero hint: valid
	_ = make(map[string]int, 10) // positive hint: valid
}
