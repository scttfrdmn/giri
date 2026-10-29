// Package interpreter - session.go is the library call mode used by pkg/exec.
//
// A Session interprets one function repeatedly with caller-constructed
// arguments, in concrete (value-faithful) mode, and instruments every executed
// instruction: per-class step counters, an optional value-cache hook consulted
// for pure instructions, an optional per-value tracer, and a step budget whose
// exhaustion halts the call without recording a violation.
//
// Sessions never touch the CLI paths (Run, RunN, RunTests).
package interpreter

import (
	"fmt"
	"go/token"
	"go/types"
	"unsafe"

	"golang.org/x/tools/go/ssa"

	"github.com/scttfrdmn/giri/pkg/shadow"
)

// InstrClass is the cost class of an executed SSA instruction.
type InstrClass uint8

// Instruction classes. The set is append-only: new classes are added before
// NumClasses, existing values never change.
const (
	ClassArithmetic      InstrClass = iota // BinOp (non-comparison), UnOp -, ^, !
	ClassCompare                           // BinOp ==, !=, <, <=, >, >=
	ClassLoad                              // UnOp * (dereference), map Lookup
	ClassStore                             // Store, MapUpdate
	ClassIndex                             // IndexAddr, FieldAddr, Index, Field, Slice
	ClassCallIntercepted                   // call modeled by giri (stdlib intercept, external)
	ClassCallInterpreted                   // call into an interpreted SSA function body
	ClassBuiltin                           // call to a builtin (len, append, copy, min, …)
	ClassBranch                            // If, Jump
	ClassPhi                               // Phi
	ClassAlloc                             // Alloc, MakeSlice, MakeMap, MakeChan, MakeClosure, MakeInterface
	ClassOther                             // everything else (Convert, Extract, Return, …)
	NumClasses
)

var classNames = [NumClasses]string{
	"arithmetic", "compare", "load", "store", "index", "call-intercepted",
	"call-interpreted", "builtin", "branch", "phi", "alloc", "other",
}

// String returns the class's stable lowercase name (e.g. "call-interpreted").
func (c InstrClass) String() string {
	if c < NumClasses {
		return classNames[c]
	}
	return fmt.Sprintf("class(%d)", uint8(c))
}

// ClassCounts holds step counts indexed by InstrClass.
type ClassCounts [NumClasses]uint64

// Tracer receives the value produced by every value-producing instruction a
// Session executes (including instructions in callees; filter by Parent()).
type Tracer interface {
	Record(v ssa.Value, val Value)
}

// ValueCache is consulted before executing an instruction classified pure by
// IsPure. On a hit the instruction is not executed: result is installed as
// its value and steps are added to the counters (attributed to the
// instruction's class). Store is called after a pure instruction executes
// without recording a violation or halting; steps is always 1.
type ValueCache interface {
	Lookup(v ssa.Value, operands []Value) (result Value, steps uint64, ok bool)
	Store(v ssa.Value, operands []Value, result Value, steps uint64)
}

// SessionOptions configures a Session.
type SessionOptions struct {
	// Config supplies detector flags and RandomSeed (math/rand intercepts).
	// MaxSteps bounds package initialization only; per-call budgets are
	// passed to Session.Call. ScheduleStrategy and MaxGoroutines apply to
	// goroutines spawned by the called function.
	Config Config

	// SkipDetectors disables shadow-memory checks and every detector. Only
	// allocation identity is kept (concrete mode needs it to key element
	// values). Built-in panics (index out of range, nil dereference, divide by
	// zero) are still raised.
	SkipDetectors bool

	// SingleGoroutine forbids goroutine creation (a go statement records a
	// "goroutine limit reached" violation) and removes the scheduler.
	SingleGoroutine bool

	// Cache, if non-nil, is the value-cache hook.
	Cache ValueCache

	// Tracer, if non-nil, receives every produced value.
	Tracer Tracer
}

// callState is the per-session instrumentation attached to an Interpreter.
type callState struct {
	classes      ClassCounts
	limit        uint64 // absolute step count at which to halt; 0 = none
	exhausted    bool
	cache        ValueCache
	tracer       Tracer
	pure         map[ssa.Instruction]bool // IsPure memo, used with cache
	cacheHits    uint64
	pureExecuted uint64
	sites        map[token.Pos]string
	nextAlloc    uint64                  // SkipDetectors allocation IDs
	layouts      map[types.Type][]leaf   // array/struct leaf layouts
	encoded      map[shadow.AllocID]bool // allocations converted to uintptr
	hb           map[int64]uint64        // clocks of finished calls (happens-before)
	single       bool
}

