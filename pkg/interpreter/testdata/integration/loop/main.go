package main

// Exercises Phi node resolution with a zero-initialised counter (#18).
// The Phi node for i starts at 0; the buggy fallback would skip it.
func main() {
	sum := 0
	for i := 0; i < 5; i++ {
		sum += i
	}
	_ = sum
}
