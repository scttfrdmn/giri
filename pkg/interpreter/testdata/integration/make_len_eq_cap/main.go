// make_len_eq_cap verifies that make([]T, len, cap) where len == cap or len < cap
// produces no violations.
//
// Expected: 0 violations.
package main

func main() {
	_ = make([]int, 5, 5)  // len == cap: valid
	_ = make([]int, 3, 10) // len < cap: valid
	_ = make([]int, 0, 8)  // len=0: valid
}