// Session interprets a single function in concrete mode.
type Session struct {
	interp *Interpreter
	fn     *ssa.Function
	opts   SessionOptions
}

// CallOutcome is the result of one Session.Call.
type CallOutcome struct {
	// Results holds the function's return values, one per result. It is nil
	// unless the call returned normally.
	Results []Value
	// Panicked is set when the call ended in an unrecovered panic, including
	// halts raised by giri (out-of-range index, nil dereference, ...).
	Panicked bool
	// Exhausted is set when the step budget ran out before the call returned.
	Exhausted bool
	// Steps and Classes count instructions executed (or supplied by cache
	// hits) during this call.
	Steps   uint64
	Classes ClassCounts
	// Violations recorded during this call, in detection order.
	Violations []error
	// CacheHits and PureExecuted count pure instructions satisfied by the
	// cache and pure instructions actually executed (only with a Cache).
	CacheHits    uint64
	PureExecuted uint64
}

// NewSession prepares fn for repeated concrete interpretation. The function's
// package initializer runs once here (its steps are not counted); package-level
// variables are allocated on first use and persist across calls, as they would
// across successive native calls.
func NewSession(fn *ssa.Function, opts SessionOptions) (*Session, error) {
	if fn == nil || fn.Blocks == nil {
		return nil, fmt.Errorf("interpreter: function has no SSA body")
	}
	cfg := opts.Config
	if opts.SkipDetectors {
		cfg.TrackUnsafe, cfg.TrackArenas, cfg.TrackRaces, cfg.TrackInit = false, false, false, false
		cfg.TrackTruncation, cfg.TrackDoubleClose = false, false
	}
	if opts.SingleGoroutine {
		cfg.MaxGoroutines = 1
	}
	interp := New(fn.Prog.Fset, cfg)
	interp.prog = fn.Prog
	interp.concrete = true
	interp.elems = make(map[elemKey]Value)
	interp.names = make(map[unsafe.Pointer]string)
	interp.globals = make(map[*ssa.Global]Value)
	interp.cs = &callState{
		cache:   opts.Cache,
		tracer:  opts.Tracer,
		sites:   make(map[token.Pos]string),
		layouts: make(map[types.Type][]leaf),
		encoded: make(map[shadow.AllocID]bool),
		hb:      make(map[int64]uint64),
		single:  opts.SingleGoroutine,
	}
	if opts.Cache != nil {
		interp.cs.pure = make(map[ssa.Instruction]bool)
	}
	if opts.SkipDetectors {
		interp.skipChecks = true
		interp.registry = nil
	}
	if opts.SingleGoroutine {
		interp.sched = nil
	}
	s := &Session{interp: interp, fn: fn, opts: opts}

	if fn.Pkg != nil {
		if initFn := fn.Pkg.Func("init"); initFn != nil {
			// Initialization is neither traced nor cached nor counted.
			cs := interp.cs
			cache, tracer := cs.cache, cs.tracer
			cs.cache, cs.tracer = nil, nil
			out := s.call(initFn, nil, cfg.MaxSteps)
			cs.cache, cs.tracer = cache, tracer
			interp.steps, cs.classes, cs.cacheHits, cs.pureExecuted = 0, ClassCounts{}, 0, 0
			interp.violations = nil
			if out.Exhausted {
				return nil, fmt.Errorf("interpreter: package %s initialization exceeded %d steps", fn.Pkg.Pkg.Path(), cfg.MaxSteps)
			}
			if out.Panicked {
				return nil, fmt.Errorf("interpreter: package %s initialization panicked", fn.Pkg.Pkg.Path())
			}
		}
	}
	return s, nil
}

// Call interprets the session's function once with args. budget bounds the
// steps this call may execute (0 = unlimited).
func (s *Session) Call(args []Value, budget uint64) CallOutcome {
	return s.call(s.fn, args, budget)
}

