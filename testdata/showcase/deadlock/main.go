// This program compiles cleanly, passes go vet, and passes go test -race.
// Giri detects the global deadlock: all goroutines are blocked with no way out.
package main

// recv blocks indefinitely waiting for a value on the channel.
// In this program no goroutine ever sends — both participants wait for each
// other, forming a two-party deadlock.
//
// go vet cannot detect this: the channel operations are type-correct.
// go test -race does not flag it: there is no concurrent *data* access.
// Giri detects it at finalization: every goroutine is in GoroutineBlocked
// state and none has finished — a global deadlock.
func recv(ch chan int) {
	<-ch
}

func main() {
	ch := make(chan int)

	// Spawn a goroutine that waits for a value on ch.
	// Nobody will ever send on ch, so this goroutine blocks forever.
	go recv(ch)

	// Main also waits for a value on ch.
	// BUG: no goroutine sends on ch — both goroutines are permanently blocked.
	recv(ch)
}
