package main

// Exercises the MaxSteps execution limit (#17).
// This loop runs 1,000,000 iterations; the test uses a small MaxSteps cap
// to verify the interpreter terminates and reports the step-limit violation.
func main() {
	i := 0
	for i < 1_000_000 {
		i++
	}
	_ = i
}
