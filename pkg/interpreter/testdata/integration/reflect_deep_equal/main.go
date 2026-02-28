// reflect_deep_equal verifies that reflect.DeepEqual intercept works (#86).
//
// Expected: 0 violations.
package main

import "reflect"

func main() {
	// Identical primitive slices.
	a := []int{1, 2, 3}
	b := []int{1, 2, 3}
	eq := reflect.DeepEqual(a, b)
	_ = eq // true

	// Different slices.
	c := []int{1, 2, 4}
	neq := reflect.DeepEqual(a, c)
	_ = neq // false

	// Identical maps.
	m1 := map[string]int{"x": 1}
	m2 := map[string]int{"x": 1}
	meq := reflect.DeepEqual(m1, m2)
	_ = meq // true

	// Nil comparisons.
	var s1 []int
	var s2 []int
	nilEq := reflect.DeepEqual(s1, s2)
	_ = nilEq // true

	// Struct comparison.
	type P struct{ X, Y int }
	p1 := P{1, 2}
	p2 := P{1, 2}
	peq := reflect.DeepEqual(p1, p2)
	_ = peq // true
}
