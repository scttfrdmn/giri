// This program compiles cleanly, passes go vet, and passes go test -race.
// Giri detects the integer division by zero on the untested code path.
package main

// itemsPerBatch computes how many items fit in each batch.
// In production, batchSize is read from configuration and validated
// at startup — but testing only covers the happy path where batchSize > 0.
//
// go vet cannot detect this: the division is type-correct.
// go test -race passes: no goroutines, no concurrent access.
// Giri detects it: by interpreting the call with batchSize == 0,
// it reaches the divide-by-zero BinOp and records a DivisionByZeroError.
func itemsPerBatch(total, batchSize int) int {
	return total / batchSize // panics at runtime when batchSize == 0
}

func main() {
	// In real code, these might come from validated config — but the
	// validation has a gap that lets zero through on one code path.
	// BUG: the call with 0 panics with "integer divide by zero".
	_ = itemsPerBatch(100, 10)
	_ = itemsPerBatch(100, 5)
	_ = itemsPerBatch(100, 0) // zero slips through — panic in production
	_ = itemsPerBatch(100, 2)
}
