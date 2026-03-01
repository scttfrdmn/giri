// array_index_valid verifies that in-bounds pointer-to-array indexing produces
// no violations.
//
// Expected: 0 violations.
package main

func main() {
	var arr [3]int
	p := &arr
	p[0] = 10
	p[1] = 20
	p[2] = 30
	_ = p[0]
	_ = p[2]
}
