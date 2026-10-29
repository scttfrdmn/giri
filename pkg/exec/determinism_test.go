package exec

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"testing"
)

// canonical serializes everything observable in a Result into bytes:
// outputs, outcomes, all counters, violations, and traces keyed by each
// value's name and source position.
func canonical(t *testing.T, r Result) []byte {
	t.Helper()
	type traceEntry struct {
		Value string
		Trace *ValueTrace
	}
	var traces [][]traceEntry
	for _, set := range r.Traces {
		var es []traceEntry
		for v, tr := range set {
			es = append(es, traceEntry{fmt.Sprintf("%s@%d=%s", v.Name(), v.Pos(), v.String()), tr})
		}
		sort.Slice(es, func(i, j int) bool { return es[i].Value < es[j].Value })
		traces = append(traces, es)
	}
	var viols []string
	for _, v := range r.Violations {
		viols = append(viols, fmt.Sprintf("%d %s %s", v.Set, v.Category, v.Message))
	}
	b, err := json.Marshal(struct {
		Outputs      [][]any
		ArgsAfter    [][]any
		Outcomes     []Outcome
		Steps        uint64
		StepsPerSet  []uint64
		StepsByClass map[string]uint64
		Violations   []string
		Exhausted    bool
		Traces       [][]traceEntry
		CacheHits    uint64
		PureExecuted uint64
	}{r.Outputs, r.ArgsAfter, r.Outcomes, r.Steps, r.StepsPerSet, classMap(r.StepsByClass), viols,
		r.Exhausted, traces, r.CacheHits, r.PureExecuted})
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func classMap(m map[Class]uint64) map[string]uint64 {
	out := map[string]uint64{}
	for c, n := range m {
		out[c.String()] = n
	}
	return out
}

func TestDeterministicResults(t *testing.T) {
	cases := []struct {
		fn   string
		args [][]any
	}{
		{"Noisy", [][]any{{100}, {7}, {1000}}},
		{"MapOrder", [][]any{{[]int{9, 3, 7, 1, 5, 3, 8, 2, 6, 4}}}},
		{"Accum", [][]any{{50}}},
		{"OOB", [][]any{{[]int{1}, 3}}}, // with a violation
		{"Strs", [][]any{{"déterminisme"}}},
		{"PrefixSum32", [][]any{{[]int32{3, 1, 4, 1, 5}, make([]int32, 5)}}},
	}
	for _, c := range cases {
		t.Run(c.fn, func(t *testing.T) {
			fn := kernelFunc(t, c.fn)
			opts := Options{Seed: 42, Trace: true}
			first, err := Run(fn, c.args, opts)
			if err != nil {
				t.Fatal(err)
			}
			want := canonical(t, first)
			for i := 0; i < 5; i++ {
				again, err := Run(fn, c.args, opts)
				if err != nil {
					t.Fatal(err)
				}
				if got := canonical(t, again); !bytes.Equal(got, want) {
					t.Fatalf("run %d differs:\n%s\n%s", i+2, got, want)
				}
			}
		})
	}
}

func TestSeedChangesRandomness(t *testing.T) {
	fn := kernelFunc(t, "Noisy")
	args := [][]any{{1 << 30}}
	a, _ := Run(fn, args, Options{Seed: 1})
	b, _ := Run(fn, args, Options{Seed: 2})
	if reflect.DeepEqual(a.Outputs, b.Outputs) {
		t.Errorf("seeds 1 and 2 gave the same draw %v", a.Outputs)
	}
}

func TestMapIterationSorted(t *testing.T) {
	res, err := Run(kernelFunc(t, "MapOrder"), [][]any{{[]int{9, 3, 7, 1, 3}}}, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if got := res.Outputs[0][0]; got != "1,3,7,9," {
		t.Errorf("MapOrder = %q, want sorted key order", got)
	}
}

// TestBudgetExhaustion: running out of MaxSteps returns partial results with
// Exhausted set, not an error, and records no violation for it.
func TestBudgetExhaustion(t *testing.T) {
	fn := kernelFunc(t, "Accum")
	full, err := Run(fn, [][]any{{10}}, Options{})
	if err != nil {
		t.Fatal(err)
	}
	perSet := full.Steps

	// The budget covers the first set and part of the second; the third
	// never runs.
	budget := perSet + perSet/2
	res, err := Run(fn, [][]any{{10}, {10}, {10}}, Options{MaxSteps: budget, Trace: true})
	if err != nil {
		t.Fatalf("exhaustion returned an error: %v", err)
	}
	if !res.Exhausted {
		t.Fatal("Exhausted not set")
	}
	if len(res.Outputs) != 2 || !reflect.DeepEqual(res.Outcomes, []Outcome{Returned, Exhausted}) {
		t.Fatalf("outcomes = %v, outputs = %v", res.Outcomes, res.Outputs)
	}
	if !reflect.DeepEqual(res.Outputs[0], full.Outputs[0]) || res.Outputs[1] != nil {
		t.Errorf("outputs = %v", res.Outputs)
	}
	if res.Steps != budget || res.StepsPerSet[0] != perSet || res.StepsPerSet[1] != budget-perSet {
		t.Errorf("steps = %d %v, budget %d", res.Steps, res.StepsPerSet, budget)
	}
	var sum uint64
	for _, n := range res.StepsByClass {
		sum += n
	}
	if sum != res.Steps {
		t.Errorf("partial class counters sum to %d, Steps = %d", sum, res.Steps)
	}
	if len(res.Violations) != 0 {
		t.Errorf("exhaustion recorded violations: %v", res.Violations)
	}
	if len(res.Traces) != 2 || len(res.Traces[1]) == 0 {
		t.Errorf("partial traces missing: %d sets", len(res.Traces))
	}

	// A budget exhausted exactly at a set boundary stops before the next set.
	res, _ = Run(fn, [][]any{{10}, {10}}, Options{MaxSteps: perSet})
	if !res.Exhausted || !reflect.DeepEqual(res.Outcomes, []Outcome{Returned, Exhausted}) || res.StepsPerSet[1] != 0 {
		t.Errorf("boundary: outcomes %v steps %v", res.Outcomes, res.StepsPerSet)
	}
}
