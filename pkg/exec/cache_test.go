package exec

import (
	"fmt"
	"go/token"
	"go/types"
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
	if !mayBeCached(v.(ssa.Instruction)) {
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

// mayBeCached is the test's own statement of which instructions the hook may
// be offered, written independently of IsPure so a misclassification there
// is caught (#256): scalar BinOps, UnOps other than load and receive, scalar
// Convert/ChangeType, and calls to package-level functions of the pure
// stdlib packages with scalar arguments and result.
func mayBeCached(instr ssa.Instruction) bool {
	scalar := func(t types.Type) bool {
		b, ok := t.Underlying().(*types.Basic)
		return ok && b.Kind() != types.UnsafePointer && b.Kind() != types.UntypedNil && b.Kind() != types.Invalid
	}
	v, ok := instr.(ssa.Value)
	if !ok || !scalar(v.Type()) {
		return false
	}
	switch i := instr.(type) {
	case *ssa.BinOp:
		return scalar(i.X.Type()) && scalar(i.Y.Type())
	case *ssa.UnOp:
		return i.Op != token.MUL && i.Op != token.ARROW && scalar(i.X.Type())
	case *ssa.Convert:
		return scalar(i.X.Type())
	case *ssa.ChangeType:
		return scalar(i.X.Type())
	case *ssa.Call:
		f := i.Call.StaticCallee()
		if f == nil || i.Call.IsInvoke() || f.Signature.Recv() != nil || f.Pkg == nil {
			return false
		}
		switch f.Pkg.Pkg.Path() {
		case "math", "math/bits", "strings", "strconv", "unicode", "unicode/utf8":
		default:
			return false
		}
		for _, a := range i.Call.Args {
			if !scalar(a.Type()) {
				return false
			}
		}
		return true
	}
	return false
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
		"PurityMix": {
			{is(func(i ssa.Instruction) bool { u, ok := i.(*ssa.UnOp); return ok && u.Op == token.SUB }), true, "negation"},
			{is(func(i ssa.Instruction) bool { u, ok := i.(*ssa.UnOp); return ok && u.Op == token.XOR }), true, "bitwise complement"},
			{is(func(i ssa.Instruction) bool { u, ok := i.(*ssa.UnOp); return ok && u.Op == token.NOT }), true, "logical not"},
			{is(func(i ssa.Instruction) bool { _, ok := i.(*ssa.ChangeType); return ok }), true, "float64→celsius"},
			{is(func(i ssa.Instruction) bool { _, ok := i.(*ssa.Extract); return ok }), false, "extract from a tuple"},
			{is(func(i ssa.Instruction) bool { _, ok := i.(*ssa.Index); return ok }), false, "index of an array value"},
			{is(func(i ssa.Instruction) bool { _, ok := i.(*ssa.Field); return ok }), false, "field of a struct value"},
			{isCall("divmod7"), false, "multi-result interpreted call"},
		},
	}
	unsafek, err := LoadDir("testdata/unsafek")
	if err != nil {
		t.Fatal(err)
	}
	isConv := func(from, to string) func(ssa.Instruction) bool {
		return func(i ssa.Instruction) bool {
			c, ok := i.(*ssa.Convert)
			return ok && c.X.Type().String() == from && c.Type().String() == to
		}
	}
	unsafeChecks := []expect{
		{isConv("unsafe.Pointer", "uintptr"), false, "unsafe.Pointer→uintptr"},
		{isConv("uintptr", "unsafe.Pointer"), false, "uintptr→unsafe.Pointer"},
		{isConv("int", "uintptr"), true, "int→uintptr"},
		{isConv("*int", "unsafe.Pointer"), false, "*int→unsafe.Pointer"},
		{isBin(token.ADD), true, "uintptr addition (static types are scalar; run-time values are checked separately)"},
	}
	funcOf := func(name string) *ssa.Function {
		if name == "PtrArith" {
			return unsafek.Func(name)
		}
		return kernelFunc(t, name)
	}
	checks["PtrArith"] = unsafeChecks
	for fnName, exps := range checks {
		fn := funcOf(fnName)
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

// TestCacheHitKeepsGCSafepoint: a cache hit on a pure intercept call still
// runs the call site's GC-safepoint check, so a uintptr held across the call
// is reported whether or not the call's result came from the cache. The hit
// is set up three ways: an earlier argument set in the same Run, an earlier
// Run sharing the hook, and a hook primed with detectors off.
func TestCacheHitKeepsGCSafepoint(t *testing.T) {
	pkg, err := LoadDir("testdata/unsafek")
	if err != nil {
		t.Fatal(err)
	}
	fn := pkg.Func("GCPoint")
	ref, err := Run(fn, [][]any{{4.0, true}}, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(ref.Violations) == 0 {
		t.Fatal("no violation without a cache: the kernel does not exercise rule 2")
	}
	want := len(ref.Violations)

	check := func(name string, res Result, err error, cache *recordingCache) {
		t.Helper()
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if res.CacheHits == 0 {
			t.Fatalf("%s: no cache hit; the case does not test a hit", name)
		}
		if len(res.Violations) != want {
			t.Errorf("%s: %d violations with a cache hit, %d without: %v", name, len(res.Violations), want, res.Violations)
		}
		if len(res.Outputs) == 0 || !reflect.DeepEqual(res.Outputs[len(res.Outputs)-1], ref.Outputs[0]) {
			t.Errorf("%s: outputs %v, want %v", name, res.Outputs, ref.Outputs)
		}
	}

	c := newRecordingCache()
	res, err := Run(fn, [][]any{{4.0, false}, {4.0, true}}, Options{Cache: c})
	check("same run", res, err, c)

	c = newRecordingCache()
	if _, err := Run(fn, [][]any{{4.0, false}}, Options{Cache: c}); err != nil {
		t.Fatal(err)
	}
	res, err = Run(fn, [][]any{{4.0, true}}, Options{Cache: c})
	check("shared hook", res, err, c)

	c = newRecordingCache()
	if _, err := Run(fn, [][]any{{4.0, true}}, Options{Cache: c, SkipDetectors: true}); err != nil {
		t.Fatal(err)
	}
	res, err = Run(fn, [][]any{{4.0, true}}, Options{Cache: c})
	check("primed without detectors", res, err, c)
}

// TestCacheNeverChangesResults: for kernels including ones that raise
// violations, a run with a value cache reports exactly what a run without
// one does (outputs, outcomes, counters and violations), however the cache
// was primed: on the same arguments, on arguments in another order, or with
// detectors off (#256).
func TestCacheNeverChangesResults(t *testing.T) {
	unsafek, err := LoadDir("testdata/unsafek")
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		fn   *ssa.Function
		args [][]any
	}{
		{kernelFunc(t, "DivMod"), [][]any{{uint64(7), uint64(2)}, {uint64(1), uint64(0)}, {uint64(9), uint64(4)}}},
		{kernelFunc(t, "IndexAfterMath"), [][]any{{[]int{1, 2}, 1}, {[]int{1, 2}, 5}}},
		{kernelFunc(t, "Mix32"), [][]any{{int32(7), int32(3)}, {int32(-9), int32(2)}}},
		{kernelFunc(t, "Shifts"), [][]any{{uint32(5), uint(3)}, {uint32(5), uint(40)}}},
		{kernelFunc(t, "SqrtSum"), [][]any{{[]float64{4, 9}}, {[]float64{2, 4}}}},
		{kernelFunc(t, "Pop"), [][]any{{uint64(3)}, {uint64(0xFFFF)}}},
		{kernelFunc(t, "Strs"), [][]any{{"héllo"}, {"abc"}}},
		{kernelFunc(t, "PrefixSum32"), [][]any{{[]int32{1, 2, 3}, make([]int32, 3)}, {[]int32{4, 5}, make([]int32, 2)}}},
		{kernelFunc(t, "StructValues"), [][]any{{1.5, int32(3)}, {1.5, int32(4)}}},
		{unsafek.Func("GCPoint"), [][]any{{4.0, false}, {4.0, true}}},
		{unsafek.Func("PtrArith"), [][]any{{1}, {6}, {2}}},
	}
	summary := func(r Result) string {
		var v []string
		for _, x := range r.Violations {
			v = append(v, fmt.Sprintf("%d %s %s", x.Set, x.Category, x.Message))
		}
		return fmt.Sprintf("out=%v after=%v outcomes=%v steps=%d %v classes=%v viol=%v",
			r.Outputs, r.ArgsAfter, r.Outcomes, r.Steps, r.StepsPerSet, r.StepsByClass, v)
	}
	reverse := func(a [][]any) [][]any {
		out := make([][]any, len(a))
		for i := range a {
			out[i] = a[len(a)-1-i]
		}
		return out
	}
	for _, c := range cases {
		t.Run(c.fn.Name(), func(t *testing.T) {
			ref, err := Run(c.fn, c.args, Options{})
			if err != nil {
				t.Fatal(err)
			}
			want := summary(ref)
			totalHits := uint64(0)
			for _, prime := range []struct {
				name string
				args [][]any
				opts Options
			}{
				{"same args", c.args, Options{}},
				{"reversed args", reverse(c.args), Options{}},
				{"primed without detectors", reverse(c.args), Options{SkipDetectors: true}},
			} {
				cache := newRecordingCache()
				prime.opts.Cache = cache
				if _, err := Run(c.fn, prime.args, prime.opts); err != nil {
					t.Fatal(err)
				}
				got, err := Run(c.fn, c.args, Options{Cache: cache})
				if err != nil {
					t.Fatal(err)
				}
				if s := summary(got); s != want {
					t.Errorf("%s:\n got %s\nwant %s", prime.name, s, want)
				}
				if len(cache.impureConsults) > 0 {
					t.Errorf("%s: hook offered %v", prime.name, cache.impureConsults)
				}
				totalHits += got.CacheHits
			}
			if totalHits == 0 {
				t.Error("no cache hits: the case does not exercise the cache")
			}
		})
	}
}
