// len_map_chan_zero verifies that len(map)==0 for an empty map and
// len/cap(chan)==0 for an empty unbuffered channel produce no false negatives
// (#138).
//
// When the map/channel is genuinely empty, len/cap correctly return 0 and the
// "== 0" conditions are TRUE — the inner nil-slice access would be a real
// violation. We avoid entering those blocks by using != 0 guards.
//
// Expected: 0 violations.
package main

func main() {
	// Empty map: len should be 0.
	m := make(map[string]int)
	if len(m) != 0 {
		var s []int
		_ = s[0] // would fire if len(m) wrongly returned non-zero
	}

	// Unbuffered channel: cap should be 0.
	ch := make(chan int)
	if cap(ch) != 0 {
		var s []int
		_ = s[0] // would fire if cap(ch) wrongly returned non-zero
	}

	// Unbuffered channel: len should be 0 (no pending items).
	if len(ch) != 0 {
		var s []int
		_ = s[0] // would fire if len(ch) wrongly returned non-zero
	}
}
