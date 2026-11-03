// rwmutex_readers holds two read locks on one RWMutex at once, then takes the
// write lock. Native Go: runs cleanly. Before #293 giri shared one lock flag
// between readers and the writer, so the second RUnlock reported a false
// "unlock of unlocked mutex".
package main

import (
	"fmt"
	"sync"
)

func main() {
	var mu sync.RWMutex
	m := map[string]int{"a": 1}

	mu.RLock()
	mu.RLock()
	a := m["a"]
	mu.RUnlock()
	mu.RUnlock()

	mu.Lock()
	m["a"] = a + 1
	mu.Unlock()

	mu.RLock()
	defer mu.RUnlock()
	fmt.Println(m["a"])
}
