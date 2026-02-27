// div_zero verifies that integer division by a statically-known zero is detected (#55).
//
// In Go, integer division by zero panics at runtime: "integer divide by zero".
// go vet: pass, go test -race: pass.
// Giri: detects the zero divisor in the BinOp and records a violation.
//
// Expected: 1 violation, "division by zero".
package main

func ratio(a, b int) int {
	return a / b
}

func main() {
	_ = ratio(10, 0)
}
