// Package exec is giri's library entry point: it interprets an arbitrary
// SSA function on caller-supplied argument sets and returns the function's
// outputs together with execution instrumentation (step counts by
// instruction class, detector violations, per-value traces).
//
// It exists for evaluators such as tumble that score many small candidate
// functions per second. Functions run in giri's concrete (value-faithful)
// mode, so outputs match native execution; detectors run as they do under
// the CLI unless Options.SkipDetectors is set.
//
// The API is additive and versioned with giri (see docs/exec-api.md).
// Building a program that imports this package requires GOEXPERIMENT=arenas,
// like the rest of giri.
package exec

import (
	"errors"
	"fmt"
	"go/types"
	"reflect"

	"golang.org/x/tools/go/ssa"

	"github.com/scttfrdmn/giri/pkg/interpreter"
	"github.com/scttfrdmn/giri/pkg/report"
	"github.com/scttfrdmn/giri/pkg/shadow"
)

// Version is the giri release this API ships with. pkg/exec is versioned with
// giri: changes are additive within the 0.x series (docs/exec-api.md).
const Version = report.Version

// DefaultMaxSteps is the step budget used when Options.MaxSteps is zero.
const DefaultMaxSteps = 10_000_000

// Options controls a Run.
type Options struct {
	// MaxSteps is the step budget for the whole Run, shared across all
	// argument sets in order. When it runs out the Run stops and returns what
	// was recorded with Result.Exhausted set. Zero means DefaultMaxSteps.
	MaxSteps uint64

	// TrackInit enables giri's uninitialized-read detector (off by default,
	// as under the CLI).
	TrackInit bool

	// Trace records, per argument set, the sequence of values taken by every
	// value-producing instruction of fn (Result.Traces). Recording is the
	// dominant memory cost of a Run; each sequence is capped at TraceCap
	// entries (zero means DefaultTraceCap).
	Trace    bool
	TraceCap int

	// Cache, if non-nil, is consulted for pure instructions (see CacheHook).
	Cache CacheHook

	// SingleGoroutine runs with max_goroutines == 1 and no scheduler: a go
	// statement records a "goroutine limit reached" violation instead of
	// spawning.
	SingleGoroutine bool

	// SkipDetectors disables shadow-memory checks and every detector, for a
	// function the caller has already verified clean with detectors on.
	// Outputs, step counts and traces are unchanged; run-time panics
	// (index out of range, nil dereference, divide by zero) are still raised.
	// See docs/perf.md for the measured speedup.
	SkipDetectors bool

	// Seed seeds every source of randomness the interpreter models: the
	// math/rand and math/rand/v2 intercepts and the Random/PCT schedulers.
	// Runs with equal SSA, arguments and Options produce identical Results.
	Seed int64
}

// Outcome describes how one argument set's call ended.
type Outcome uint8

const (
	// Returned means the function returned normally; its Outputs entry is set.
	Returned Outcome = iota
	// Panicked means the call ended in an unrecovered panic, including the
	// run-time panics giri raises (index out of range, nil dereference,
	// integer divide by zero).
	Panicked
	// Exhausted means the step budget ran out during this call.
	Exhausted
	// Opaque means the function returned, but a result could not be represented
	// as a Go value (it came from an intercept that returns a placeholder).
	Opaque
)

func (o Outcome) String() string {
	switch o {
	case Returned:
		return "returned"
	case Panicked:
		return "panicked"
	case Exhausted:
		return "exhausted"
	case Opaque:
		return "opaque"
	}
	return fmt.Sprintf("outcome(%d)", uint8(o))
}

// Violation is one detector finding, attributed to the argument set during
// whose call it was detected.
type Violation struct {
	Set      int    // index into the args passed to Run
	Category string // report category, e.g. "out-of-bounds"
	Message  string
	Err      error // the underlying giri error (wrapped with its call stack)
}

// Result is what a Run recorded.
type Result struct {
	// Outputs holds one entry per argument set that was run, each with one
	// Go value per function result (typed as the result's underlying Go
	// type: int8, uint64, []float32, ...). An entry is nil unless that set's
	// Outcome is Returned. When the budget runs out, Outputs (and Outcomes)
	// are shorter than the args passed in.
	Outputs  [][]any
	Outcomes []Outcome

	// ArgsAfter holds, for each argument set that was run, the contents of
	// its slice arguments after the call: one entry per parameter, typed like
	// Outputs (the parameter's underlying Go type), nil for non-slice
	// parameters and for a nil slice. A function that writes into a slice
	// argument reports what it wrote; one that doesn't reports its input.
	// An entry is nil unless that set's Outcome is Returned. The slices
	// passed to Run are never modified. Each slice argument gets its own
	// backing array, so aliasing between arguments is not modeled (#266).
	ArgsAfter [][]any

	// Steps is the number of SSA instructions executed across all argument
	// sets (callees included); StepsPerSet breaks it down by set, and
	// StepsByClass by instruction class (every class is present, possibly 0).
	// The sum of StepsByClass is Steps.
	Steps        uint64
	StepsPerSet  []uint64
	StepsByClass map[Class]uint64

	// Violations lists detector findings in detection order.
	Violations []Violation

	// Exhausted is set when MaxSteps ran out; the last Outcome is Exhausted.
	Exhausted bool

	// Traces holds one entry per argument set that was run when
	// Options.Trace is set, nil otherwise.
	Traces []Traces

	// CacheHits counts pure instructions satisfied by Options.Cache;
	// PureExecuted counts pure instructions actually executed. Both are zero
	// without a cache.
	CacheHits    uint64
	PureExecuted uint64
}

