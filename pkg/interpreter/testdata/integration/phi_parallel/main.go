// phi_parallel checks that a block's phi nodes take effect together. For
// "a, b = b, a" in a loop, each iteration's phi for a must read b's value from
// the previous iteration, and vice versa. Evaluating the phis one at a time
// let the second read the value the first had just assigned, so after one
// swap of (1, 2) giri saw (2, 2).
//
// Each comparison that is true natively has wrong() in its else branch: one
// violation per wrong result.
// Expected: 0 violations.
package main

func wrong() {
	var t []int
	_ = t[0]
}

func swap(n int) (int, int) {
	a, b := 1, 2
	for i := 0; i < n; i++ {
		a, b = b, a
	}
	return a, b
}

func rotate(n int) (int, int, int) {
	a, b, c := 1, 2, 3
	for i := 0; i < n; i++ {
		a, b, c = b, c, a
	}
	return a, b, c
}

func main() {
	if a, b := swap(1); a == 2 && b == 1 {
	} else {
		wrong()
	}
	if a, b, c := rotate(1); a == 2 && b == 3 && c == 1 {
	} else {
		wrong()
	}
}
