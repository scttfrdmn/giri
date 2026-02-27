// type_switch_nil verifies that a nil interface value dispatches to the default case,
// not to the first typed case.
//
// The *Dog case body contains a misaligned unsafe.Pointer access (Rule 1).
// If a nil interface incorrectly enters the *Dog case (old bug), Giri reports a
// "rule 1" violation. With correct dispatch (nil → default), 0 violations.
// Expected: 0 violations.
package main

import "unsafe"

type Animal interface{ Sound() string }
type Dog struct{}

func (d *Dog) Sound() string { return "woof" }

func checkAnimal(a Animal) {
	switch a.(type) {
	case *Dog:
		// Intentionally misaligned — only safe to reach for an actual *Dog.
		// If nil-interface dispatch is broken, this fires a Rule 1 violation.
		var b [5]byte
		_ = *(*uint32)(unsafe.Pointer(&b[1])) // offset 1 mod 4 != 0
	default:
		// nil interface correctly dispatches here: no violation.
	}
}

func main() {
	var a Animal // nil interface — concrete type unknown
	checkAnimal(a)
}
