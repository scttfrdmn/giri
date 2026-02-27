// sort_slice verifies that sort.Slice is intercepted and the comparator
// callback is probed without interpreter crashes (#68).
//
// Expected: 0 violations.
package main

import "sort"

func main() {
	nums := []int{5, 2, 8, 1, 9, 3}
	sort.Slice(nums, func(i, j int) bool {
		return nums[i] < nums[j]
	})
	_ = nums

	// SliceStable with the same pattern
	words := []string{"banana", "apple", "cherry"}
	sort.SliceStable(words, func(i, j int) bool {
		return words[i] < words[j]
	})
	_ = words
}
