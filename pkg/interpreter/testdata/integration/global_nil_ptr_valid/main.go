// global_nil_ptr_valid verifies that a package-level *string that IS explicitly
// initialized inside main() can be read without violation (#147).
//
// This is the counter-case to global_nil_ptr: it ensures the handleLoad
// uninitialized-zero-return path does not interfere when the global has a
// concrete value written via handleStore before the load.
//
// Expected: 0 violations.
package main

var s *string

func main() {
	greeting := "hello"
	s = &greeting // explicit init: store *string into global

	// Read back through the pointer — must succeed without violation.
	if *s != "hello" {
		var x []int
		_ = x[0] // false positive canary: only reached if *s is wrong
	}
}
