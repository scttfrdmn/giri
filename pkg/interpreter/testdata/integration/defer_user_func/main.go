// defer_user_func verifies that a deferred call to a named user-defined
// function with an explicit pointer argument is properly executed (#47).
//
// Without the fix, executeDeferred silently dropped any non-arena deferred
// call, so the cleanup function never ran. With the fix, the SSA function body
// is interpreted via execFunction.
//
// Expected: 0 violations.
package main

func cleanup(x *int) {
	*x = 0
}

func work() {
	x := new(int)
	*x = 42
	defer cleanup(x)
}

func main() {
	work()
}
