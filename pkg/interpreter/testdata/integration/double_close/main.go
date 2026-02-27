// double_close verifies that closing an already-closed channel is detected (#52).
//
// In Go, closing a closed channel panics at runtime: "close of closed channel".
// go vet: pass, go test -race: pass (only the second close would crash at runtime).
// Giri: detects the second close as a violation.
//
// Expected: 1 violation, "closed channel".
package main

func main() {
	ch := make(chan int)
	close(ch)
	close(ch) // close of closed channel
}
