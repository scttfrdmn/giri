// slice_3index verifies that 3-index slice expressions s[low:high:max] correctly
// set the capacity of the resulting slice (#85).
//
// Expected: 0 violations.
package main

func main() {
	s := make([]int, 10, 20)

	// 2-index slice: len=3, cap=18 (20-2)
	s2 := s[2:5]
	_ = len(s2) // 3
	_ = cap(s2) // 18

	// 3-index slice: len=3, cap=8 (10-2)
	s3 := s[2:5:10]
	_ = len(s3) // 3
	_ = cap(s3) // 8

	// Re-slice within cap: fine.
	s4 := s3[:cap(s3)]
	_ = s4

	// 3-index starting at 0: len=5, cap=5
	s5 := s[0:5:5]
	_ = len(s5) // 5
	_ = cap(s5) // 5

	// Full 3-index: low==high==max is valid (empty slice with zero capacity).
	s6 := s[3:3:3]
	_ = len(s6) // 0
	_ = cap(s6) // 0

	// Array slicing with 3-index.
	arr := [8]byte{1, 2, 3, 4, 5, 6, 7, 8}
	a := arr[1:4:6]
	_ = len(a) // 3
	_ = cap(a) // 5
}
