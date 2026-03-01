// slice_index_valid verifies that valid slice indexing produces no violations.
// Expected: 0 violations.
package main

func main() {
	s := []int{10, 20, 30}
	for i := 0; i < len(s); i++ {
		_ = s[i]
	}
}
