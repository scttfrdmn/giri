// fieldaddr_nil_struct verifies that accessing a field on a nil struct pointer
// is reported as a "nil-pointer-deref" violation.
//
// At runtime Go panics: "runtime error: invalid memory address or nil pointer dereference".
// Expected: 1 violation, category "nil-pointer-deref".
package main

type Point struct {
	X, Y int
}

func nilPoint() *Point { return nil }

func main() {
	p := nilPoint() // nil *Point
	_ = p.X         // FieldAddr on nil struct pointer: runtime panic
}
