// sort_strings verifies that sort.Strings, sort.Ints, and sort.Search
// are intercepted cleanly (#68).
//
// Expected: 0 violations.
package main

import "sort"

func main() {
	strs := []string{"banana", "apple", "cherry"}
	sort.Strings(strs)
	_ = strs

	nums := []int{5, 2, 8, 1, 9}
	sort.Ints(nums)
	_ = nums

	// sort.Search: find first index where nums[i] >= 5
	n := sort.Search(len(nums), func(i int) bool {
		return nums[i] >= 5
	})
	_ = n
}
