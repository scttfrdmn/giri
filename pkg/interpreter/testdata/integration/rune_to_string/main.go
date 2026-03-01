// rune_to_string verifies that the []rune → string conversion is handled
// correctly by the interpreter (#142).
//
// False-positive canary: if string([]rune{...}) doesn't produce the expected
// string, the equality check fires a nil-slice OOB via the canary.
//
// Expected: 0 violations.
package main

func main() {
	// Round-trip: string → []rune → string must reproduce the original.
	original := "héllo"
	runes := []rune(original)
	back := string(runes)
	if back != original {
		var s []int
		_ = s[0] // false positive: only reached if []rune→string is wrong
	}

	// Build a string from explicit rune values.
	r := []rune{'G', 'o', '!'}
	result := string(r)
	if result != "Go!" {
		var s []int
		_ = s[0]
	}

	// Empty rune slice → empty string.
	empty := string([]rune{})
	if empty != "" {
		var s []int
		_ = s[0]
	}
}
