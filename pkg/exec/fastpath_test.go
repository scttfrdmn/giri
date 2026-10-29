package exec

import (
	"reflect"
	"testing"
)

// TestFastPathsPreserveResults reruns the round-trip kernels with each fast
// path and checks outputs, step counters and traces are unchanged.
func TestFastPathsPreserveResults(t *testing.T) {
	kernels := map[string][][]any{
		"MixU64":     {{uint64(1 << 63), uint64(3)}},
		"PrefixSums": {{[]int64{5, -2, 9}}},
		"Evens":      {{[]int{1, 2, 4, 7, 8}}},
		"Strs":       {{"héllo"}},
		"SortCopy":   {{[]int{3, 1, 2}}},
		"Fields":     {{3, 4}},
		"Fib":        {{8}},
		"IntLoop":    {{100}},
	}
	for name, args := range kernels {
		fn := kernelFunc(t, name)
		base, err := Run(fn, args, Options{Trace: true})
		if err != nil {
			t.Fatal(err)
		}
		for _, o := range []Options{
			{Trace: true, SingleGoroutine: true},
			{Trace: true, SkipDetectors: true},
			{Trace: true, SingleGoroutine: true, SkipDetectors: true},
		} {
			got, err := Run(fn, args, o)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got.Outputs, base.Outputs) || got.Steps != base.Steps ||
				!reflect.DeepEqual(got.StepsByClass, base.StepsByClass) || !reflect.DeepEqual(got.Traces, base.Traces) {
				t.Errorf("%s with %+v: results differ from the default run", name, o)
			}
		}
	}
}

func TestSingleGoroutineRejectsGo(t *testing.T) {
	fn := kernelFunc(t, "Spawns")
	res, err := Run(fn, [][]any{{1}}, Options{SingleGoroutine: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Violations) != 1 {
		t.Fatalf("violations = %v, want the goroutine limit", res.Violations)
	}
	res, _ = Run(fn, [][]any{{1}}, Options{})
	if len(res.Violations) != 0 || res.Outputs[0][0] != 1 {
		t.Errorf("default mode: %v %v", res.Violations, res.Outputs)
	}
}

func TestSkipDetectorsStillPanics(t *testing.T) {
	res, err := Run(kernelFunc(t, "OOB"), [][]any{{[]int{1, 2}, 5}}, Options{SkipDetectors: true})
	if err != nil {
		t.Fatal(err)
	}
	if res.Outcomes[0] != Panicked {
		t.Errorf("out-of-range index with SkipDetectors: outcome %v", res.Outcomes[0])
	}
}

// BenchmarkIntLoop measures interpretation cost per instruction on a
// straight-line integer loop with each fast-path combination. Reported as
// ns/instr (results recorded in docs/perf.md).
func BenchmarkIntLoop(b *testing.B) {
	fn := kernelFunc(b, "IntLoop")
	args := [][]any{{10_000}}
	for _, c := range []struct {
		name string
		opts Options
	}{
		{"default", Options{}},
		{"single-goroutine", Options{SingleGoroutine: true}},
		{"skip-detectors", Options{SkipDetectors: true}},
		{"both", Options{SingleGoroutine: true, SkipDetectors: true}},
	} {
		b.Run(c.name, func(b *testing.B) {
			var steps uint64
			for b.Loop() {
				res, err := Run(fn, args, c.opts)
				if err != nil {
					b.Fatal(err)
				}
				steps += res.Steps
			}
			b.ReportMetric(float64(b.Elapsed().Nanoseconds())/float64(steps), "ns/instr")
		})
	}
}

// BenchmarkSliceLoop is a memory-bound companion to BenchmarkIntLoop: a
// prefix sum over 10,000 elements (a load and a store per iteration), where
// shadow-memory checks and detectors are on the hot path.
func BenchmarkSliceLoop(b *testing.B) {
	fn := kernelFunc(b, "PrefixSums")
	xs := make([]int64, 10_000)
	for i := range xs {
		xs[i] = int64(i)
	}
	args := [][]any{{xs}}
	for _, c := range []struct {
		name string
		opts Options
	}{
		{"default", Options{}},
		{"skip-detectors", Options{SkipDetectors: true}},
		{"both", Options{SingleGoroutine: true, SkipDetectors: true}},
	} {
		b.Run(c.name, func(b *testing.B) {
			var steps uint64
			for b.Loop() {
				res, err := Run(fn, args, c.opts)
				if err != nil {
					b.Fatal(err)
				}
				steps += res.Steps
			}
			b.ReportMetric(float64(b.Elapsed().Nanoseconds())/float64(steps), "ns/instr")
		})
	}
}