func (s *Session) call(fn *ssa.Function, args []Value, budget uint64) CallOutcome {
	interp, cs := s.interp, s.interp.cs
	steps0, classes0 := interp.steps, cs.classes
	hits0, pure0 := cs.cacheHits, cs.pureExecuted
	nviol := len(interp.violations)
	cs.exhausted = false
	cs.limit = 0
	if budget > 0 {
		cs.limit = steps0 + budget
	}

	out := CallOutcome{}
	gid, err := interp.spawnGoroutine(fn.String(), "entry")
	if err != nil {
		out.Panicked = true
		out.Violations = []error{err}
		return out
	}
	// Successive calls (and package init before them) are sequential, as
	// successive native calls are: each call happens after everything the
	// previous ones did.
	g := interp.goroutines[gid]
	mergeClock(g, cs.hb)
	ret := interp.execFunction(gid, fn, args)
	for id, t := range g.VClock.Clocks {
		if t > cs.hb[id] {
			cs.hb[id] = t
		}
	}
	halted := g.Panicked || g.Panicking
	delete(interp.goroutines, gid)
	if !cs.single {
		interp.drainSessionQueue()
	}

	out.Exhausted = cs.exhausted
	out.Panicked = halted && !cs.exhausted
	if !halted {
		out.Results = splitResults(ret, fn.Signature.Results().Len())
	}
	out.Steps = interp.steps - steps0
	for i := range out.Classes {
		out.Classes[i] = cs.classes[i] - classes0[i]
	}
	out.CacheHits = cs.cacheHits - hits0
	out.PureExecuted = cs.pureExecuted - pure0
	if len(interp.violations) > nviol {
		out.Violations = append([]error(nil), interp.violations[nviol:]...)
	}
	return out
}

// drainSessionQueue runs goroutines spawned during a call to completion via the
// scheduler (mirrors the drain loop in Run), then forgets them.
func (interp *Interpreter) drainSessionQueue() {
	for len(interp.runQueue) > 0 {
		runnable := make([]int64, len(interp.runQueue))
		for i, t := range interp.runQueue {
			runnable[i] = t.gid
		}
		next := interp.sched.Next(runnable)
		for i, t := range interp.runQueue {
			if t.gid != next {
				continue
			}
			last := len(interp.runQueue) - 1
			interp.runQueue[i] = interp.runQueue[last]
			interp.runQueue = interp.runQueue[:last]
			interp.execFunction(t.gid, t.fn, t.args)
			interp.sched.OnFinish(t.gid)
			break
		}
	}
	for id, g := range interp.goroutines {
		if g.Status != GoroutineBlocked {
			delete(interp.goroutines, id)
		}
	}
}

// splitResults turns execFunction's return value into one Value per result.
func splitResults(ret Value, n int) []Value {
	switch n {
	case 0:
		return []Value{}
	case 1:
		return []Value{ret}
	}
	if tuple, ok := ret.Raw.([]Value); ok && len(tuple) == n {
		return tuple
	}
	return make([]Value, n)
}

// MakeSlice allocates a slice with element type elemType holding elems, for
// passing as an argument.
func (s *Session) MakeSlice(elemType types.Type, elems []Value) Value {
	return s.interp.newConcreteSlice(elemType, elems, len(elems), "<argument>")
}

// SliceElems reads the elements of a slice value. isNil reports a nil slice;
// ok=false means v is not a slice the interpreter can read.
func (s *Session) SliceElems(v Value, elemType types.Type) (elems []Value, isNil, ok bool) {
	return s.interp.sliceElems(v, elemType)
}

// --- Instrumented instruction prologue ---

