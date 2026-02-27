// delete_race verifies that delete(map, key) triggers race detection (#63).
//
// Two sibling goroutines access a shared map without synchronisation:
// one calls delete, the other reads. Since they have no happens-before
// relationship, this is a data race.
//
// Expected: 1 violation (data race on the map).
package main

func worker1(m map[string]int, done chan struct{}) {
	delete(m, "key")
	done <- struct{}{}
}

func worker2(m map[string]int, done chan struct{}) {
	_ = m["key"]
	done <- struct{}{}
}

func main() {
	m := map[string]int{"key": 42}
	done := make(chan struct{}, 2)
	go worker1(m, done)
	go worker2(m, done)
	<-done
	<-done
}
