// unbuffered_deadlock: main sends on an unbuffered channel nobody receives
// from. Native Go: fatal error: all goroutines are asleep - deadlock!
// Before #295 unbuffered sends never blocked.
package main

func main() {
	ch := make(chan int)
	ch <- 1
}
