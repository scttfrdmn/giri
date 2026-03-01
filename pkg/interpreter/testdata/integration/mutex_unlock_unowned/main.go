// mutex_unlock_unowned verifies that unlocking an already-unlocked mutex is detected.
// In Go, sync.Mutex.Unlock() on an unlocked mutex panics:
// "sync: unlock of unlocked mutex".
// Expected: 1 violation (mutex-unlock).
package main

import "sync"

func main() {
	var mu sync.Mutex
	mu.Lock()
	mu.Unlock()
	mu.Unlock() // second unlock — mutex is already unlocked
}
