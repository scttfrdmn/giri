// iface_nil_compare checks == and != between interface values and nil
// (#259). A non-nil interface holding a user-defined error compared as
// neither equal nor unequal to nil, so error branches were never taken.
//
// Each comparison that is true natively has wrong() in its else branch, and
// each that is false has it in its then branch: one violation per wrong
// comparison.
// Expected: 0 violations.
package main

type myErr struct{ code int }

func (e *myErr) Error() string { return "fail" }

func wrong() {
	var t []int
	_ = t[0]
}

func check(fail bool) error {
	if fail {
		return &myErr{code: 1}
	}
	return nil
}

func main() {
	if err := check(true); err != nil {
	} else {
		wrong()
	}
	if err := check(false); err == nil {
	} else {
		wrong()
	}
	var a, b interface{} = 3, 3
	if a == b {
	} else {
		wrong()
	}
	var c interface{} = "x"
	if a != c {
	} else {
		wrong()
	}
}
