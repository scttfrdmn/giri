// nil_channel_close verifies that close() on a nil channel is detected.
// In Go, close(nil) panics at runtime: "close of nil channel".
// Expected: 1 violation (nil-channel).
package main

func main() {
	var ch chan int // nil channel
	close(ch)
}
