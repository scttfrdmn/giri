// rand_v2 verifies that math/rand/v2.* functions are correctly intercepted
// (Go 1.22+).
//
// Exercises the package-level generators (Int64, Float64, Intn), the N
// generic helper, Shuffle, and Perm.
//
// Expected: 0 violations.
package main

import "math/rand/v2"

func main() {
	// Package-level scalar generators.
	_ = rand.Int64()
	_ = rand.Float64()
	_ = rand.Float32()
	_ = rand.Uint64()

	// Bounded generator.
	n := rand.IntN(100)
	_ = n

	// N[T] generic helper (Go 1.22+).
	m := rand.N(int64(50))
	_ = m

	// Perm returns a permutation slice.
	p := rand.Perm(5)
	_ = p

	// Shuffle is a no-op for violation analysis.
	s := []int{1, 2, 3, 4, 5}
	rand.Shuffle(len(s), func(i, j int) {
		s[i], s[j] = s[j], s[i]
	})
}
