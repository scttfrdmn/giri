// negative_shift verifies that a negative shift count is detected.
// In Go 1.13+, x << n where n < 0 panics: "runtime error: negative shift count".
// Expected: 1 violation (negative-shift).
package main

func shiftBy(x int, n int) int {
	return x << n
}

func main() {
	_ = shiftBy(1, -1)
}
