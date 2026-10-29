// recover_runtime_panic checks that recover() catches a run-time panic giri
// raises (index out of range), that the function returns through its recover
// path with the named result the deferred call set, and that main then
// continues (#259).
//
// When recovery works, main reaches the nil-map write below: a second,
// different violation. Without recovery, the out-of-range index ended the
// program and only that one violation was reported.
//
// Expected: 2 violations (the recovered out-of-bounds index, then a
// nil-map-write).
package main

func at(s []int, i int) (r int) {
	defer func() {
		if recover() != nil {
			r = -1
		}
	}()
	return s[i]
}

func main() {
	s := []int{1, 2, 3}
	if at(s, 5) == -1 {
		var m map[string]int
		m["reached"] = 1 // only reachable after a correct recovery
	}
}