// Value is giri's interpreter value representation.
type Value = interpreter.Value

// Class is the cost class of an executed instruction. See interpreter.ClassOf
// for which SSA instructions fall in each class.
type Class = interpreter.InstrClass

// Instruction classes.
const (
	ClassArithmetic      = interpreter.ClassArithmetic
	ClassCompare         = interpreter.ClassCompare
	ClassLoad            = interpreter.ClassLoad
	ClassStore           = interpreter.ClassStore
	ClassIndex           = interpreter.ClassIndex
	ClassCallIntercepted = interpreter.ClassCallIntercepted
	ClassCallInterpreted = interpreter.ClassCallInterpreted
	ClassBuiltin         = interpreter.ClassBuiltin
	ClassBranch          = interpreter.ClassBranch
	ClassPhi             = interpreter.ClassPhi
	ClassAlloc           = interpreter.ClassAlloc
	ClassOther           = interpreter.ClassOther
	NumClasses           = interpreter.NumClasses
)

// UnsupportedTypeError reports a parameter or result type that v0
// marshaling does not cover (only integers, bools, floats, strings and
// slices of those are supported).
type UnsupportedTypeError struct {
	Func   string
	Result bool // true for a result, false for a parameter
	Index  int
	Type   types.Type
}

func (e *UnsupportedTypeError) Error() string {
	what := "parameter"
	if e.Result {
		what = "result"
	}
	return fmt.Sprintf("exec: %s: %s %d has unsupported type %s", e.Func, what, e.Index, e.Type)
}

// ArgError reports an argument value that does not match its parameter type.
type ArgError struct {
	Set   int          // argument set index
	Index int          // parameter index (-1 when the count is wrong)
	Want  types.Type   // parameter type (nil when the count is wrong)
	Got   reflect.Type // nil for an untyped nil or a missing value
	Msg   string
}

func (e *ArgError) Error() string {
	if e.Index < 0 {
		return fmt.Sprintf("exec: argument set %d: %s", e.Set, e.Msg)
	}
	return fmt.Sprintf("exec: argument set %d, parameter %d: want %s, got %v", e.Set, e.Index, e.Want, e.Got)
}

