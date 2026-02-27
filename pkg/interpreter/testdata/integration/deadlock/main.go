// deadlock verifies that a global deadlock (all goroutines blocked) is detected (#56).
//
// Both main and a spawned goroutine block on the same unbuffered channel with
// no sender. Go's runtime would print "all goroutines are asleep — deadlock!"
// and abort. Giri detects this at finalization time when no goroutine has
// finished and all are GoroutineBlocked.
//
// go vet: pass, go test -race: pass (no concurrent data access).
// Giri: detects the global deadlock.
//
// Expected: >= 1 violation, "deadlock".
package main

func recv(ch chan int) {
	<-ch
}

func main() {
	ch := make(chan int)
	go recv(ch) // spawned goroutine blocks on <-ch
	recv(ch)    // main also blocks on <-ch — nobody ever sends
}
