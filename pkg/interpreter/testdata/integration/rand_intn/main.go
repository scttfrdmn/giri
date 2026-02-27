// rand_intn verifies that math/rand.Intn and related functions are intercepted
// cleanly (#64). Without intercepts the interpreter tries to execute stdlib
// internals (atomics, global locked source) and may crash or produce wrong values.
//
// Expected: 0 violations.
package main

import "math/rand"

func main() {
	n := rand.Intn(10) // [0, 10)
	s := make([]byte, n+1)
	for i := range s {
		s[i] = byte(rand.Intn(256))
	}
	_ = rand.Float64()
	_ = rand.Int63()
	p := rand.Perm(5)
	_ = p
}
