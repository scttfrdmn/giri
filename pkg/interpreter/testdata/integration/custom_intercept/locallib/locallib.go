// Package locallib simulates an external library that Giri cannot interpret
// directly. Its functions have real bodies — they are intercepted in the test
// via Config.Intercepts so the interpreter never has to execute them.
package locallib

// Compute returns an expensive-to-compute result. In the intercepted version
// the callback simply returns a sentinel value without executing the body.
func Compute(n int) int {
	// Intentionally non-trivial body so the interpreter would diverge without
	// an intercept (the loop runs n times).
	sum := 0
	for i := 0; i < n; i++ {
		sum += i
	}
	return sum
}

// MustAlloc allocates a buffer of size n. The intercepted version returns an
// opaque non-nil value so downstream code can call methods on the result.
func MustAlloc(n int) []byte {
	return make([]byte, n)
}
