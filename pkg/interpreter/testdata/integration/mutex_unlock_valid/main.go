// mutex_unlock_valid verifies that a correct lock/unlock sequence produces no violations.
// Expected: 0 violations.
package main

import "sync"

func main() {
	var mu sync.Mutex

	mu.Lock()
	mu.Unlock()

	mu.Lock()
	mu.Unlock()
}
