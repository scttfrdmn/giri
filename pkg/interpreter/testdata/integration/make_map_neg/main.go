// make_map_neg verifies that make(map[K]V, n) where n < 0 is reported as a
// "make-invalid" violation.
//
// At runtime Go panics: "makemap: size out of range".
// The compiler rejects constant negative hints, so we compute the value via a
// helper function to bypass compile-time checks.
// Expected: 1 violation, category "make-invalid".
package main

func negCap() int { return -1 }

func main() {
	_ = make(map[string]int, negCap()) // cap=-1: runtime panic
}
