// send_closed_wakes: a goroutine sends on an unbuffered channel nobody
// receives from, and main closes it. Whether the close comes before or while
// the sender waits, native Go panics: send on closed channel. In giri the
// sender is waiting when main closes the channel, and wakes to panic.
// Native -race also reports the unordered send and close as a data race;
// giri doesn't model close as an access yet.
package main

import "sync"

func main() {
	ch := make(chan int)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		ch <- 1
	}()
	// Let the sender run first: main waits on a goroutine started after it.
	yield := make(chan struct{})
	go func() { close(yield) }()
	<-yield
	close(ch)
	wg.Wait()
}
