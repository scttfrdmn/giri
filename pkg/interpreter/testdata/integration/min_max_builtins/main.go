// min_max_builtins verifies that the Go 1.21+ min() and max() builtins
// are handled correctly by the interpreter (#69).
//
// Expected: 0 violations.
package main

func main() {
	// Integer min/max
	a, b, c := 10, 3, 7
	lo := min(a, b, c)
	hi := max(a, b, c)
	_ = lo // 3
	_ = hi // 10

	// Two-argument forms
	x := min(5, 9)
	y := max(5, 9)
	_ = x // 5
	_ = y // 9

	// Float min/max
	f1, f2 := 1.5, 2.5
	flo := min(f1, f2)
	fhi := max(f1, f2)
	_ = flo // 1.5
	_ = fhi // 2.5

	// String min/max
	s1, s2 := "apple", "banana"
	slo := min(s1, s2)
	shi := max(s1, s2)
	_ = slo // "apple"
	_ = shi // "banana"
}
