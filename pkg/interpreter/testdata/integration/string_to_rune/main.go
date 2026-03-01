// string_to_rune verifies that the string → []rune conversion is handled
// correctly by the interpreter (#142).
//
// False-positive canary: if []rune("héllo") doesn't yield a 5-element slice,
// the length check fires an OOB access. Without the fix, the conversion passes
// through unchanged (a string, not []Value), and len() returns Value{} →
// the "!= 5" comparison returns Value{} → ssa.If takes the TRUE default
// branch → nil-slice OOB fires as a false positive.
//
// Expected: 0 violations.
package main

func main() {
	s := "héllo" // 5 Unicode codepoints (é = U+00E9)
	r := []rune(s)

	// Check that the rune slice has the correct length (5, not 6 bytes).
	if len(r) != 5 {
		var s []int
		_ = s[0] // false positive: only reached if []rune conversion is wrong
	}

	// Check the second rune is 'é' (U+00E9 = 233).
	if len(r) >= 2 && r[1] != 'é' {
		var s []int
		_ = s[0] // false positive: only reached if rune value is wrong
	}

	// ASCII string: rune slice length == byte length.
	ascii := "hello"
	ra := []rune(ascii)
	if len(ra) != 5 {
		var s []int
		_ = s[0]
	}
}
