package exec

import (
	"runtime"
	"testing"
	"time"
)

// TestGoroutineKernel: goroutines in a call run to completion, blocking and
// resuming, before the call returns (#294).
func TestGoroutineKernel(t *testing.T) {
	res, err := Run(kernelFunc(t, "PingPong"), [][]any{{0}, {3}, {100}}, Options{})
	if err != nil {
		t.Fatal(err)
	}
	for i, want := range []int{0, 3, 100} {
		if res.Outcomes[i] != Returned {
			t.Fatalf("set %d: outcome %v, violations %v", i, res.Outcomes[i], res.Violations)
		}
		if got := res.Outputs[i][0]; got != want {
			t.Errorf("set %d: PingPong = %v, want %d", i, got, want)
		}
	}
	if len(res.Violations) != 0 {
		t.Errorf("violations: %v", res.Violations)
	}
}

// TestChannelValues: values sent on a channel arrive at the receiver, typed;
// a closed channel yields the element type's zero value (#295).
func TestChannelValues(t *testing.T) {
	res, err := Run(kernelFunc(t, "ChanSum"), [][]any{{0}, {10}}, Options{})
	if err != nil {
		t.Fatal(err)
	}
	for i, want := range []uint64{0, 55} {
		if res.Outcomes[i] != Returned {
			t.Fatalf("set %d: outcome %v, violations %v", i, res.Outcomes[i], res.Violations)
		}
		out := res.Outputs[i]
		if out[0] != want || out[1] != uint8(0) || out[2] != false {
			t.Errorf("set %d: ChanSum = %v, want [%d 0 false]", i, out, want)
		}
	}
}

// TestCallDeadlock: a call whose goroutines all wait panics with a deadlock
// violation, as native Go's fatal error, and the next call is unaffected.
func TestCallDeadlock(t *testing.T) {
	for _, opts := range []Options{{}, {SingleGoroutine: true}} {
		res, err := Run(kernelFunc(t, "WaitForever"), [][]any{{1}, {2}}, opts)
		if err != nil {
			t.Fatal(err)
		}
		for set := 0; set < 2; set++ {
			if res.Outcomes[set] != Panicked {
				t.Errorf("SingleGoroutine=%v set %d: outcome %v, want Panicked", opts.SingleGoroutine, set, res.Outcomes[set])
			}
		}
		n := 0
		for _, v := range res.Violations {
			if v.Category == "deadlock" {
				n++
			}
		}
		if n != 2 {
			t.Errorf("SingleGoroutine=%v: %d deadlock violations, want one per set: %v", opts.SingleGoroutine, n, res.Violations)
		}
	}
}

// TestNoHostGoroutineOutlivesCall: every host goroutine a call starts has
// exited by the time Run returns, including after a deadlock.
func TestNoHostGoroutineOutlivesCall(t *testing.T) {
	base := runtime.NumGoroutine()
	for _, k := range []string{"PingPong", "WaitForever"} {
		if _, err := Run(kernelFunc(t, k), [][]any{{5}, {9}}, Options{}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := Run(kernelFunc(t, "RacyWrites"), [][]any{{[]int{0}}}, Options{}); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for runtime.NumGoroutine() > base && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if n := runtime.NumGoroutine(); n > base {
		t.Fatalf("%d goroutines after Run, %d before", n, base)
	}
}
