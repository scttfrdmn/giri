package exec

import (
	"go/token"
	"reflect"
	"testing"

	"golang.org/x/tools/go/ssa"
)

// accumValues returns Accum's accumulator phi (s) and its update (s + i*i).
func accumValues(t *testing.T) (*ssa.Function, ssa.Value, ssa.Value) {
	fn := kernelFunc(t, "Accum")
	var phi, add ssa.Value
	for _, b := range fn.Blocks {
		for _, instr := range b.Instrs {
			if p, ok := instr.(*ssa.Phi); ok && p.Comment == "s" {
				phi = p
			}
		}
	}
	for _, b := range fn.Blocks {
		for _, instr := range b.Instrs {
			if op, ok := instr.(*ssa.BinOp); ok && op.Op == token.ADD && op.X == phi {
				add = op
			}
		}
	}
	if phi == nil || add == nil {
		t.Fatal("accumulator phi/add not found")
	}
	return fn, phi, add
}

func TestTraces(t *testing.T) {
	fn, phi, add := accumValues(t)
	res, err := Run(fn, [][]any{{4}, {2}}, Options{Trace: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Traces) != 2 {
		t.Fatalf("got %d trace sets, want 2", len(res.Traces))
	}
	// The loop header runs n+1 times; the body n times.
	for set, want := range []struct{ phi, add []int64 }{
		{phi: []int64{0, 1, 5, 14, 30}, add: []int64{1, 5, 14, 30}},
		{phi: []int64{0, 1, 5}, add: []int64{1, 5}},
	} {
		tr := res.Traces[set]
		if got := tr[phi]; got == nil || !reflect.DeepEqual(got.Ints, want.phi) || got.Truncated || got.Kind != TraceInt {
			t.Errorf("set %d phi trace = %+v, want %v", set, got, want.phi)
		}
		if got := tr[add]; got == nil || !reflect.DeepEqual(got.Ints, want.add) || got.Count != len(want.add) {
			t.Errorf("set %d add trace = %+v, want %v", set, got, want.add)
		}
	}
	// Every traced value belongs to Accum itself, and only executed values appear.
	for v := range res.Traces[0] {
		if v.(ssa.Instruction).Parent() != fn {
			t.Errorf("traced foreign value %v", v)
		}
	}
}

func TestTraceCapTruncates(t *testing.T) {
	fn, phi, add := accumValues(t)
	res, err := Run(fn, [][]any{{4}}, Options{Trace: true, TraceCap: 3})
	if err != nil {
		t.Fatal(err)
	}
	got := res.Traces[0][phi]
	if !reflect.DeepEqual(got.Ints, []int64{0, 1, 5}) || !got.Truncated || got.Count != 5 {
		t.Errorf("capped phi trace = %+v", got)
	}
	if a := res.Traces[0][add]; !a.Truncated || a.Len() != 3 {
		t.Errorf("capped add trace = %+v", a)
	}
}

func TestTraceKindsAndOff(t *testing.T) {
	res, err := Run(kernelFunc(t, "Strs"), [][]any{{"héllo"}}, Options{Trace: true})
	if err != nil {
		t.Fatal(err)
	}
	kinds := map[TraceKind]bool{}
	for _, tr := range res.Traces[0] {
		kinds[tr.Kind] = true
		if tr.Kind == TraceOther && tr.Len() != 0 {
			t.Errorf("TraceOther recorded values: %+v", tr)
		}
	}
	for _, k := range []TraceKind{TraceInt, TraceString, TraceOther} {
		if !kinds[k] {
			t.Errorf("no %v trace in Strs", k)
		}
	}
	res, _ = Run(kernelFunc(t, "Accum"), [][]any{{3}}, Options{})
	if res.Traces != nil {
		t.Error("traces recorded with Trace off")
	}
}
