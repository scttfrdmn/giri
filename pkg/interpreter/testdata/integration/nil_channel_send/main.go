// nil_channel_send verifies that a send on a nil channel is detected.
// In Go, sending to a nil channel blocks forever (deadlock).
// Expected: 1 violation (nil-channel).
package main

func main() {
	var ch chan int // nil channel
	ch <- 42       // blocks forever in real Go
}
