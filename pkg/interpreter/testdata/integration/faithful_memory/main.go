// faithful_memory checks that CLI loads return the value last stored at every
// offset, not only offset 0 (#260, the CLI half of #248): slice and array
// elements, struct fields, whole-value array/struct copies, append, copy and
// []byte conversion, and that sort.Ints and friends see real elements.
//
// Each comparison that is true natively has wrong() in its else branch: one
// violation per value giri gets wrong.
// Expected: 0 violations.
package main

import "sort"

type rec struct {
	a, b int
	c    [3]int
}

func wrong() {
	var t []int
	_ = t[0]
}

func sum(xs []int) int {
	s := 0
	for _, x := range xs {
		s += x
	}
	return s
}

func main() {
	is := []int{1, 3, 7}
	if is[2] == 7 && sum(is) == 11 {
	} else {
		wrong()
	}

	var a [4]int
	a[3] = 9
	b := a // whole-array copy
	b[0] = 5
	if b[3] == 9 && a[0] == 0 && b[0] == 5 {
	} else {
		wrong()
	}

	r := rec{a: 1, b: 2}
	r.c[1] = 4
	q := r // whole-struct copy
	if q.b == 2 && q.c[1] == 4 {
	} else {
		wrong()
	}

	ys := append([]int{}, is...)
	ys = append(ys, 10)
	zs := make([]int, 2)
	n := copy(zs, ys[2:])
	if ys[3] == 10 && n == 2 && zs[1] == 10 {
	} else {
		wrong()
	}

	bs := []byte("hey")
	if bs[1] == 'e' {
	} else {
		wrong()
	}

	xs := []int{5, 1, 4}
	sort.Ints(xs)
	if xs[0] == 1 && xs[2] == 5 && sort.SearchInts(xs, 4) == 1 && sort.IntsAreSorted(xs) {
	} else {
		wrong()
	}
}
