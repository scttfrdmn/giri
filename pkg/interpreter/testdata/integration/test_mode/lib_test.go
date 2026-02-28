package testmode

import "testing"

// TestSafeAdd verifies that SafeAdd produces no violations.
// Expected: 0 violations.
func TestSafeAdd(t *testing.T) {
	r := SafeAdd(2, 3)
	_ = r
}

// TestCounterRace exercises two goroutines that race on the Counter global.
// Expected: 1 data-race violation.
func TestCounterRace(t *testing.T) {
	done := make(chan struct{}, 2)
	go writeCounter(done)
	go readCounter(done)
	<-done
	<-done
}

func writeCounter(done chan<- struct{}) {
	Counter = 1
	done <- struct{}{}
}

func readCounter(done chan<- struct{}) {
	_ = Counter
	done <- struct{}{}
}