// execInstructionSession is execInstruction for library call mode.
func (interp *Interpreter) execInstructionSession(gid int64, fn *ssa.Function, instr ssa.Instruction) *ssa.BasicBlock {
	cs := interp.cs
	if cs.limit > 0 && interp.steps >= cs.limit {
		cs.exhausted = true
		if g := interp.goroutines[gid]; g != nil {
			g.Panicked = true
		}
		return nil
	}

	frame := interp.currentFrame(gid)
	class := interp.classify(frame, instr)
	site, ok := cs.sites[instr.Pos()]
	if !ok {
		site = interp.posString(instr.Pos())
		cs.sites[instr.Pos()] = site
	}
	interp.currentSite = site

	var (
		cacheable bool
		value     ssa.Value
		operands  []Value
	)
	if cs.cache != nil && cs.isPure(instr) {
		value = instr.(ssa.Value)
		operands = interp.pureOperands(frame, instr)
		cacheable = allScalarValues(operands)
	}
	if cacheable {
		if res, steps, hit := cs.cache.Lookup(value, operands); hit {
			if _, isCall := instr.(*ssa.Call); isCall {
				// The only effect of a pure intercept call is the GC
				// safepoint execCall would have checked; a hit keeps it.
				interp.gcSafepoint(gid, site)
			}
			interp.steps += steps
			cs.classes[class] += steps
			cs.cacheHits++
			frame.Locals[interp.vname(value)] = res
			if cs.tracer != nil {
				cs.tracer.Record(value, res)
			}
			return nil
		}
	}

	interp.steps++
	cs.classes[class]++
	nviol := len(interp.violations)
	next := interp.execOp(gid, fn, instr, site)

	if cacheable {
		cs.pureExecuted++
		g := interp.goroutines[gid]
		if len(interp.violations) == nviol && g != nil && !g.Panicked && !g.Panicking {
			if res, ok := frame.Locals[interp.vname(value)]; ok && isScalarValue(res) {
				cs.cache.Store(value, operands, res, 1)
			}
		}
	}
	if cs.tracer != nil {
		if v, ok := instr.(ssa.Value); ok {
			if res, ok := frame.Locals[interp.vname(v)]; ok {
				cs.tracer.Record(v, res)
			}
		}
	}
	return next
}

// allScalarValues reports whether every operand is a scalar at run time.
// IsPure decides from static types; this guards the hook against values whose
// representation is not a scalar despite their type (#255).
func allScalarValues(vs []Value) bool {
	for _, v := range vs {
		if !isScalarValue(v) {
			return false
		}
	}
	return true
}

// isPure memoizes IsPure per instruction.
func (cs *callState) isPure(instr ssa.Instruction) bool {
	p, ok := cs.pure[instr]
	if !ok {
		p = IsPure(instr)
		cs.pure[instr] = p
	}
	return p
}

// pureOperands resolves the operand values a pure instruction's result
// depends on, in a fixed order: BinOp (X, Y); UnOp, Convert, ChangeType (X);
// Call (arguments).
func (interp *Interpreter) pureOperands(frame *Frame, instr ssa.Instruction) []Value {
	switch i := instr.(type) {
	case *ssa.BinOp:
		return []Value{interp.resolveValue(frame, i.X), interp.resolveValue(frame, i.Y)}
	case *ssa.UnOp:
		return []Value{interp.resolveValue(frame, i.X)}
	case *ssa.Convert:
		return []Value{interp.resolveValue(frame, i.X)}
	case *ssa.ChangeType:
		return []Value{interp.resolveValue(frame, i.X)}
	case *ssa.Call:
		ops := make([]Value, len(i.Call.Args))
		for k, a := range i.Call.Args {
			ops[k] = interp.resolveValue(frame, a)
		}
		return ops
	}
	return nil
}

// IsPure reports whether instr is pure for value-cache purposes: its result
// is a function of its operand values alone, it reads and writes no memory
// (shadow or otherwise), it performs no channel or goroutine operation, and
// its operands and result are all basic scalar types (booleans, numbers,
// strings; not unsafe.Pointer). The pure instructions are:
//
//   - BinOp
//   - UnOp other than * (load) and <- (receive)
//   - Convert and ChangeType between basic types
//   - Call of a concrete stdlib intercept marked pure (see IsConcreteIntercept),
//     e.g. math.Sqrt or math/bits.OnesCount64
//
// Instructions that can raise a run-time panic for some operands (integer
// division, shifts) are still pure; the interpreter never stores a cache entry
// for an execution that recorded a violation or halted, so a cache hit can
// never mask one.
func IsPure(instr ssa.Instruction) bool {
	v, ok := instr.(ssa.Value)
	if !ok || !isScalarType(v.Type()) {
		return false
	}
	switch i := instr.(type) {
	case *ssa.BinOp:
		return isScalarType(i.X.Type()) && isScalarType(i.Y.Type())
	case *ssa.UnOp:
		if i.Op == token.MUL || i.Op == token.ARROW {
			return false
		}
		return isScalarType(i.X.Type())
	case *ssa.Convert:
		return isScalarType(i.X.Type())
	case *ssa.ChangeType:
		return isScalarType(i.X.Type())
	case *ssa.Call:
		if i.Call.IsInvoke() {
			return false
		}
		callee := i.Call.StaticCallee()
		if callee == nil {
			return false
		}
		impl := concreteCallee(callee)
		if impl == nil || !impl.pure {
			return false
		}
		for _, a := range i.Call.Args {
			if !isScalarType(a.Type()) {
				return false
			}
		}
		return true
	}
	return false
}

