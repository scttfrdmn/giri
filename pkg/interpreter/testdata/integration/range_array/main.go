// range_array verifies that range over a fixed-size array executes the correct
// number of iterations (regression for the silent-skip bug, #137).
//
// The loop accumulates an index sum; if no iterations run (old bug), sum stays 0
// and the subsequent divide-by-zero fires.  With the fix, sum > 0 and no
// violation occurs.
//
// Expected: 0 violations.
package main

func main() {
	arr := [5]int{10, 20, 30, 40, 50}
	sum := 0
	for i := range arr {
		sum += i + 1 // i ∈ {0,1,2,3,4} → sum = 15 after all iterations
	}
	// sum is 15; this does NOT divide by zero.
	// If range-over-array were broken (sum==0), the division below would fire.
	_ = 100 / sum
}
