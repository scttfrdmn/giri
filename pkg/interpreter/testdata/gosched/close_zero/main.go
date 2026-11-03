// close_zero: receivers waiting on a channel get the buffered values, then
// the zero value with ok=false once it is closed. Native Go: got is 7+8 and
// the last receive reports !ok, so trap indexes out of range.
package main

func trap(n int) {
	xs := make([]int, 15)
	_ = xs[n]
}

func main() {
	ch := make(chan int, 2)
	done := make(chan int)
	go func() {
		got := 0
		for {
			v, ok := <-ch
			if !ok {
				done <- got + v // v is 0
				return
			}
			got += v
		}
	}()
	ch <- 7
	ch <- 8
	close(ch)
	trap(<-done)
}
