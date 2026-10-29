package exec

import (
	"go/types"

	"golang.org/x/tools/go/ssa"

	"github.com/scttfrdmn/giri/pkg/interpreter"
)

// DefaultTraceCap is the per-value trace length used when Options.TraceCap
// is zero.
const DefaultTraceCap = 1024

// TraceKind is the representation of a traced value's sequence, chosen from
// the value's static type.
type TraceKind uint8

const (
	TraceInt    TraceKind = iota // signed integers → ValueTrace.Ints
	TraceUint                    // unsigned integers, uintptr → ValueTrace.Uints
	TraceFloat                   // float32, float64 → ValueTrace.Floats
	TraceBool                    // bool → ValueTrace.Bools
	TraceString                  // string → ValueTrace.Strings
	TraceOther                   // anything else (pointers, slices, tuples): Count only
)

// ValueTrace is the sequence of concrete values one SSA value took during one
// argument set's call: one entry per execution of the defining instruction,
// in execution order, including executions inside callee recursion of the
// same function. Exactly one of the slices is used, selected by Kind.
type ValueTrace struct {
	Kind    TraceKind
	Ints    []int64
	Uints   []uint64
	Floats  []float64
	Bools   []bool
	Strings []string

	// Count is the number of times the instruction executed, including
	// executions past the cap.
	Count int
	// Truncated is set when Count exceeded the cap; the slice holds the first
	// cap values only.
	Truncated bool
	// Opaque is set when an execution produced a value with no concrete
	// representation (for example a placeholder from a non-concrete
	// intercept); recording stopped at that point. Treat a truncated or
	// opaque trace as having no fingerprint.
	Opaque bool
}

// Len returns the number of recorded values.
func (t *ValueTrace) Len() int {
	switch t.Kind {
	case TraceInt:
		return len(t.Ints)
	case TraceUint:
		return len(t.Uints)
	case TraceFloat:
		return len(t.Floats)
	case TraceBool:
		return len(t.Bools)
	case TraceString:
		return len(t.Strings)
	}
	return 0
}

// Traces maps each value-producing instruction of the traced function to its
// trace. Instructions that never executed are absent.
type Traces map[ssa.Value]*ValueTrace

// traceKind picks the representation for values of type t.
func traceKind(t types.Type) TraceKind {
	b, ok := t.Underlying().(*types.Basic)
	if !ok {
		return TraceOther
	}
	info := b.Info()
	switch {
	case info&types.IsBoolean != 0:
		return TraceBool
	case info&types.IsString != 0:
		return TraceString
	case info&types.IsFloat != 0:
		return TraceFloat
	case info&types.IsUnsigned != 0:
		return TraceUint
	case info&types.IsInteger != 0:
		return TraceInt
	}
	return TraceOther
}

// tracer records the values of fn's own instructions into traces.
type tracer struct {
	fn     *ssa.Function
	cap    int
	traces Traces
}

var _ interpreter.Tracer = (*tracer)(nil)

func (tr *tracer) Record(v ssa.Value, val Value) {
	instr, ok := v.(ssa.Instruction)
	if !ok || instr.Parent() != tr.fn {
		return
	}
	t := tr.traces[v]
	if t == nil {
		if tup, ok := v.Type().(*types.Tuple); ok && tup.Len() == 0 {
			return // call with no result
		}
		t = &ValueTrace{Kind: traceKind(v.Type())}
		tr.traces[v] = t
	}
	t.Count++
	if t.Kind == TraceOther || t.Truncated || t.Opaque {
		return
	}
	if t.Len() >= tr.cap {
		t.Truncated = true
		return
	}
	switch t.Kind {
	case TraceInt:
		n, ok := val.Raw.(int64)
		if !ok {
			t.Opaque = true
			return
		}
		t.Ints = append(t.Ints, n)
	case TraceUint:
		n, ok := val.Raw.(int64)
		if !ok {
			t.Opaque = true
			return
		}
		t.Uints = append(t.Uints, uint64(n))
	case TraceFloat:
		f, ok := val.Raw.(float64)
		if !ok {
			t.Opaque = true
			return
		}
		t.Floats = append(t.Floats, f)
	case TraceBool:
		b, ok := val.Raw.(bool)
		if !ok {
			t.Opaque = true
			return
		}
		t.Bools = append(t.Bools, b)
	case TraceString:
		s, ok := val.Raw.(string)
		if !ok {
			t.Opaque = true
			return
		}
		t.Strings = append(t.Strings, s)
	}
}
