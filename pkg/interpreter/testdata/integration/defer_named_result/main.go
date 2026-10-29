// defer_named_result checks that a deferred closure assigning a named result
// changes the returned value (#259). SSA emits an explicit rundefers before
// the result is loaded; without handling it, the deferred write was lost.
//
// wrong() indexes an empty slice: one violation per wrong result.
// Expected: 0 violations.
package main

func wrong() {
	var t []int
	_ = t[0]
}

func plusOne(x int) (res int) {
	defer func(b int) { res = b + 1 }(x)
	x = 1000
	return 0
}

func double(x int) (res int) {
	defer func() { res *= 2 }()
	return x
}

func main() {
	if plusOne(7) != 8 {
		wrong()
	}
	if double(21) != 42 {
		wrong()
	}
}
