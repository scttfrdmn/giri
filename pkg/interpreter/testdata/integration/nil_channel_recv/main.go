// nil_channel_recv verifies that a receive from a nil channel is detected.
// In Go, receiving from a nil channel blocks forever (deadlock).
// Expected: 1 violation (nil-channel).
package main

func main() {
	var ch chan int // nil channel
	_ = <-ch       // blocks forever in real Go
}
