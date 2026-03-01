// len_map_chan verifies that len(map), len(chan), and cap(chan) return correct
// values (#138).
//
// False-positive canary pattern: each block below is guarded by a condition
// that should evaluate to FALSE for a non-empty map/channel. If the builtin
// returns Value{} instead of the correct count, evalBinOp cannot produce a
// bool and ssa.If takes the default (true) branch — entering the block and
// triggering a nil-slice OOB that should never fire.
//
// Expected: 0 violations.
package main

func main() {
	// len(map): non-empty map → len should be 2, not 0.
	m := map[string]int{"a": 1, "b": 2}
	if len(m) == 0 {
		var s []int
		_ = s[0] // false positive: only reached if len(m) wrongly returns 0
	}

	// cap(chan): buffered channel → cap should be 4, not 0.
	ch := make(chan int, 4)
	if cap(ch) == 0 {
		var s []int
		_ = s[0] // false positive: only reached if cap(ch) wrongly returns 0
	}

	// len(chan): send 2 items → len should be 2, not 0.
	ch <- 1
	ch <- 2
	if len(ch) == 0 {
		var s []int
		_ = s[0] // false positive: only reached if len(ch) wrongly returns 0
	}
}
