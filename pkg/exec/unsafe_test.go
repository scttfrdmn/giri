package exec

import (
	"testing"

	"golang.org/x/tools/go/ssa"
)

// scalarCheckCache records every operand and result the hook sees whose Raw
// is not one of the documented scalar representations. The check is written
// out here rather than calling into the interpreter, so it is independent of
// the code under test.
type scalarCheckCache struct {
	recordingCache
	bad []string
}

func documentedScalar(v Value) bool {
	switch v.Raw.(type) {
	case int64, float64, bool, string, complex128:
		return v.Provenance == nil
	}
	return false
}

func (c *scalarCheckCache) Lookup(v ssa.Value, ops []Value) (Value, uint64, bool) {
	for _, o := range ops {
		if !documentedScalar(o) {
			c.bad = append(c.bad, v.String())
		}
	}
	return c.recordingCache.Lookup(v, ops)
}

func (c *scalarCheckCache) Store(v ssa.Value, ops []Value, r Value, steps uint64) {
	if !documentedScalar(r) {
		c.bad = append(c.bad, "result of "+v.String())
	}
	c.recordingCache.Store(v, ops, r, steps)
}

func unsafeFunc(t *testing.T, name string) *ssa.Function {
	t.Helper()
	pkg, err := LoadDir("testdata/unsafek")
	if err != nil {
		t.Fatal(err)
	}
	return pkg.Func(name)
}

// TestUintptrArithmetic: pointer arithmetic through uintptr addresses the
// right element in concrete mode, and the cache only ever sees scalars (#255).
func TestUintptrArithmetic(t *testing.T) {
	fn := unsafeFunc(t, "PtrArith")
	for _, opts := range []Options{{}, {SkipDetectors: true}} {
		c := &scalarCheckCache{recordingCache: *newRecordingCache()}
		opts.Cache = c
		res, err := Run(fn, [][]any{{0}, {1}, {3}, {1}}, opts)
		if err != nil {
			t.Fatal(err)
		}
		for i, want := range []int{1, 2, 4, 2} {
			if res.Outcomes[i] != Returned || res.Outputs[i][0] != want {
				t.Errorf("SkipDetectors=%v set %d: %v %v, want %d", opts.SkipDetectors, i, res.Outcomes[i], res.Outputs[i], want)
			}
		}
		if len(res.Violations) != 0 {
			t.Errorf("in-bounds arithmetic reported %v", res.Violations)
		}
		if res.PureExecuted == 0 {
			t.Error("no pure instruction offered to the cache; the case tests nothing")
		}
		if len(c.bad) > 0 {
			t.Errorf("hook saw non-scalar values: %v", c.bad)
		}
	}

	res, err := Run(unsafeFunc(t, "PtrDiff"), [][]any{{1, 5}, {2, 2}}, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Outputs[0][0] != 32 || res.Outputs[1][0] != 0 {
		t.Errorf("PtrDiff = %v, want 32 and 0", res.Outputs)
	}
}

// TestUintptrArithmeticOutOfBounds: arithmetic that leaves the allocation is
// still reported.
func TestUintptrArithmeticOutOfBounds(t *testing.T) {
	res, err := Run(unsafeFunc(t, "PtrArith"), [][]any{{6}}, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Violations) == 0 {
		t.Fatalf("no violation for a[6] via uintptr: outcome %v, outputs %v", res.Outcomes, res.Outputs)
	}
	t.Logf("reported: %s", res.Violations[0].Category)
}
