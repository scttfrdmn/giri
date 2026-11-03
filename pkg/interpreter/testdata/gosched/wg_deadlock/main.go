// wg_deadlock: main waits on a WaitGroup that is never done. Native Go:
// fatal error: all goroutines are asleep - deadlock!
package main

import "sync"

func main() {
	var wg sync.WaitGroup
	wg.Add(1)
	wg.Wait()
}
