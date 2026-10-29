package exec

import (
	"fmt"
	"go/token"
	"reflect"
	"testing"

	"golang.org/x/tools/go/ssa"
)

// recordingCache memoizes every Store and serves it back on Lookup.
type recordingCache struct {
	entries        map[string]Value
	lookups, hits  int
	stores         int
	impureConsults []ssa.Value
}

func newRecordingCache() *recordingCache {
	return &recordingCache{entries: make(map[string]Value)}
}

func cacheKey(v ssa.Value, operands []Value) string {
	k := fmt.Sprintf("%p", v)
	for _, o := range operands {
		k += fmt.Sprintf("|%T:%v", o.Raw, o.Raw)
	}
	return k
}

func (c *recordingCache) Lookup(v ssa.Value, operands []Value) (Value, uint64, bool) {
	c.lookups++
	if !IsPure(v.(ssa.Instruction)) {
		c.impureConsults = append(c.impureConsults, v)
	}
	r, ok := c.entries[cacheKey(v, operands)]
	if ok {
		c.hits++
	}
	return r, 1, ok
}

func (c *recordingCache) Store(v ssa.Value, operands []Value, result Value, steps uint64) {
	c.stores++
	c.entries[cacheKey(v, operands)] = result
}

// TestCacheSecondRunExecutesNoPureInstructions runs kernels twice with one
// recording cache: the second run must execute zero pure instructions and
// produce identical outputs, counters and traces.
func TestCacheSecondRunExecutesNoPureInstructions(t *testing.T) {
	cases := []struct {
		fn   string
		args [][]any
	}{
		{"Accum", [][]any{{10}, {3}}},
		{"MixU64", [][]any{{uint64(1 << 63), uint64(3)}, {uint64(5), uint64(9)}}},
		{"Pop", [][]any{{uint64(0xF0F0)}}},
		{"ClassMem", [][]any{{[]int{4, 5, 6}}}},
		{"Poly32", [][]any{{float32(2.5)}}},
	}
	for _, c := range cases {
		t.Run(c.fn, func(t *testing.T) {
			fn := kernelFunc(t, c.fn)
			cache := newRecordingCache()
			opts := Options{Cache: cache, Trace: true}
			first, err := Run(fn, c.args, opts)
			if err != nil {
				t.Fatal(err)
			}
			second, err := Run(fn, c.args, opts)
			if err != nil {
				t.Fatal(err)
			}
			if first.PureExecuted == 0 {
				t.Fatal("first run executed no pure instructions")
			}
			if second.PureExecuted != 0 {
				t.Errorf("second run executed %d pure instructions", second.PureExecuted)
			}
			// Later argument sets of the first run may already hit entries
			// stored by earlier ones; every pure execution of the second run hits.
			if second.CacheHits != first.PureExecuted+first.CacheHits {
				t.Errorf("second run hits = %d, first run pure executions + hits = %d", second.CacheHits, first.PureExecuted+first.CacheHits)
			}
			if !reflect.DeepEqual(first.Outputs, second.Outputs) {
				t.Errorf("outputs differ: %v vs %v", first.Outputs, second.Outputs)
			}
			if first.Steps != second.Steps || !reflect.DeepEqual(first.StepsByClass, second.StepsByClass) ||
				!reflect.DeepEqual(first.StepsPerSet, second.StepsPerSet) {
				t.Errorf("counters differ: %d %v vs %d %v", first.Steps, first.StepsByClass, second.Steps, second.StepsByClass)
			}
			if !reflect.DeepEqual(first.Traces, second.Traces) {
				t.Error("traces differ between cached and uncached runs")
			}
			if len(cache.impureConsults) > 0 {
				t.Errorf("cache consulted for impure instructions: %v", cache.impureConsults)
			}
		})
	}
}

// TestCacheNotStoredOnViolation: a pure instruction that panics (integer
// divide by zero) must not be cached, so a later run still reports it.
func TestCacheNotStoredOnViolation(t *testing.T) {
	fn := kernelFunc(t, "DivMod")
	cache := newRecordingCache()
	for run := 0; run < 2; run++ {
		res, err := Run(fn, [][]any{{uint64(1), uint64(0)}}, Options{Cache: cache})
		if err != nil {
			t.Fatal(err)
		}
		if len(res.Violations) == 0 || res.Outcomes[0] != Panicked {
			t.Errorf("run %d: violation masked by cache (%v, %v)", run, res.Violations, res.Outcomes)
		}
	}
}

// TestIsPure checks the purity classification instruction by instruction.
func TestIsPure(t *testing.T) {
	type expect struct {
		match func(ssa.Instruction) bool
		pure  bool
		what  string
	}
	isBin := func(op token.Token) func(ssa.Instruction) bool {
		return func(i ssa.Instruction) bool { b, ok := i.(*ssa.BinOp); return ok && b.Op == op }
	}
	isCall := func(name string) func(ssa.Instruction) bool {
		return func(i ssa.Instruction) bool {
			c, ok := i.(*ssa.Call)
			return ok && c.Call.Value.Name() == name
		}
	}
	is := func(f func(ssa.Instruction) bool) func(ssa.Instruction) bool { return f }
	checks := map[string][]expect{
		"ClassMem": {
			{isBin(token.SHL), true, "shift"},
			{isBin(token.LSS), true, "compare"},
			{isCall("OnesCount64"), true, "concrete pure intercept"},
			{isCall("helper"), false, "interpreted call"},
			{isCall("len"), false, "builtin"},
			{is(func(i ssa.Instruction) bool { _, ok := i.(*ssa.Convert); return ok }), true, "int→uint64 convert"},
			{is(func(i ssa.Instruction) bool { u, ok := i.(*ssa.UnOp); return ok && u.Op == token.MUL }), false, "load"},
			{is(func(i ssa.Instruction) bool { _, ok := i.(*ssa.Store); return ok }), false, "store"},
			{is(func(i ssa.Instruction) bool { _, ok := i.(*ssa.IndexAddr); return ok }), false, "index address"},
			{is(func(i ssa.Instruction) bool { _, ok := i.(*ssa.Phi); return ok }), false, "phi"},
			{is(func(i ssa.Instruction) bool { _, ok := i.(*ssa.MakeSlice); return ok }), false, "alloc"},
		},
		"SortCopy": {
			{isCall("Ints"), false, "sort.Ints (writes memory)"},
		},
		"Strs": {
			{is(func(i ssa.Instruction) bool { c, ok := i.(*ssa.Convert); return ok && c.Type().String() == "[]byte" }), false, "string→[]byte"},
			{isCall("ToUpper"), true, "strings.ToUpper"},
		},
		"Fib": {
			{isCall("Fib"), false, "recursive interpreted call"},
		},
	}
	for fnName, exps := range checks {
		fn := kernelFunc(t, fnName)
		for _, e := range exps {
			found := false
			for _, b := range fn.Blocks {
				for _, instr := range b.Instrs {
					if e.match(instr) {
						found = true
						if got := IsPure(instr); got != e.pure {
							t.Errorf("%s: IsPure(%s %v) = %v, want %v", fnName, e.what, instr, got, e.pure)
						}
					}
				}
			}
			if !found {
				t.Errorf("%s: no instruction for %s", fnName, e.what)
			}
		}
	}
}
