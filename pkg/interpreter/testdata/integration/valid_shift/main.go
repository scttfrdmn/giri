// valid_shift verifies that valid shift operations produce no violations.
// Expected: 0 violations.
package main

func main() {
	x := 1
	_ = x << 3
	_ = x >> 2
	_ = x << 0
}
