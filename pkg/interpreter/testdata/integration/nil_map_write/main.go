// nil_map_write verifies that writing to a nil map is detected (#54).
//
// In Go, writing to a nil map panics at runtime: "assignment to entry in nil map".
// go vet: pass, go test -race: pass.
// Giri: detects the map write with a nil backing as a violation.
//
// Expected: 1 violation, "nil map".
package main

func main() {
	var m map[string]int
	m["key"] = 1 // assignment to entry in nil map
}
