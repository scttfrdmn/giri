package main

// goroutine_leak demonstrates a goroutine that blocks forever on a channel
// receive because no sender ever sends on the channel.
// Giri detects this as a goroutine leak.
// Expected: 1 violation with category "goroutine-leak".

func receiver(c chan int) {
	<-c // blocks forever — no goroutine ever sends on c
}

func main() {
	ch := make(chan int)
	go receiver(ch) // spawns goroutine that will block
	// main exits without sending on ch
}
