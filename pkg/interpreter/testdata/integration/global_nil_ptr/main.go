// global_nil_ptr verifies that reading through a package-level *string that is
// never initialized (default nil) reports nil-pointer-deref, not a spurious
// out-of-bounds access (#147).
//
// Before the fix, handleLoad's fallthrough path returned the container's shadow
// pointer as the loaded value when no valueStore entry existed. The subsequent
// dereference then called CheckAccess with the 8-byte *string allocation and a
// 16-byte string size, producing a false "out-of-bounds" violation.
//
// After the fix, the offset-0 / no-valueStore-entry path returns Value{} (zero),
// which the next dereference correctly classifies as nil-pointer-deref.
//
// Expected: 1 violation (nil-pointer-deref).
package main

// s is a package-level *string, default-initialized to nil.
// Giri does not call init() before main(), so it is never set.
var s *string

func main() {
	// Dereference nil *string — must be nil-pointer-deref, not out-of-bounds.
	_ = *s
}
