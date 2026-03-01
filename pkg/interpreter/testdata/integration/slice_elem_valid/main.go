// slice_elem_valid verifies that in-bounds slice element accesses produce no violations.
//
// Expected: 0 violations.
package main

func main() {
	s := make([]int, 5, 10)
	s[0] = 1
	s[4] = 5
	_ = s[0]
	_ = s[4]

	// Resliced: new len=3, cap=10; indices 0..2 are valid.
	t := s[:3]
	_ = t[2]
}
