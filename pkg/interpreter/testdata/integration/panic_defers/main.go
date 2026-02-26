package main

// Exercises ssa.Panic deferred-call execution (#20).
// The deferred increment of *x must run even though the function panics.
// If defers are skipped, the interpreter would report wrong behaviour.
func mayPanic(x *int) {
	defer func() { *x++ }()
	panic("intentional")
}

func main() {
	x := new(int)
	*x = 0
	mayPanic(x)
}
