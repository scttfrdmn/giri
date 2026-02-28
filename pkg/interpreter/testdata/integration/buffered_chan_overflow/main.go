// buffered_chan_overflow verifies that a goroutine blocked trying to send into
// a full buffered channel is reported as a goroutine leak when no receiver
// ever drains the channel.
//
// ch := make(chan int, 1) — capacity 1.
// main sends one value (fills the buffer), then spawns a goroutine that tries
// to send a second value. The buffer is already full and no receiver exists,
// so the goroutine blocks forever — goroutine leak.
//
// Expected: 1 violation, category "goroutine leak".
package main

func sender(ch chan<- int) {
	ch <- 2 // blocks: buffer is full, no receiver
}

func main() {
	ch := make(chan int, 1)
	ch <- 1 // fills the buffer (non-blocking)
	go sender(ch)
}
