// oncefunc_probe: a sync.OnceFunc body waits for a goroutine started after
// OnceFunc returns. giri probes the body when OnceFunc is called; the wait
// there must not be reported as a deadlock. Native Go: runs cleanly.
package main

import "sync"

func main() {
	ready := make(chan struct{})
	f := sync.OnceFunc(func() { <-ready })
	go func() { close(ready) }()
	f()
}
