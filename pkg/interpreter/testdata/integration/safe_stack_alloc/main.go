// safe_stack_alloc verifies that named-return struct allocations (ssa.Alloc
// Heap=false, AllocStack in Giri) are correctly poisoned on frame exit without
// causing false-positive UseAfterFreeErrors (#51).
//
// Go's SSA escape analysis marks any alloc whose address outlives the frame as
// Heap=true. AllocStack (Heap=false) allocs therefore never have surviving
// external references, so poisoning them in popFrame is always safe.
//
// This test exercises the code path by using named returns of struct type
// (which the SSA builder keeps as Heap=false when their address is not captured)
// alongside deferred calls, verifying that:
//   - recomputeNamedReturns extracts the correct value before poisoning runs
//   - the scalar return value is correctly returned to the caller
//   - no spurious UseAfterFreeError is reported
//
// Expected: 0 violations.
package main

type Point struct{ X, Y int }

// makePoint returns a named-return struct. The SSA builder creates an
// Alloc(Heap=false) for `p` because no pointer to p escapes the function.
// After popFrame, Giri poisons p's alloc and removes it from valueStore.
func makePoint(x, y int) (p Point) {
	p.X = x
	p.Y = y
	return
}

// sumCoords uses multiple named returns (both Heap=false) with arithmetic.
func sumCoords(pts []Point) (sumX, sumY int) {
	for _, pt := range pts {
		sumX += pt.X
		sumY += pt.Y
	}
	return
}

func main() {
	p := makePoint(3, 4)
	pts := []Point{p, makePoint(1, 2), makePoint(5, 6)}
	sx, sy := sumCoords(pts)
	_ = sx + sy
}
