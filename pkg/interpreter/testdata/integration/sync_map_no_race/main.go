// sync_map_no_race verifies that concurrent access to a sync.Map does not
// produce a data-race violation. sync.Map is internally synchronized;
// the interpreter models its methods as noops to avoid false positives.
//
// Expected: 0 violations.
package main

import "sync"

func storeValue(m *sync.Map, done chan<- struct{}) {
	m.Store("key", 42)
	done <- struct{}{}
}

func main() {
	var m sync.Map
	done := make(chan struct{})
	m.Store("key", 1)
	go storeValue(&m, done)
	m.Load("key") // concurrent — but sync.Map is safe
	<-done
}
