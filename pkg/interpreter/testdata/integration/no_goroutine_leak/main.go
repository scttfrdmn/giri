package main

// no_goroutine_leak verifies that a goroutine blocked on a channel receive
// is NOT reported as a leak when a sender goroutine exists.
// Giri tracks channelSenders; if a send ever occurred on the channel,
// the receiving goroutine is not reported as leaked regardless of order.
// Expected: 0 violations.

func recvFrom(c chan int) {
	<-c // receives from the sender — not a leak
}

func sendTo(c chan int) {
	c <- 42 // provides the value to the receiver
}

func main() {
	ch := make(chan int)
	go recvFrom(ch) // GID 2 — reader
	go sendTo(ch)   // GID 3 — writer, runs first (round-robin picks higher GID)
}