// isScalarType reports whether t is a basic boolean, numeric or string type.
func isScalarType(t types.Type) bool {
	b, ok := t.Underlying().(*types.Basic)
	if !ok {
		return false
	}
	return b.Info()&(types.IsBoolean|types.IsNumeric|types.IsString) != 0
}

// concreteCallee returns the concrete implementation of a static callee, or
// nil. The table is keyed by package-level function name, so methods never
// match: strings.(*Replacer).Replace is not strings.Replace (#257).
func concreteCallee(callee *ssa.Function) *concreteImpl {
	if callee == nil || callee.Signature.Recv() != nil {
		return nil
	}
	pkg, name := calleePkgName(callee)
	if pkg == "" {
		return nil
	}
	return lookupConcrete(pkg, name)
}

// calleePkgName returns the package path and (generic-stripped) name of a
// static callee, resolving generic instantiations through Origin.
func calleePkgName(callee *ssa.Function) (string, string) {
	pkg, name := callee.Package(), callee.Name()
	if pkg == nil {
		if origin := callee.Origin(); origin != nil {
			pkg, name = origin.Package(), origin.Name()
		}
	}
	if pkg == nil || pkg.Pkg == nil {
		if obj, ok := callee.Object().(*types.Func); ok && obj.Pkg() != nil {
			return obj.Pkg().Path(), genericBaseName(obj.Name())
		}
		return "", genericBaseName(name)
	}
	return pkg.Pkg.Path(), genericBaseName(name)
}

// ClassOf returns the cost class of instr. Calls are classified from their
// static callee: builtins, intercepted (stdlib, external, or dynamic) and
// interpreted (functions with an SSA body outside the standard library). At
// run time a Session refines dynamic calls by the value actually called.
func ClassOf(instr ssa.Instruction) InstrClass {
	return (*Interpreter)(nil).classify(nil, instr)
}

// classify returns instr's cost class; frame (may be nil) refines dynamic calls.
func (interp *Interpreter) classify(frame *Frame, instr ssa.Instruction) InstrClass {
	switch i := instr.(type) {
	case *ssa.BinOp:
		switch i.Op {
		case token.EQL, token.NEQ, token.LSS, token.LEQ, token.GTR, token.GEQ:
			return ClassCompare
		}
		return ClassArithmetic
	case *ssa.UnOp:
		switch i.Op {
		case token.MUL:
			return ClassLoad
		case token.ARROW:
			return ClassOther
		}
		return ClassArithmetic
	case *ssa.Lookup:
		return ClassLoad
	case *ssa.Store, *ssa.MapUpdate:
		return ClassStore
	case *ssa.IndexAddr, *ssa.FieldAddr, *ssa.Index, *ssa.Field, *ssa.Slice:
		return ClassIndex
	case *ssa.Call:
		return interp.callClass(frame, &i.Call)
	case *ssa.If, *ssa.Jump:
		return ClassBranch
	case *ssa.Phi:
		return ClassPhi
	case *ssa.Alloc, *ssa.MakeSlice, *ssa.MakeMap, *ssa.MakeChan, *ssa.MakeClosure, *ssa.MakeInterface:
		return ClassAlloc
	}
	return ClassOther
}

// callClass mirrors execCall's dispatch to decide whether a call is a builtin,
// intercepted, or interpreted.
func (interp *Interpreter) callClass(frame *Frame, call *ssa.CallCommon) InstrClass {
	if _, ok := call.Value.(*ssa.Builtin); ok {
		return ClassBuiltin
	}
	if call.IsInvoke() {
		if interp != nil && frame != nil && interp.prog != nil {
			if iv, ok := interp.resolveValue(frame, call.Value).Raw.(*InterfaceValue); ok && iv.Type != nil {
				if fn := interp.prog.LookupMethod(iv.Type, call.Method.Pkg(), call.Method.Name()); fn != nil && fn.Blocks != nil {
					return ClassCallInterpreted
				}
			}
		}
		return ClassCallIntercepted
	}
	callee := call.StaticCallee()
	if callee == nil {
		if interp != nil && frame != nil {
			switch f := interp.resolveValue(frame, call.Value).Raw.(type) {
			case *ClosureValue:
				return ClassCallInterpreted
			case *ssa.Function:
				if f.Blocks != nil {
					return ClassCallInterpreted
				}
			}
		}
		return ClassCallIntercepted
	}
	pkg, _ := calleePkgName(callee)
	if isStdlibOrKnownExternalPkg(pkg) || callee.Blocks == nil {
		return ClassCallIntercepted
	}
	return ClassCallInterpreted
}

