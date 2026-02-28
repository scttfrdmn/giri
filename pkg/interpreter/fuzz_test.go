// Fuzz tests for the interpreter (issue #106).
//
// Run seed corpus only (CI):
//
//	go test -run=FuzzXxx ./pkg/interpreter/
//
// Run full fuzzer (local):
//
//	go test -fuzz=FuzzExecStdlibCall -fuzztime=30s ./pkg/interpreter/
package interpreter

import (
	"testing"

	"github.com/scttfrdmn/giri/pkg/shadow"
)

// FuzzExecStdlibCall fuzzes the stdlib intercept dispatch with random package
// paths and function names. Invariant: must not panic regardless of input.
func FuzzExecStdlibCall(f *testing.F) {
	// Seed corpus: known-good (pkgPath, name) pairs.
	seeds := [][2]string{
		{"strings", "Contains"},
		{"strings", "Split"},
		{"strings", "NewReader"},
		{"bytes", "NewBuffer"},
		{"bytes", "Join"},
		{"fmt", "Sprintf"},
		{"fmt", "Println"},
		{"strconv", "Itoa"},
		{"strconv", "Atoi"},
		{"math", "Sqrt"},
		{"math/rand", "Intn"},
		{"os", "Getenv"},
		{"time", "Now"},
		{"runtime", "GOOS"},
		{"net/http", "Get"},
		{"database/sql", "Open"},
		{"crypto/tls", "Dial"},
		{"testing", "Log"},
		{"unknown/pkg", "UnknownFunc"},
		{"", ""},
	}
	for _, s := range seeds {
		f.Add(s[0], s[1])
	}

	// Build a minimal interpreter for fuzz testing.
	interp := &Interpreter{
		Memory:         shadow.NewMemory(),
		goroutines:     make(map[int64]*Goroutine),
		channelSenders: make(map[ChanID]bool),
		channels:       make(map[ChanID]*chanEntry),
		mutexes:        make(map[shadow.AllocID]*mutexState),
		onceState:      make(map[shadow.AllocID]bool),
		valueStore:     make(map[shadow.AllocID]Value),
	}
	// Minimal goroutine for fuzz calls that need it.
	interp.goroutines[1] = &Goroutine{
		ID:     1,
		Status: GoroutineRunning,
		Stack:  []*Frame{},
	}

	f.Fuzz(func(t *testing.T, pkgPath, name string) {
		// Must not panic; the function returns (Value, bool).
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("panic in execStdlibCall(%q, %q): %v", pkgPath, name, r)
			}
		}()
		_, _ = interp.execStdlibCall(1, "fuzz:site", pkgPath, name, nil)
	})
}

// FuzzToInt64 fuzzes the toInt64 value conversion helper.
// Invariant: must not panic on any Value.
func FuzzToInt64(f *testing.F) {
	f.Add(int64(0))
	f.Add(int64(42))
	f.Add(int64(-1))
	f.Add(int64(1<<62 - 1))

	f.Fuzz(func(t *testing.T, v int64) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("panic in toInt64: %v", r)
			}
		}()
		// Test several raw types that toInt64 handles.
		_ = toInt64(Value{Raw: v})
		_ = toInt64(Value{Raw: uint64(v)})
		_ = toInt64(Value{Raw: float64(v)})
		_ = toInt64(Value{Raw: int(v)})
		_ = toInt64(Value{Raw: int32(v)})
	})
}
