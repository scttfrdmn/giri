package main

// Exercises FreeVars binding in execFunction (#19).
// The anonymous function captures x from the enclosing scope.
func main() {
	x := new(int)
	*x = 1
	add := func(n int) int {
		return *x + n // x is a FreeVar — was always zero before the fix
	}
	result := add(41)
	_ = result
}