// --- Concrete-mode helpers that need the session ---

// binOpConcrete evaluates a BinOp in concrete mode, raising Go's run-time
// panics for integer division by zero and negative signed shift counts.
func (interp *Interpreter) binOpConcrete(gid int64, inst *ssa.BinOp, x, y Value, site string) Value {
	xt, yt := inst.X.Type(), inst.Y.Type()
	switch inst.Op {
	case token.QUO, token.REM:
		if _, isInt := basicIntKind(xt); isInt {
			if yi, ok := y.Raw.(int64); ok && yi == 0 {
				interp.recordViolation(gid, &shadow.DivisionByZeroError{Site: site, GID: gid})
				interp.runtimePanic(gid, "integer divide by zero")
				return Value{Raw: int64(0)}
			}
		}
	case token.SHL, token.SHR:
		if k, ok := basicIntKind(yt); ok && !k.unsigned {
			if yi, ok := y.Raw.(int64); ok && yi < 0 {
				interp.recordViolation(gid, &shadow.NegativeShiftError{Count: yi, Site: site, GID: gid})
				interp.runtimePanic(gid, "negative shift amount")
				return Value{Raw: int64(0)}
			}
		}
	}
	if r, ok := concreteBinOp(inst.Op, x, y, xt, yt); ok {
		return r
	}
	if inst.Op == token.EQL || inst.Op == token.NEQ {
		if _, isIface := xt.Underlying().(*types.Interface); isIface {
			eq, ok := interp.valuesEqual(x, y, xt)
			if !ok {
				return stdlibOpaque
			}
			return Value{Raw: eq == (inst.Op == token.EQL)}
		}
		if isAggregateType(xt) {
			eq, ok := interp.aggregateEqual(x, y, xt)
			if !ok {
				return stdlibOpaque
			}
			return Value{Raw: eq == (inst.Op == token.EQL)}
		}
		if eq, ok := concreteEqual(x, y); ok {
			return Value{Raw: eq == (inst.Op == token.EQL)}
		}
	}
	return evalBinOp(inst.Op, x, y)
}

// halt stops goroutine gid fatally: no deferred calls run and nothing can
// recover it (a fault, the step budget). Recoverable run-time panics use
// runtimePanic.
func (interp *Interpreter) halt(gid int64) {
	if g := interp.goroutines[gid]; g != nil {
		g.Panicked = true
	}
}

// sliceStringConcrete evaluates s[low:high] (high < 0 means len(s)), raising
// an out-of-range panic for invalid bounds.
func (interp *Interpreter) sliceStringConcrete(gid int64, s string, low, high int, site string) Value {
	if high < 0 {
		high = len(s)
	}
	if low < 0 || high < low || high > len(s) {
		interp.recordViolation(gid, &shadow.OutOfBoundsError{
			AllocSize: len(s), Offset: high, AccessSize: high - low, Site: site, TypeName: "string",
		})
		interp.runtimePanic(gid, "index out of range")
		return Value{Raw: ""}
	}
	return Value{Raw: s[low:high]}
}

// sessionGlobal allocates a package-level variable on first use.
func (interp *Interpreter) sessionGlobal(g *ssa.Global) Value {
	size := interp.typeSizeOf(deref(g.Type()))
	if size <= 0 {
		size = 8
	}
	id := interp.allocate(shadow.AllocGlobal, size, g.Type().String(), g.Name())
	if !interp.skipChecks {
		interp.Memory.MarkInitialized(id, 0, size) // globals are zero-initialized
	}
	ptr := &shadow.Pointer{Alloc: id}
	v := Value{Raw: ptr, Provenance: ptr}
	interp.globals[g] = v
	return v
}

// derive returns a pointer offset from base. Without detector checks the
// derivation chain (DerivedFrom) is not needed, so a flat pointer is used.
func (interp *Interpreter) derive(base *shadow.Pointer, delta int) *shadow.Pointer {
	if interp.skipChecks {
		return &shadow.Pointer{Alloc: base.Alloc, Offset: base.Offset + delta}
	}
	return interp.Memory.DerivePointer(base, delta)
}