// Run interprets fn once per argument set in args. Each args[i] holds one Go
// value per parameter. Argument sets run in order in one interpreter, so
// package-level state persists between them exactly as it would across
// successive native calls.
//
// Run returns an error, before running anything, if a parameter or result
// type is unsupported (*UnsupportedTypeError) or an argument does not match
// its parameter (*ArgError).
func Run(fn *ssa.Function, args [][]any, opts Options) (Result, error) {
	if fn == nil || fn.Blocks == nil {
		return Result{}, errors.New("exec: function has no SSA body")
	}
	sig := fn.Signature
	if err := checkSignature(fn); err != nil {
		return Result{}, err
	}
	for set, a := range args {
		if len(a) != sig.Params().Len() {
			return Result{}, &ArgError{Set: set, Index: -1,
				Msg: fmt.Sprintf("%d values for %d parameters", len(a), sig.Params().Len())}
		}
		for i, v := range a {
			if err := checkArg(sig.Params().At(i).Type(), v); err != nil {
				err.Set, err.Index = set, i
				return Result{}, err
			}
		}
	}

	budget := opts.MaxSteps
	if budget == 0 {
		budget = DefaultMaxSteps
	}
	cfg := interpreter.DefaultConfig()
	cfg.MaxSteps = budget
	cfg.TrackInit = opts.TrackInit
	cfg.RandomSeed = opts.Seed
	var tr *tracer
	sopts := interpreter.SessionOptions{
		Config:          cfg,
		SingleGoroutine: opts.SingleGoroutine,
		SkipDetectors:   opts.SkipDetectors,
	}
	if opts.Cache != nil {
		sopts.Cache = opts.Cache
	}
	if opts.Trace {
		tr = &tracer{fn: fn, cap: opts.TraceCap}
		if tr.cap <= 0 {
			tr.cap = DefaultTraceCap
		}
		sopts.Tracer = tr
	}
	sess, err := interpreter.NewSession(fn, sopts)
	if err != nil {
		return Result{}, err
	}

	var res Result
	var classes interpreter.ClassCounts
	remaining := budget
	for set, a := range args {
		if remaining == 0 {
			if tr != nil {
				res.Traces = append(res.Traces, Traces{})
			}
			res.Outputs = append(res.Outputs, nil)
			res.ArgsAfter = append(res.ArgsAfter, nil)
			res.Outcomes = append(res.Outcomes, Exhausted)
			res.StepsPerSet = append(res.StepsPerSet, 0)
			res.Exhausted = true
			break
		}
		vals := make([]Value, len(a))
		for i, v := range a {
			vals[i] = marshal(sess, sig.Params().At(i).Type(), v)
		}
		// marshal copies slice arguments into interpreter memory, so vals
		// keeps pointing at them after the call: read back from there.
		if tr != nil {
			tr.traces = make(Traces)
			res.Traces = append(res.Traces, tr.traces)
		}
		out := sess.Call(vals, remaining)
		remaining -= min(out.Steps, remaining)

		res.Steps += out.Steps
		res.CacheHits += out.CacheHits
		res.PureExecuted += out.PureExecuted
		for c := range classes {
			classes[c] += out.Classes[c]
		}
		res.StepsPerSet = append(res.StepsPerSet, out.Steps)
		for _, err := range out.Violations {
			res.Violations = append(res.Violations, Violation{
				Set: set, Category: category(err), Message: err.Error(), Err: err,
			})
		}

		outcome, outputs := Returned, []any(nil)
		switch {
		case out.Exhausted:
			outcome = Exhausted
		case out.Panicked:
			outcome = Panicked
		default:
			outputs = make([]any, len(out.Results))
			for i, r := range out.Results {
				v, ok := unmarshal(sess, sig.Results().At(i).Type(), r)
				if !ok {
					outcome, outputs = Opaque, nil
					break
				}
				outputs[i] = v
			}
		}
		var after []any
		if outcome == Returned {
			var ok bool
			if after, ok = argsAfter(sess, sig, vals); !ok {
				outcome, outputs, after = Opaque, nil, nil
			}
		}
		res.Outputs = append(res.Outputs, outputs)
		res.ArgsAfter = append(res.ArgsAfter, after)
		res.Outcomes = append(res.Outcomes, outcome)
		if outcome == Exhausted {
			res.Exhausted = true
			break
		}
	}
	res.StepsByClass = make(map[Class]uint64, NumClasses)
	for c := Class(0); c < NumClasses; c++ {
		res.StepsByClass[c] = classes[c]
	}
	return res, nil
}

// category returns the report category of a giri error.
func category(err error) string {
	var vws *interpreter.ViolationWithStack
	if errors.As(err, &vws) {
		err = vws.Err
	}
	return shadow.CategoryOf(err)
}

// argsAfter reads each slice argument back from interpreter memory after a
// call. ok is false if a slice now holds a value with no concrete
// representation (the set is then Opaque, like an unrepresentable result).
func argsAfter(s *interpreter.Session, sig *types.Signature, vals []Value) ([]any, bool) {
	after := make([]any, len(vals))
	for i, v := range vals {
		t := sig.Params().At(i).Type()
		if _, isSlice := t.Underlying().(*types.Slice); !isSlice {
			continue
		}
		got, ok := unmarshal(s, t, v)
		if !ok {
			return nil, false
		}
		if rv := reflect.ValueOf(got); rv.Kind() == reflect.Slice && rv.IsNil() {
			got = nil // a nil slice argument stays nil
		}
		after[i] = got
	}
	return after, true
}

// --- Marshaling ---

// basicKinds maps supported basic types to their Go reflect kind and type.
var basicKinds = map[types.BasicKind]reflect.Type{
	types.Bool:    reflect.TypeFor[bool](),
	types.Int:     reflect.TypeFor[int](),
	types.Int8:    reflect.TypeFor[int8](),
	types.Int16:   reflect.TypeFor[int16](),
	types.Int32:   reflect.TypeFor[int32](),
	types.Int64:   reflect.TypeFor[int64](),
	types.Uint:    reflect.TypeFor[uint](),
	types.Uint8:   reflect.TypeFor[uint8](),
	types.Uint16:  reflect.TypeFor[uint16](),
	types.Uint32:  reflect.TypeFor[uint32](),
	types.Uint64:  reflect.TypeFor[uint64](),
	types.Uintptr: reflect.TypeFor[uintptr](),
	types.Float32: reflect.TypeFor[float32](),
	types.Float64: reflect.TypeFor[float64](),
	types.String:  reflect.TypeFor[string](),
}

