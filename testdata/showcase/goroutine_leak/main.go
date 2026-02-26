// This program compiles cleanly, passes go vet, and passes go test -race.
// Giri detects the goroutine that permanently blocks on a channel with no sender.
package main

// worker reads results from a channel that the caller never writes to.
// In production code, this pattern appears when the done channel or result
// channel is never closed or written to after an error, leaving the worker
// goroutine permanently blocked (a goroutine leak).
//
// go vet cannot detect this because the channel operations are type-correct.
// go test -race does not flag it because there is no concurrent *data* access.
// Giri detects it by tracking channel senders at finalization time.
func worker(results chan int) {
	// Blocks forever: main exits without sending on results
	val := <-results
	_ = val
}

func main() {
	results := make(chan int)
	go worker(results) // spawns goroutine that will block on <-results
	// BUG: main returns without ever sending on results.
	// The worker goroutine is permanently leaked.
}
