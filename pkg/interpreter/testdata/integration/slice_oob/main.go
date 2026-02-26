package main

// slice_oob exercises slice re-slice bounds validation (#32).
// s[0:100] where cap(s) = 4 exceeds the slice capacity.
// Expected: >= 1 "out of bounds" violation.
//
// Note: _ = s[0:100] would be DCE'd by Go SSA (Slice has no explicit side
// effects in SSA form). Routing through sink() forces the Slice instruction
// to be preserved because function arguments must be evaluated.
func sink(s []int) {}

func main() {
	s := make([]int, 4)
	sink(s[0:100]) // high=100 > cap=4: out-of-bounds reslice
}
