package main

// nil_deref exercises nil pointer dereference detection (#36).
// Reads from a nil *int pointer before any allocation.
// Expected: >= 1 "nil pointer" violation.
func main() {
	var p *int // nil pointer
	_ = *p     // nil dereference — should fire NilPointerDerefError
}
