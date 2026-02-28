// reflect_type_of verifies that reflect.TypeOf, reflect.ValueOf, and basic
// reflect.Type / reflect.Value methods are intercepted (#86).
//
// Expected: 0 violations.
package main

import "reflect"

type Point struct {
	X, Y int
}

func main() {
	// reflect.TypeOf returns a non-nil reflect.Type.
	t := reflect.TypeOf(Point{})
	_ = t

	// reflect.ValueOf returns a reflect.Value.
	v := reflect.ValueOf(Point{X: 1, Y: 2})
	_ = v

	// Kind on a struct type.
	k := reflect.TypeOf(Point{}).Kind()
	_ = k

	// NumField on a struct type.
	n := reflect.TypeOf(Point{}).NumField()
	_ = n // 2

	// reflect.New returns a Value holding a pointer.
	ptr := reflect.New(t)
	_ = ptr

	// reflect.Zero returns the zero value for a type.
	z := reflect.Zero(t)
	_ = z

	// IsValid on the result of ValueOf.
	ok := reflect.ValueOf(42).IsValid()
	_ = ok // true
}