// goType returns the Go type a supported SSA type marshals from and to.
func goType(t types.Type) (reflect.Type, bool) {
	switch u := t.Underlying().(type) {
	case *types.Basic:
		rt, ok := basicKinds[u.Kind()]
		return rt, ok
	case *types.Slice:
		if eb, ok := u.Elem().Underlying().(*types.Basic); ok {
			if rt, ok := basicKinds[eb.Kind()]; ok {
				return reflect.SliceOf(rt), true
			}
		}
	}
	return nil, false
}

func checkSignature(fn *ssa.Function) error {
	sig := fn.Signature
	for i := 0; i < sig.Params().Len(); i++ {
		if _, ok := goType(sig.Params().At(i).Type()); !ok {
			return &UnsupportedTypeError{Func: fn.String(), Index: i, Type: sig.Params().At(i).Type()}
		}
	}
	for i := 0; i < sig.Results().Len(); i++ {
		if _, ok := goType(sig.Results().At(i).Type()); !ok {
			return &UnsupportedTypeError{Func: fn.String(), Result: true, Index: i, Type: sig.Results().At(i).Type()}
		}
	}
	return nil
}

// checkArg verifies that v is acceptable for parameter type t: its Go kind
// must match t's underlying type exactly (named Go types are accepted by
// kind), and only slice parameters accept nil.
func checkArg(t types.Type, v any) *ArgError {
	want, _ := goType(t)
	if v == nil {
		if want.Kind() == reflect.Slice {
			return nil
		}
		return &ArgError{Want: t}
	}
	got := reflect.TypeOf(v)
	if got.Kind() != want.Kind() {
		return &ArgError{Want: t, Got: got}
	}
	if want.Kind() == reflect.Slice && got.Elem().Kind() != want.Elem().Kind() {
		return &ArgError{Want: t, Got: got}
	}
	return nil
}

// scalarValue converts a checked Go scalar to an interpreter value.
func scalarValue(rv reflect.Value) Value {
	switch rv.Kind() {
	case reflect.Bool:
		return Value{Raw: rv.Bool()}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return Value{Raw: rv.Int()}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return Value{Raw: int64(rv.Uint())}
	case reflect.Float32, reflect.Float64:
		return Value{Raw: rv.Float()}
	case reflect.String:
		return Value{Raw: rv.String()}
	}
	return Value{}
}

// marshal converts a checked Go value to an interpreter value for type t.
func marshal(s *interpreter.Session, t types.Type, v any) Value {
	st, isSlice := t.Underlying().(*types.Slice)
	if !isSlice {
		return scalarValue(reflect.ValueOf(v))
	}
	if v == nil {
		return Value{Raw: &interpreter.SliceValue{}}
	}
	rv := reflect.ValueOf(v)
	if rv.IsNil() {
		return Value{Raw: &interpreter.SliceValue{}}
	}
	elems := make([]Value, rv.Len())
	for i := range elems {
		elems[i] = scalarValue(rv.Index(i))
	}
	return s.MakeSlice(st.Elem(), elems)
}

// setScalar stores interpreter value v into the settable Go value dst.
func setScalar(dst reflect.Value, v Value) bool {
	switch dst.Kind() {
	case reflect.Bool:
		b, ok := v.Raw.(bool)
		dst.SetBool(b)
		return ok
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		n, ok := v.Raw.(int64)
		dst.SetInt(n)
		return ok
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		n, ok := v.Raw.(int64)
		dst.SetUint(uint64(n))
		return ok
	case reflect.Float32, reflect.Float64:
		f, ok := v.Raw.(float64)
		dst.SetFloat(f)
		return ok
	case reflect.String:
		s, ok := v.Raw.(string)
		dst.SetString(s)
		return ok
	}
	return false
}

// unmarshal converts an interpreter result of type t to a Go value.
// ok=false means the value is not concretely representable.
func unmarshal(s *interpreter.Session, t types.Type, v Value) (any, bool) {
	rt, _ := goType(t)
	if rt.Kind() != reflect.Slice {
		dst := reflect.New(rt).Elem()
		ok := setScalar(dst, v)
		return dst.Interface(), ok
	}
	elemType := t.Underlying().(*types.Slice).Elem()
	elems, isNil, ok := s.SliceElems(v, elemType)
	if !ok {
		return nil, false
	}
	if isNil {
		return reflect.Zero(rt).Interface(), true
	}
	out := reflect.MakeSlice(rt, len(elems), len(elems))
	for i, e := range elems {
		if !setScalar(out.Index(i), e) {
			return nil, false
		}
	}
	return out.Interface(), true
}
