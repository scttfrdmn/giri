// Benchmark tests for interpreter hot paths (issue #107).
//
// Run: GOEXPERIMENT=arenas go test -bench=. -benchmem ./pkg/interpreter/
package interpreter

import (
	"testing"

	"github.com/scttfrdmn/giri/pkg/shadow"
)

// BenchmarkStdlibDispatchHit measures execStdlibCall for a known package (strings).
func BenchmarkStdlibDispatchHit(b *testing.B) {
	interp := &Interpreter{
		Memory:         shadow.NewMemory(),
		goroutines:     make(map[int64]*Goroutine),
		channelSenders: make(map[ChanID]bool),
		channels:       make(map[ChanID]*chanEntry),
		mutexes:        make(map[shadow.AllocID]*mutexState),
		onceState:      make(map[shadow.AllocID]bool),
		valueStore:     make(map[shadow.AllocID]Value),
	}
	interp.goroutines[1] = &Goroutine{ID: 1, Status: GoroutineRunning, Stack: []*Frame{}}
	args := []Value{{Raw: "hello world"}, {Raw: "world"}}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = interp.execStdlibCall(1, "bench:site", "strings", "Contains", args, nil)
	}
}

// BenchmarkStdlibDispatchMiss measures execStdlibCall for an unknown package.
func BenchmarkStdlibDispatchMiss(b *testing.B) {
	interp := &Interpreter{
		Memory:         shadow.NewMemory(),
		goroutines:     make(map[int64]*Goroutine),
		channelSenders: make(map[ChanID]bool),
		channels:       make(map[ChanID]*chanEntry),
		mutexes:        make(map[shadow.AllocID]*mutexState),
		onceState:      make(map[shadow.AllocID]bool),
		valueStore:     make(map[shadow.AllocID]Value),
	}
	interp.goroutines[1] = &Goroutine{ID: 1, Status: GoroutineRunning, Stack: []*Frame{}}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = interp.execStdlibCall(1, "bench:site", "unknown/pkg", "Func", nil, nil)
	}
}

// BenchmarkToInt64 measures the toInt64 conversion helper.
func BenchmarkToInt64(b *testing.B) {
	v := Value{Raw: int64(42)}
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = toInt64(v)
	}
}
