// once_block: a sync.Once body waits for a goroutine. Native Go: Do returns,
// so trap indexes out of range.
package main

import "sync"

func trap(n int) {
	xs := []int{0}
	_ = xs[n]
}

func main() {
	var once sync.Once
	ready := make(chan struct{})
	go func() { close(ready) }()
	ok := 0
	once.Do(func() {
		<-ready
		ok = 1
	})
	trap(ok)
}
