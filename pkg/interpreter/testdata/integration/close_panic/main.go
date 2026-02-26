package main

// close_panic exercises send-on-closed-channel detection (#31).
// Closes a channel then attempts a send on it.
// Expected: >= 1 "closed channel" violation.
func main() {
	ch := make(chan struct{})
	close(ch)
	ch <- struct{}{} // send on closed channel — should fire violation
}
