// select_wait: main's select waits until a goroutine sends on b.
// Native Go: the b case runs, so trap indexes out of range.
package main

func trap(n int) {
	xs := []int{0}
	_ = xs[n]
}

func main() {
	a, b := make(chan int), make(chan int)
	go func() { b <- 1 }()
	select {
	case <-a:
	case <-b:
		trap(1)
	}
}
