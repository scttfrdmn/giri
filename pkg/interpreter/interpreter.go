// Package interpreter executes Go programs via SSA interpretation with
// shadow memory instrumentation. This is the core of Giri — every SSA
// instruction is interpreted and its memory effects are validated against
// the shadow memory system.
//
// Architecture:
//
//	Program SSA ──► Interpreter ──► Shadow Memory
//	                    │                │
//	                    │           Detectors query
//	                    │           shadow state to
//	                    ▼           find violations
//	               Scheduler
//	          (goroutine interleaving)
//
// The interpreter maintains a call stack per goroutine, with each frame
// holding local variable bindings. Values in the interpreter are either
// concrete Go values (for computation) or tracked pointers (for memory
// safety verification).
package interpreter

import (
	"fmt"
	"go/token"
	"go/types"
	"runtime"
	"strings"
	"sync/atomic"

	"golang.org/x/tools/go/ssa"

	"github.com/scttfrdmn/giri/pkg/detector"
	"github.com/scttfrdmn/giri/pkg/scheduler"
	"github.com/scttfrdmn/giri/pkg/shadow"
	"github.com/scttfrdmn/safearena"
)

// Value represents an interpreted value. It wraps a concrete Go value
// and optionally carries pointer provenance metadata.
type Value struct {
	// Raw is the concrete value for computation.
	// For scalars: int64, float64, bool, string
	// For pointers: a *shadow.Pointer (provenance metadata)
	// For structs: map[string]Value
	// For slices: *SliceValue
	// For interfaces: *InterfaceValue
	Raw interface{}

	// Provenance is non-nil if this value is or contains a pointer.
	// This is how Giri tracks which allocation a value derives from.
	Provenance *shadow.Pointer
}

// SliceValue represents an interpreted slice with bounds tracking.
type SliceValue struct {
	Backing *shadow.Pointer // Points to the backing array allocation
	Len     int
	Cap     int
}

// InterfaceValue represents a boxed interface value.
type InterfaceValue struct {
	Type  types.Type
	Value Value
}

// StackFrame represents a single frame captured from a goroutine's call stack.
// Frames are ordered innermost-first in ViolationWithStack.Frames.
type StackFrame struct {
	FuncName string
	Site     string // "file:line"
}

// ViolationWithStack wraps a violation error with the goroutine's call stack
// at the point of detection. It implements Unwrap() so existing type switches
// in classifyError still work after unwrapping.
type ViolationWithStack struct {
	Err       error
	GID       int64
	SpawnSite string        // where this goroutine was created
	Frames    []StackFrame  // call stack, innermost first
}

// Error implements the error interface. The message is the underlying error's.
func (v *ViolationWithStack) Error() string { return v.Err.Error() }

// Unwrap returns the underlying violation for errors.As / type-switch chains.
func (v *ViolationWithStack) Unwrap() error { return v.Err }

// StackTrace renders the captured call stack as a formatted string.
func (v *ViolationWithStack) StackTrace() string {
	var b strings.Builder
	spawnSite := v.SpawnSite
	if spawnSite == "" {
		spawnSite = "entry"
	}
	fmt.Fprintf(&b, "goroutine %d (spawned at %s):\n", v.GID, spawnSite)
	for _, f := range v.Frames {
		fmt.Fprintf(&b, "  %s\n    %s\n", f.FuncName, f.Site)
	}
	return b.String()
}

// GoroutineID returns the goroutine ID that recorded this violation.
func (v *ViolationWithStack) GoroutineID() int64 { return v.GID }

// ClosureValue represents a closure: a function together with its captured
// free variables. Created by ssa.MakeClosure; called by execCall/ssa.Go.
type ClosureValue struct {
	Fn       *ssa.Function
	FreeVars []Value
}

// Frame represents a single stack frame in the interpreter.
type Frame struct {
	// Function being executed
	FuncName string
	Site     string // "file:line"

	// Local variable bindings: SSA value name → interpreted value
	Locals map[string]Value

	// Deferred function calls (executed on return)
	Defers []DeferredCall

	// Caller frame (for stack traces)
	Caller *Frame

	// Previous basic block (for Phi node resolution)
	PrevBlock *ssa.BasicBlock

	// ReturnInst holds the ssa.Return instruction so popFrame can re-evaluate
	// named return values after executing deferred calls that may have modified
	// named-return allocs (#49). Nil for functions without named returns.
	ReturnInst *ssa.Return

	// StackAllocs holds AllocIDs for ssa.Alloc instructions with Heap=false
	// (stack-local allocations). These are poisoned in popFrame after deferred
	// calls and named-return recomputation have run (#51). Heap=false allocs
	// cannot legitimately escape their frame (Go's SSA escape analysis marks
	// any escaped alloc as Heap=true), so poisoning them is always safe.
	StackAllocs []shadow.AllocID
}

// DeferredCall represents a deferred function invocation.
type DeferredCall struct {
	// Callee is the resolved SSA function (nil for unresolved dynamic calls).
	Callee *ssa.Function
	// IsClosure is true when the call target is a closure value.
	IsClosure bool
	// ClosureVal holds the closure when IsClosure is true.
	ClosureVal *ClosureValue
	// PkgPath is the package path of the callee (e.g. "sync") for intercepted calls.
	PkgPath string
	// FuncName is the method/function name (e.g. "Unlock") for dispatching.
	FuncName string
	Args     []Value // Captured arguments
	Site     string  // Where defer was declared
}

// Goroutine represents a single goroutine's execution state.
type Goroutine struct {
	ID     int64
	Stack  []*Frame // Call stack, top of stack = last element
	Status GoroutineStatus

	// Panicked is set when this goroutine has been halted by a fatal condition
	// (violation-detected panic, step limit exceeded, or unrecovered ssa.Panic).
	// All subsequent instructions are skipped.
	Panicked bool

	// Panicking is set when an ssa.Panic is in-flight and defer unwinding is
	// in progress. Unlike Panicked, Panicking can be cleared by recover() (#48).
	Panicking bool

	// Recovered is set by recover() to signal to popFrame that the current
	// defer has caught the in-flight panic. Reset before each defer runs.
	Recovered bool

	// PanicValue holds the value passed to panic(), used by recover() (#34, #48).
	PanicValue Value

	// Vector clock for happens-before tracking
	VClock *VectorClock

	// SpawnSite is the source location of the ssa.Go instruction that created
	// this goroutine. Empty for the main goroutine.
	SpawnSite string

	// BlockSite is the source location where this goroutine blocked.
	// Only meaningful when Status == GoroutineBlocked.
	BlockSite string

	// BlockChanID is the channel this goroutine is blocked on.
	// Only meaningful when Status == GoroutineBlocked.
	BlockChanID ChanID

	// BlockOnSend is true when the goroutine blocked trying to send (vs. receive).
	// Used by checkGoroutineLeaks to distinguish send-blocked from recv-blocked.
	BlockOnSend bool
}

// GoroutineStatus tracks goroutine lifecycle.
type GoroutineStatus uint8

const (
	GoroutineRunning GoroutineStatus = iota
	GoroutineBlocked                 // Waiting on channel/mutex/etc.
	GoroutineFinished
)

// VectorClock implements Lamport vector clocks for happens-before tracking.
// Each goroutine maintains a vector of logical timestamps, one per goroutine.
type VectorClock struct {
	Clocks map[int64]uint64
}

// NewVectorClock creates a new vector clock for a goroutine.
func NewVectorClock(goroutineID int64) *VectorClock {
	vc := &VectorClock{Clocks: make(map[int64]uint64)}
	vc.Clocks[goroutineID] = 1
	return vc
}

// Tick increments this goroutine's logical clock.
func (vc *VectorClock) Tick(goroutineID int64) {
	vc.Clocks[goroutineID]++
}

// Merge takes the pointwise maximum of two vector clocks.
// Called when goroutines synchronize (channel send/recv, mutex lock/unlock).
func (vc *VectorClock) Merge(other *VectorClock) {
	for id, t := range other.Clocks {
		if t > vc.Clocks[id] {
			vc.Clocks[id] = t
		}
	}
}

// HappensBefore returns true if this clock happens-before the other.
func (vc *VectorClock) HappensBefore(other *VectorClock) bool {
	for id, t := range vc.Clocks {
		if t > other.Clocks[id] {
			return false
		}
	}
	return true
}

// ChanID is a unique identifier for an interpreted channel.
type ChanID uint64

// chanEntry holds synchronization state for a channel, used to propagate
// happens-before relationships from sender to receiver.
type chanEntry struct {
	lastSenderGID   int64
	lastSenderClock map[int64]uint64
	closed          bool  // set by close(ch) (#31)
	hasPending      bool  // a value has been sent but not yet received
	pendingVal      Value // the pending value (for select readiness checks)
	capacity     int  // buffered channel capacity (0 = unbuffered)
	pendingCount int  // number of buffered values currently held (#44)
}

// mutexState tracks synchronization state for sync.Mutex and sync.WaitGroup (#33).
type mutexState struct {
	locked          bool
	lockGoroutine   int64
	lastUnlockClock map[int64]uint64 // clock snapshot at last Unlock/Done
	wgCounter       int              // WaitGroup counter; negative → violation (#57)
}

// goroutineTask holds a pending goroutine execution queued by ssa.Go.
type goroutineTask struct {
	gid  int64
	fn   *ssa.Function
	args []Value
}

// Interpreter is the main SSA interpreter with shadow memory instrumentation.
type Interpreter struct {
	// Shadow memory system
	Memory *shadow.Memory

	// File set for resolving source positions
	Fset *token.FileSet

	// Active goroutines
	goroutines map[int64]*Goroutine
	nextGID    atomic.Int64

	// Collected violations
	violations []error

	// Configuration
	config Config

	// Type size calculator for the target platform
	sizes types.Sizes

	// Detector registry (runs all enabled detectors on each access)
	registry *detector.Registry

	// Goroutine scheduler
	sched scheduler.Scheduler

	// Pending goroutine tasks queued by ssa.Go
	runQueue []goroutineTask

	// Channel state for happens-before tracking
	channels   map[ChanID]*chanEntry
	nextChanID atomic.Uint64

	// channelSenders tracks channels that have had at least one send operation.
	// Used by checkGoroutineLeaks: a goroutine blocked on channel C is only
	// reported as leaked if no goroutine ever sent on C.
	channelSenders map[ChanID]bool

	// channelReceivers tracks channels that have had at least one receive.
	// Used by checkGoroutineLeaks: a goroutine blocked on a send is not a leak
	// if a receiver already consumed (or will consume) from that channel.
	channelReceivers map[ChanID]bool

	// Sync primitive state for sync.Mutex and sync.WaitGroup (#33).
	// Key is the AllocID of the mutex/waitgroup's shadow memory allocation.
	mutexes map[shadow.AllocID]*mutexState

	// Total SSA instructions executed (checked against Config.MaxSteps)
	steps uint64

	// Global variable state: maps each ssa.Global to its shadow-memory pointer.
	// Initialized in Run() by iterating all packages before main executes.
	globals map[*ssa.Global]Value

	// prog is the SSA program, used for interface method dispatch (LookupMethod).
	prog *ssa.Program

	// arena is a per-interpretation-run arena. Hot-path structs (Frame,
	// Goroutine, SliceValue, shadow.Pointer) are arena-allocated here to
	// reduce GC pressure during interpretation. Freed automatically by Run()
	// via safearena.Scoped when the run completes.
	//
	// nil in unit tests that construct Interpreter directly via New().
	arena *safearena.Arena

	// valueStore tracks the most recent value written through a pointer address.
	// Keyed by AllocID. Used by popFrame to re-evaluate named return values after
	// deferred closures may have modified named-return allocs (#49).
	valueStore map[shadow.AllocID]Value

	// suppressions maps "file:line" → true for each //giri:ignore comment in
	// the source. Violations whose currentSite matches an entry are dropped (#58).
	suppressions map[string]bool

	// currentSite is the posString of the SSA instruction currently executing.
	// Updated at the start of each instruction in execBlock; read by
	// recordViolation to check against suppressions.
	currentSite string
}

// Config controls interpreter behavior.
type Config struct {
	// MaxSteps limits total SSA instructions executed (0 = unlimited).
	// Prevents infinite loops from hanging the interpreter.
	MaxSteps uint64

	// MaxGoroutines limits concurrent goroutines (0 = unlimited).
	MaxGoroutines int

	// ScheduleStrategy controls goroutine interleaving.
	ScheduleStrategy ScheduleStrategy

	// RandomSeed for reproducible scheduling (if strategy is random or PCT).
	RandomSeed int64

	// BugDepth is the target bug depth for PCT scheduling.
	// Most real-world concurrency bugs have depth 1–2.
	BugDepth int

	// TrackUnsafe enables unsafe.Pointer rule checking.
	TrackUnsafe bool

	// TrackArenas enables arena lifecycle checking.
	TrackArenas bool

	// TrackRaces enables data race detection via vector clocks.
	TrackRaces bool

	// TrackInit enables uninitialized memory read detection.
	TrackInit bool

	// Verbose enables detailed execution tracing.
	Verbose bool
}

// ScheduleStrategy controls how the interpreter chooses which goroutine to run next.
type ScheduleStrategy uint8

const (
	// ScheduleRoundRobin runs goroutines in order (deterministic, fast).
	ScheduleRoundRobin ScheduleStrategy = iota
	// ScheduleRandom picks a random runnable goroutine (finds more bugs).
	ScheduleRandom
	// ScheduleAdversarial tries interleavings most likely to trigger races.
	ScheduleAdversarial
	// SchedulePCT uses probabilistic concurrency testing.
	SchedulePCT
)

// DefaultConfig returns a sensible default configuration.
func DefaultConfig() Config {
	return Config{
		MaxSteps:         10_000_000, // 10M instructions
		MaxGoroutines:    1000,
		ScheduleStrategy: ScheduleRoundRobin,
		BugDepth:         3, // Covers most real-world concurrency bugs
		TrackUnsafe:      true,
		TrackArenas:      true,
		TrackRaces:       true,
		TrackInit:        false, // Expensive, opt-in
	}
}

// New creates a new interpreter with the given configuration.
func New(fset *token.FileSet, config Config) *Interpreter {
	memOpts := []shadow.Option{}
	if config.Verbose {
		memOpts = append(memOpts, shadow.WithVerbose())
	}
	if config.TrackInit {
		memOpts = append(memOpts, shadow.WithInitTracking())
	}

	interp := &Interpreter{
		Memory:           shadow.NewMemory(memOpts...),
		Fset:             fset,
		goroutines:       make(map[int64]*Goroutine),
		channels:         make(map[ChanID]*chanEntry),
		channelSenders:   make(map[ChanID]bool),
		channelReceivers: make(map[ChanID]bool),
		mutexes:          make(map[shadow.AllocID]*mutexState),
		valueStore:       make(map[shadow.AllocID]Value),
		config:           config,
		sizes:            types.SizesFor("gc", runtime.GOARCH),
	}

	// Build detector registry from config flags
	var dets []detector.Detector
	if config.TrackArenas {
		dets = append(dets, &detector.ArenaDetector{})
	}
	if config.TrackArenas || config.TrackUnsafe {
		dets = append(dets, &detector.BoundsDetector{})
	}
	if config.TrackUnsafe {
		dets = append(dets, detector.NewUnsafeDetector())
	}
	if config.TrackRaces {
		dets = append(dets, detector.NewRaceDetector())
	}
	interp.registry = detector.NewRegistry(dets...)

	// Initialize scheduler
	switch config.ScheduleStrategy {
	case ScheduleRoundRobin:
		interp.sched = scheduler.NewRoundRobin()
	case ScheduleRandom:
		interp.sched = scheduler.NewRandom(config.RandomSeed)
	case SchedulePCT:
		interp.sched = scheduler.NewPCT(config.RandomSeed, config.BugDepth)
	default:
		interp.sched = scheduler.NewRoundRobin()
	}

	return interp
}

// newWithArena is like New but wires a per-run safearena.Arena into the
// interpreter and shadow memory system. All hot-path struct allocations
// (Frame, Goroutine, SliceValue, shadow.Pointer) are arena-backed for the
// duration of the run, which keeps them off the GC heap and eliminates
// per-object tracing overhead.
//
// Called by Run() via safearena.Scoped; the arena is freed automatically
// when Run() returns.
func newWithArena(fset *token.FileSet, config Config, a *safearena.Arena) *Interpreter {
	memOpts := []shadow.Option{shadow.WithPointerArena(a)}
	if config.Verbose {
		memOpts = append(memOpts, shadow.WithVerbose())
	}
	if config.TrackInit {
		memOpts = append(memOpts, shadow.WithInitTracking())
	}

	interp := &Interpreter{
		Memory:           shadow.NewMemory(memOpts...),
		Fset:             fset,
		goroutines:       make(map[int64]*Goroutine),
		channels:         make(map[ChanID]*chanEntry),
		channelSenders:   make(map[ChanID]bool),
		channelReceivers: make(map[ChanID]bool),
		mutexes:          make(map[shadow.AllocID]*mutexState),
		valueStore:       make(map[shadow.AllocID]Value),
		config:           config,
		sizes:            types.SizesFor("gc", runtime.GOARCH),
		arena:            a,
	}

	var dets []detector.Detector
	if config.TrackArenas {
		dets = append(dets, &detector.ArenaDetector{})
	}
	if config.TrackArenas || config.TrackUnsafe {
		dets = append(dets, &detector.BoundsDetector{})
	}
	if config.TrackUnsafe {
		dets = append(dets, detector.NewUnsafeDetector())
	}
	if config.TrackRaces {
		dets = append(dets, detector.NewRaceDetector())
	}
	interp.registry = detector.NewRegistry(dets...)

	switch config.ScheduleStrategy {
	case ScheduleRoundRobin:
		interp.sched = scheduler.NewRoundRobin()
	case ScheduleRandom:
		interp.sched = scheduler.NewRandom(config.RandomSeed)
	case SchedulePCT:
		interp.sched = scheduler.NewPCT(config.RandomSeed, config.BugDepth)
	default:
		interp.sched = scheduler.NewRoundRobin()
	}

	return interp
}

// arenaNew allocates a value of type T in the interpreter's per-run arena
// and returns a raw *T. If no arena is configured (unit tests, or arena
// support unavailable) it falls back to regular heap allocation.
//
// Usage: frame := arenaNew(interp.arena, Frame{FuncName: fn})
//
// The returned pointer is valid for the lifetime of the arena (i.e., the
// entire Run() call). Do not retain it past Run().
func arenaNew[T any](a *safearena.Arena, val T) *T {
	if a == nil {
		v := val
		return &v
	}
	return safearena.Alloc(a, val).Get()
}

// Violations returns all UB violations detected during interpretation.
func (interp *Interpreter) Violations() []error {
	return interp.violations
}

// captureStack returns the current call stack for a goroutine, innermost first.
func (interp *Interpreter) captureStack(gid int64) []StackFrame {
	g := interp.goroutines[gid]
	if g == nil {
		return nil
	}
	frames := make([]StackFrame, 0, len(g.Stack))
	for i := len(g.Stack) - 1; i >= 0; i-- {
		f := g.Stack[i]
		frames = append(frames, StackFrame{FuncName: f.FuncName, Site: f.Site})
	}
	return frames
}

// recordViolation adds a detected violation, wrapping it with the goroutine's
// current call stack so reporters can display accurate stack traces.
// If the current instruction's site matches a //giri:ignore suppression, the
// violation is silently dropped (#58).
func (interp *Interpreter) recordViolation(gid int64, err error) {
	// Check suppression before allocating the wrapper.
	if interp.currentSite != "" && len(interp.suppressions) > 0 {
		if interp.suppressions[interp.currentSite] {
			return
		}
	}
	var spawnSite string
	if g := interp.goroutines[gid]; g != nil {
		spawnSite = g.SpawnSite
	}
	wrapped := &ViolationWithStack{
		Err:       err,
		GID:       gid,
		SpawnSite: spawnSite,
		Frames:    interp.captureStack(gid),
	}
	interp.violations = append(interp.violations, wrapped)
}

// --- Goroutine Management ---

// spawnGoroutine creates a new goroutine and returns its ID.
func (interp *Interpreter) spawnGoroutine(funcName, site string) (int64, error) {
	id := interp.nextGID.Add(1)

	if interp.config.MaxGoroutines > 0 && len(interp.goroutines) >= interp.config.MaxGoroutines {
		return 0, fmt.Errorf("goroutine limit reached (%d)", interp.config.MaxGoroutines)
	}

	// Arena-allocate the initial frame and goroutine struct. These are
	// hot allocations: every goroutine spawn (ssa.Go) and every interpreter
	// start goes through here.
	initialFrame := arenaNew(interp.arena, Frame{
		FuncName: funcName,
		Site:     site,
	})
	initialFrame.Locals = make(map[string]Value)

	g := arenaNew(interp.arena, Goroutine{
		ID:     id,
		Status: GoroutineRunning,
		VClock: NewVectorClock(id),
		Stack:  []*Frame{initialFrame},
	})

	interp.goroutines[id] = g
	return id, nil
}

// currentFrame returns the top of the call stack for a goroutine.
func (interp *Interpreter) currentFrame(gid int64) *Frame {
	g := interp.goroutines[gid]
	if g == nil || len(g.Stack) == 0 {
		return nil
	}
	return g.Stack[len(g.Stack)-1]
}

// pushFrame pushes a new call frame onto the goroutine's stack.
func (interp *Interpreter) pushFrame(gid int64, funcName, site string) *Frame {
	g := interp.goroutines[gid]
	caller := interp.currentFrame(gid)

	frame := arenaNew(interp.arena, Frame{
		FuncName: funcName,
		Site:     site,
		Locals:   make(map[string]Value),
		Caller:   caller,
	})
	g.Stack = append(g.Stack, frame)
	return frame
}

// popFrame pops the call frame, running deferred calls in LIFO order.
// During panic unwinding (g.Panicking=true), each defer is given a chance
// to call recover(). If recovery occurs, unwinding stops and execution
// resumes normally in the caller (#48).
func (interp *Interpreter) popFrame(gid int64) {
	g := interp.goroutines[gid]
	if g == nil || len(g.Stack) == 0 {
		return
	}

	frame := g.Stack[len(g.Stack)-1]

	// Execute deferred calls in LIFO order.
	for i := len(frame.Defers) - 1; i >= 0; i-- {
		d := frame.Defers[i]
		if g.Panicking {
			// During panic unwind: temporarily clear Panicking so the deferred
			// function can execute normally and potentially call recover().
			g.Panicking = false
			g.Recovered = false
			interp.executeDeferred(gid, d)
			if g.Recovered {
				// recover() was called — panic is suppressed; stop unwinding.
				g.Recovered = false
				break
			}
			// recover() was not called. If a new panic fired inside the defer,
			// g.Panicking is already true; otherwise restore it for the next defer.
			if !g.Panicking {
				g.Panicking = true
			}
		} else {
			interp.executeDeferred(gid, d)
		}
	}

	// Re-evaluate named return values if deferred closures may have modified
	// named-return allocs (e.g. `func f() (err error) { defer func() { err = wrap(err) }() }`).
	// Only applies when execution is NOT unwinding (g.Panicking would mean the
	// return path was aborted by a panic that wasn't recovered in this frame).
	if !g.Panicking && !g.Panicked && frame.ReturnInst != nil {
		interp.recomputeNamedReturns(frame)
	}

	// Poison stack allocs and evict their valueStore entries (#51, #60).
	// This runs AFTER defers and recomputeNamedReturns so that:
	//   • deferred closures can still use the allocs while they run, and
	//   • named-return values are extracted before the alloc is invalidated.
	// Go's SSA escape analysis guarantees that Heap=false allocs never have
	// surviving external references, so poisoning is always safe and never
	// produces false-positive UseAfterFreeErrors in well-formed programs.
	poisonSite := frame.Site + " (stack frame exited)"
	for _, id := range frame.StackAllocs {
		interp.Memory.Poison(id, poisonSite)
		if interp.valueStore != nil {
			delete(interp.valueStore, id)
		}
	}

	g.Stack = g.Stack[:len(g.Stack)-1]
}

// recomputeNamedReturns re-evaluates the return values from a function's
// ssa.Return instruction after deferred closures have run. For each result
// that is a load from an alloc (named return variable), the latest value
// from valueStore is used if available (#49).
func (interp *Interpreter) recomputeNamedReturns(frame *Frame) {
	ri := frame.ReturnInst
	if ri == nil || interp.valueStore == nil {
		return
	}
	newVals := make([]Value, len(ri.Results))
	changed := false
	for i, r := range ri.Results {
		// Named return pattern: `t_n = *alloc_result; return t_n`
		// In SSA, r is an *ssa.UnOp with Op=token.MUL (pointer dereference).
		if unop, ok := r.(*ssa.UnOp); ok && unop.Op == token.MUL {
			allocName := unop.X.Name()
			if allocV, ok2 := frame.Locals[allocName]; ok2 && allocV.Provenance != nil {
				if stored, ok3 := interp.valueStore[allocV.Provenance.Alloc]; ok3 {
					newVals[i] = stored
					changed = true
					continue
				}
			}
		}
		// Fall back to the value captured at ssa.Return time.
		if v, ok := frame.Locals[r.Name()]; ok {
			newVals[i] = v
		}
	}
	if !changed {
		return
	}
	switch len(newVals) {
	case 0:
	case 1:
		frame.Locals["__return__"] = newVals[0]
	default:
		frame.Locals["__return__"] = Value{Raw: newVals}
	}
}

// executeDeferred runs a single deferred call, dispatching to the appropriate
// handler: closure, sync package call, stdlib call, arena.Free, or general
// SSA function (#47).
func (interp *Interpreter) executeDeferred(gid int64, d DeferredCall) {
	// Closure call: deferred anonymous function with captured free variables.
	if d.IsClosure && d.ClosureVal != nil {
		allArgs := append(d.Args, d.ClosureVal.FreeVars...)
		interp.execFunction(gid, d.ClosureVal.Fn, allArgs)
		return
	}

	// sync package: Unlock, Done, etc. — modeled as vector-clock updates.
	if d.PkgPath == "sync" && d.FuncName != "" {
		interp.handleSyncCall(gid, d.FuncName, d.Args, d.Site)
		return
	}

	// stdlib intercept (strings, strconv, fmt, time, …): modeled directly.
	if d.PkgPath != "" && d.FuncName != "" && d.Callee != nil {
		if _, ok := interp.execStdlibCall(d.PkgPath, d.FuncName, d.Args); ok {
			return
		}
	}

	// arena.Free() special case: poisons arena allocations.
	if d.Callee != nil && strings.HasSuffix(d.Callee.String(), ".Free") && len(d.Args) > 0 {
		interp.handleArenaFree(gid, d)
		return
	}

	// General SSA function with an interpretable body.
	if d.Callee != nil && d.Callee.Blocks != nil {
		interp.execFunction(gid, d.Callee, d.Args)
		return
	}
	// Unresolved / external callee: silently ignore (can't interpret).
}

// --- Instruction Interpretation Stubs ---
// These will be filled in as we implement each SSA instruction type.

// handleAlloc interprets an allocation instruction (new, make, arena.New).
// Callers with a concrete types.Type should use Memory.Allocate directly with typeSizeOf.
func (interp *Interpreter) handleAlloc(gid int64, typeName, site string, isArena bool, arenaID shadow.ArenaID) Value {
	typeSize := 8 // Conservative default; Alloc SSA case uses typeSizeOf directly

	var allocID shadow.AllocID
	if isArena && arenaID != 0 {
		allocID = interp.Memory.AllocateInArena(arenaID, typeSize, typeName, site)
	} else {
		allocID = interp.Memory.Allocate(shadow.AllocHeap, typeSize, typeName, site)
	}

	ptr := arenaNew(interp.arena, shadow.Pointer{
		Alloc:  allocID,
		Offset: 0,
	})

	return Value{
		Raw:        ptr,
		Provenance: ptr,
	}
}

// handleLoad interprets a load (dereference) instruction.
func (interp *Interpreter) handleLoad(gid int64, addr Value, size int, site string) (Value, error) {
	if addr.Provenance == nil {
		// Distinguish nil pointer dereference from an untracked (opaque) value.
		// When addr.Raw is also nil, this is a program nil dereference (#36).
		if addr.Raw == nil {
			g := interp.goroutines[gid]
			if g != nil {
				g.Panicked = true
			}
			return Value{}, &shadow.NilPointerDerefError{Site: site, GID: gid}
		}
		// Non-nil Raw but no provenance: untracked value, skip shadow checks.
		return Value{Raw: addr.Raw}, nil
	}

	g := interp.goroutines[gid]
	err := interp.Memory.CheckAccess(addr.Provenance, size, shadow.AccessRead, site, g.ID)
	if err != nil {
		return Value{}, err
	}

	// Run all registered detectors (pass vector clock for race detection)
	if interp.registry != nil {
		for _, rerr := range interp.registry.CheckAccess(interp.Memory, addr.Provenance, size, shadow.AccessRead, site, g.ID, g.VClock.Clocks) {
			interp.recordViolation(gid, rerr)
		}
	}

	// If a concrete value was stored at this address, return it. This provides
	// proper load/store semantics for pointer indirection (e.g. **int loads,
	// closure-captured variables, and named-return allocs). Only applies to
	// offset-0 loads to avoid confusion with field/index addresses (#47, #49).
	if addr.Provenance.Offset == 0 && interp.valueStore != nil {
		if stored, ok := interp.valueStore[addr.Provenance.Alloc]; ok {
			return stored, nil
		}
	}

	// The loaded value inherits provenance if it's a pointer type
	return Value{
		Raw:        addr.Raw,
		Provenance: addr.Provenance,
	}, nil
}

// handleStore interprets a store instruction.
func (interp *Interpreter) handleStore(gid int64, addr Value, val Value, size int, site string) error {
	if addr.Provenance == nil {
		// Nil pointer store (#36)
		if addr.Raw == nil {
			g := interp.goroutines[gid]
			if g != nil {
				g.Panicked = true
			}
			return &shadow.NilPointerDerefError{Site: site, GID: gid}
		}
		return nil // untracked destination, skip
	}

	g := interp.goroutines[gid]
	err := interp.Memory.CheckAccess(addr.Provenance, size, shadow.AccessWrite, site, g.ID)
	if err != nil {
		return err
	}

	// Run all registered detectors (pass vector clock for race detection)
	if interp.registry != nil {
		for _, rerr := range interp.registry.CheckAccess(interp.Memory, addr.Provenance, size, shadow.AccessWrite, site, g.ID, g.VClock.Clocks) {
			interp.recordViolation(gid, rerr)
		}
	}

	// Mark bytes as initialized
	interp.Memory.MarkInitialized(addr.Provenance.Alloc, addr.Provenance.Offset, size)

	// Check for arena pointer escape via store to global variable (#35).
	// Storing an arena-allocated pointer into a global means the pointer
	// could outlive the arena — flag it as an escape.
	if interp.config.TrackArenas && val.Provenance != nil {
		srcAlloc, srcOk := interp.Memory.GetAllocation(val.Provenance.Alloc)
		if srcOk && srcAlloc.Kind == shadow.AllocArena {
			destAlloc, destOk := interp.Memory.GetAllocation(addr.Provenance.Alloc)
			if destOk && destAlloc.Kind == shadow.AllocGlobal {
				interp.recordViolation(gid, &shadow.EscapedPointerError{
					AllocID:    srcAlloc.ID,
					ArenaID:    srcAlloc.Arena,
					AllocSite:  srcAlloc.AllocSite,
					EscapeSite: site,
					EscapeKind: "global",
				})
			}
		}
	}

	// Track provenance: if storing a pointer, record what's now at this location
	if val.Provenance != nil {
		interp.Memory.TrackPointer(
			fmt.Sprintf("store@%s", site),
			val.Provenance,
		)
	}

	// Record the stored value for named-return defer re-evaluation (#49).
	// Only track stores to offset 0 (scalar or the first field of a struct)
	// to avoid clobbering struct-field tracking with partial writes.
	if addr.Provenance != nil && addr.Provenance.Offset == 0 && interp.valueStore != nil {
		interp.valueStore[addr.Provenance.Alloc] = val
	}

	return nil
}

// handleFieldAddr interprets a field address computation (s.Field).
func (interp *Interpreter) handleFieldAddr(gid int64, base Value, fieldOffset int, site string) Value {
	if base.Provenance == nil {
		return Value{}
	}

	derived := interp.Memory.DerivePointer(base.Provenance, fieldOffset)
	return Value{
		Raw:        derived,
		Provenance: derived,
	}
}

// handleIndexAddr interprets an index address computation (a[i]).
func (interp *Interpreter) handleIndexAddr(gid int64, base Value, index, elemSize int, site string) Value {
	if base.Provenance == nil {
		return Value{}
	}

	derived := interp.Memory.DerivePointer(base.Provenance, index*elemSize)
	return Value{
		Raw:        derived,
		Provenance: derived,
	}
}

// handleUnsafePointer interprets unsafe.Pointer conversions and arithmetic.
// targetType is the destination type of the Convert instruction (e.g. *uint32).
// valueID is the SSA name of the result value (used for uintptr tracking).
func (interp *Interpreter) handleUnsafePointer(gid int64, op UnsafeOp, val Value, site string, targetType types.Type, valueID string) (Value, error) {
	if !interp.config.TrackUnsafe {
		return val, nil
	}

	switch op {
	case UnsafeOpToPointer:
		// *T → unsafe.Pointer: legal, preserve provenance
		return val, nil

	case UnsafeOpFromPointer:
		// Rule 6: unsafe.Pointer → *reflect.SliceHeader or *reflect.StringHeader.
		// These types should not be manipulated via unsafe.Pointer; use
		// unsafe.SliceData / unsafe.StringData instead (Go 1.17+).
		if targetType != nil && isReflectHeaderType(targetType) {
			return val, &shadow.UnsafePointerViolation{
				Rule: shadow.RuleSliceHeader,
				Site: site,
				Details: fmt.Sprintf(
					"unsafe.Pointer → %s: use unsafe.SliceData/unsafe.StringData instead (Go 1.17+)",
					targetType,
				),
			}
		}

		// Rule 1: the offset into the allocation must be divisible by align(T).
		if val.Provenance != nil && targetType != nil {
			elemType := deref(targetType)
			if elemType != nil {
				align := int(interp.sizes.Alignof(elemType))
				if align > 1 && val.Provenance.Offset%align != 0 {
					return val, &shadow.UnsafePointerViolation{
						Rule: shadow.RuleConversion,
						Site: site,
						Details: fmt.Sprintf(
							"unsafe.Pointer → %s: offset %d is not aligned to %d bytes",
							targetType, val.Provenance.Offset, align,
						),
					}
				}
			}
		}
		return val, nil

	case UnsafeOpToUintptr:
		// unsafe.Pointer → uintptr: legal but dangerous.
		// Record the pending conversion so we can flag it if a GC point occurs.
		if interp.registry != nil && val.Provenance != nil {
			interp.registry.RecordUintptrConversion(valueID, site, val.Provenance)
		}
		return val, nil

	case UnsafeOpArithmetic:
		// uintptr → unsafe.Pointer: the uintptr is being consumed. Clear it from
		// pending so we don't flag a false GC-point violation.
		if interp.registry != nil {
			interp.registry.ClearAllUintptrConversions()
		}
		// Also check bounds (Rule 3) for the resulting pointer
		if val.Provenance != nil {
			alloc, ok := interp.Memory.GetAllocation(val.Provenance.Alloc)
			if ok && (val.Provenance.Offset < 0 || val.Provenance.Offset > alloc.Size) {
				err := &shadow.UnsafePointerViolation{
					Rule: shadow.RuleArithmetic,
					Site: site,
					Details: fmt.Sprintf(
						"pointer arithmetic resulted in offset %d, but allocation is only %d bytes",
						val.Provenance.Offset, alloc.Size,
					),
				}
				return val, err
			}
		}
		return val, nil
	}

	return val, nil
}

// UnsafeOp identifies the type of unsafe.Pointer operation.
type UnsafeOp uint8

const (
	UnsafeOpToPointer   UnsafeOp = iota // *T → unsafe.Pointer
	UnsafeOpFromPointer                 // unsafe.Pointer → *T
	UnsafeOpToUintptr                   // unsafe.Pointer → uintptr
	UnsafeOpArithmetic                  // uintptr arithmetic → unsafe.Pointer
)

// isReflectHeaderType reports whether t is *reflect.SliceHeader or *reflect.StringHeader.
// These types are deprecated in Go 1.17+ and their manipulation via unsafe.Pointer
// violates Rule 6 of Go's unsafe.Pointer rules.
func isReflectHeaderType(t types.Type) bool {
	pt, ok := t.(*types.Pointer)
	if !ok {
		return false
	}
	named, ok := pt.Elem().(*types.Named)
	if !ok {
		return false
	}
	obj := named.Obj()
	return obj.Pkg() != nil && obj.Pkg().Path() == "reflect" &&
		(obj.Name() == "SliceHeader" || obj.Name() == "StringHeader")
}

// handleArenaNew interprets arena.New[T](a) calls.
func (interp *Interpreter) handleArenaNew(gid int64, arenaVal Value, typeName, site string) (Value, error) {
	if !interp.config.TrackArenas {
		return interp.handleAlloc(gid, typeName, site, false, 0), nil
	}

	// Extract arena ID from the value
	arenaID, ok := interp.resolveArenaID(arenaVal)
	if !ok {
		return Value{}, fmt.Errorf("arena.New with non-arena value at %s", site)
	}

	// Check if arena is already freed
	arenaState, exists := interp.Memory.GetArena(arenaID)
	if !exists {
		return Value{}, fmt.Errorf("arena.New with unknown arena at %s", site)
	}
	if arenaState.Freed {
		err := &shadow.UseAfterFreeError{
			ArenaID:    arenaID,
			AccessSite: site,
			FreeSite:   arenaState.FreeSite,
		}
		return Value{}, err
	}

	return interp.handleAlloc(gid, typeName, site, true, arenaID), nil
}

// handleArenaFree interprets arena.Free() calls.
func (interp *Interpreter) handleArenaFree(gid int64, d DeferredCall) {
	if !interp.config.TrackArenas || len(d.Args) == 0 {
		return
	}

	arenaID, ok := interp.resolveArenaID(d.Args[0])
	if !ok {
		return
	}

	errs := interp.Memory.FreeArena(arenaID, d.Site)
	for _, err := range errs {
		interp.recordViolation(gid, err)
	}
}

// resolveArenaID extracts an ArenaID from an interpreted value.
func (interp *Interpreter) resolveArenaID(val Value) (shadow.ArenaID, bool) {
	// In the full implementation, this would look up the arena
	// object in the interpreter's value space and extract its ID.
	// For now, we use a convention where arena values store the ID.
	if id, ok := val.Raw.(shadow.ArenaID); ok {
		return id, true
	}
	return 0, false
}

// createChannel allocates a new channel and returns its ChanID.
// capacity == 0 creates an unbuffered channel; capacity > 0 creates a buffered channel.
func (interp *Interpreter) createChannel(capacity int) ChanID {
	id := ChanID(interp.nextChanID.Add(1))
	interp.channels[id] = &chanEntry{capacity: capacity}
	return id
}

// handleReturn interprets a return instruction.
// Checks for arena pointer escapes via return values.
func (interp *Interpreter) handleReturn(gid int64, values []Value, site string) {
	if !interp.config.TrackArenas {
		return
	}

	for _, val := range values {
		if val.Provenance == nil {
			continue
		}

		alloc, ok := interp.Memory.GetAllocation(val.Provenance.Alloc)
		if !ok {
			continue
		}

		// If returning an arena-allocated pointer, it's an escape
		if alloc.Kind == shadow.AllocArena && alloc.Arena != 0 {
			interp.recordViolation(gid, &shadow.EscapedPointerError{
				AllocID:    alloc.ID,
				ArenaID:    alloc.Arena,
				AllocSite:  alloc.AllocSite,
				EscapeSite: site,
				EscapeKind: "return",
			})
		}
	}
}

// handleChannelClose marks a channel as closed (#31).
// Subsequent sends to the channel will record a violation.
func (interp *Interpreter) handleChannelClose(gid int64, chanID ChanID, site string) {
	if ch, ok := interp.channels[chanID]; ok {
		if ch.closed {
			g := interp.goroutines[gid]
			if g != nil {
				g.Panicked = true
			}
			interp.recordViolation(gid, &shadow.DoubleCloseError{Site: site, GID: gid})
			return
		}
		ch.closed = true
	}
}

// handleChannelSend interprets a channel send and checks for escapes.
// It records the sender's vector clock in the channel state for happens-before propagation.
// For buffered channels (capacity > 0): blocks the goroutine when the buffer is full.
func (interp *Interpreter) handleChannelSend(gid int64, chanID ChanID, val Value, site string) {
	g := interp.goroutines[gid]

	// Check for send on closed channel (#31)
	ch, ok := interp.channels[chanID]
	if ok && ch.closed {
		if g != nil {
			g.Panicked = true
		}
		interp.recordViolation(gid, fmt.Errorf("send on closed channel at %s (goroutine %d)", site, gid))
		return
	}

	// Buffered channel: block the goroutine if the buffer is full (#44)
	if ok && ch.capacity > 0 && ch.pendingCount >= ch.capacity {
		if g != nil {
			g.Status = GoroutineBlocked
			g.BlockSite = site
			g.BlockChanID = chanID
			g.BlockOnSend = true
		}
		// Still record that this channel has a sender.
		interp.channelSenders[chanID] = true
		return
	}

	// Synchronize vector clocks and record sender clock in channel state
	if interp.config.TrackRaces {
		if g != nil {
			g.VClock.Tick(gid)
		}
		if ok {
			ch.lastSenderGID = gid
			ch.lastSenderClock = make(map[int64]uint64, len(g.VClock.Clocks))
			for k, v := range g.VClock.Clocks {
				ch.lastSenderClock[k] = v
			}
			ch.hasPending = true
			ch.pendingVal = val
			if ch.capacity > 0 {
				ch.pendingCount++
			}
		}
	} else {
		// Even without race tracking, mark the channel as having a pending value
		// so goroutine leak detection works correctly (a receiver is not a leak
		// if a sender exists, regardless of scheduling order).
		if ok {
			ch.hasPending = true
			if ch.capacity > 0 {
				ch.pendingCount++
			}
		}
	}

	// Track that this channel has had a sender (for goroutine leak detection).
	interp.channelSenders[chanID] = true

	// Check for arena pointer escape via channel
	if interp.config.TrackArenas && val.Provenance != nil {
		alloc, allocOk := interp.Memory.GetAllocation(val.Provenance.Alloc)
		if allocOk && alloc.Kind == shadow.AllocArena {
			interp.recordViolation(gid, &shadow.EscapedPointerError{
				AllocID:    alloc.ID,
				ArenaID:    alloc.Arena,
				AllocSite:  alloc.AllocSite,
				EscapeSite: site,
				EscapeKind: "channel",
			})
		}
	}
}

// handleChannelRecv interprets a channel receive.
// It merges the sender's clock from the channel state into the receiver's clock.
// For buffered channels, decrements pendingCount when a buffered value is consumed.
func (interp *Interpreter) handleChannelRecv(gid int64, chanID ChanID, site string) {
	// Track that this channel has had a receiver.
	interp.channelReceivers[chanID] = true

	if !interp.config.TrackRaces {
		// Still decrement pendingCount for buffered channels (#44)
		if ch, ok := interp.channels[chanID]; ok && ch.capacity > 0 && ch.pendingCount > 0 {
			ch.pendingCount--
			if ch.pendingCount == 0 {
				ch.hasPending = false
			}
		}
		return
	}

	g := interp.goroutines[gid]
	if ch, ok := interp.channels[chanID]; ok && ch.lastSenderGID != 0 {
		for id, t := range ch.lastSenderClock {
			if t > g.VClock.Clocks[id] {
				g.VClock.Clocks[id] = t
			}
		}
		if ch.capacity > 0 && ch.pendingCount > 0 {
			ch.pendingCount--
		}
		if ch.pendingCount == 0 {
			ch.hasPending = false // value consumed
		}
	}
	g.VClock.Tick(gid)
}

// --- Stack Trace ---

// StackTrace returns a formatted stack trace for a goroutine.
func (interp *Interpreter) StackTrace(gid int64) string {
	g := interp.goroutines[gid]
	if g == nil {
		return "<unknown goroutine>"
	}

	var b strings.Builder
	fmt.Fprintf(&b, "goroutine %d:\n", gid)
	for i := len(g.Stack) - 1; i >= 0; i-- {
		frame := g.Stack[i]
		fmt.Fprintf(&b, "  %s\n    %s\n", frame.FuncName, frame.Site)
	}
	return b.String()
}

// --- Finalization ---

// checkGoroutineLeaks reports goroutines that are permanently blocked on a
// channel with no corresponding counterpart. A recv-blocked goroutine is not
// a leak if a sender ran on that channel; a send-blocked goroutine is not a
// leak if a receiver ran on that channel.
// Also detects global deadlock: when ALL goroutines are blocked simultaneously
// and none has finished (i.e. main itself is blocked, not just spawned goroutines).
func (interp *Interpreter) checkGoroutineLeaks() []error {
	var leaks []error
	for _, g := range interp.goroutines {
		if g.Status != GoroutineBlocked {
			continue
		}
		if g.BlockOnSend {
			// Goroutine is blocked trying to send. Not a leak if a receiver ran.
			if g.BlockChanID != 0 && interp.channelReceivers[g.BlockChanID] {
				continue
			}
			leaks = append(leaks, &shadow.GoroutineLeakError{
				GID:       g.ID,
				SpawnSite: g.SpawnSite,
				BlockSite: g.BlockSite,
				BlockKind: "channel send",
			})
		} else {
			// Goroutine is blocked trying to receive. Not a leak if a sender ran.
			if g.BlockChanID != 0 && interp.channelSenders[g.BlockChanID] {
				continue
			}
			leaks = append(leaks, &shadow.GoroutineLeakError{
				GID:       g.ID,
				SpawnSite: g.SpawnSite,
				BlockSite: g.BlockSite,
				BlockKind: "channel receive",
			})
		}
	}

	// Deadlock detection (#56): if ALL goroutines are blocked and none has
	// finished, no goroutine can ever unblock the others — global deadlock.
	// Distinguished from goroutine leak: leak = main finished, some blocked;
	// deadlock = nobody finished (main is blocked too).
	allBlocked := true
	anyFinished := false
	blocked := 0
	for _, g := range interp.goroutines {
		if g.Panicked {
			continue // panicked goroutine is effectively dead
		}
		switch g.Status {
		case GoroutineFinished:
			anyFinished = true
		case GoroutineBlocked:
			blocked++
		default: // GoroutineRunning — shouldn't happen at Finish() time
			allBlocked = false
		}
	}
	if allBlocked && blocked > 0 && !anyFinished {
		leaks = append(leaks, &shadow.DeadlockError{GoroutineCount: blocked})
	}

	return leaks
}

// Finish is called when interpretation completes. Runs detector finalization
// checks (leak detection, pending uintptr checks, etc.).
func (interp *Interpreter) Finish() []error {
	var errs []error

	// Run detector finalization checks (arena leaks, pending uintptrs, etc.)
	if interp.registry != nil {
		errs = append(errs, interp.registry.Finalize(interp.Memory)...)
	}

	// Check for goroutine leaks (goroutines blocked with no sender).
	errs = append(errs, interp.checkGoroutineLeaks()...)

	// Collect recorded violations (already wrapped with call stack traces).
	errs = append(errs, interp.violations...)
	return errs
}

// handleSyncCall intercepts sync.Mutex and sync.WaitGroup method calls (#33).
// These runtime types use futexes that can't be interpreted; we model their
// clock semantics directly.
func (interp *Interpreter) handleSyncCall(gid int64, name string, args []Value, site string) Value {
	g := interp.goroutines[gid]
	if g == nil || len(args) == 0 {
		return Value{}
	}

	// Extract the AllocID of the sync primitive's backing memory as the map key.
	var key shadow.AllocID
	if ptr, ok := args[0].Raw.(*shadow.Pointer); ok {
		key = ptr.Alloc
	} else if args[0].Provenance != nil {
		key = args[0].Provenance.Alloc
	} else {
		return Value{} // can't track without shadow provenance
	}

	if _, exists := interp.mutexes[key]; !exists {
		interp.mutexes[key] = &mutexState{}
	}
	ms := interp.mutexes[key]

	switch name {
	case "Lock", "RLock":
		// Merge the last-unlock clock into the current goroutine's clock.
		// This establishes that everything before the matching Unlock HB this Lock.
		if ms.lastUnlockClock != nil {
			for id, t := range ms.lastUnlockClock {
				if t > g.VClock.Clocks[id] {
					g.VClock.Clocks[id] = t
				}
			}
		}
		ms.locked = true
		ms.lockGoroutine = gid

	case "Unlock", "RUnlock":
		// Tick the goroutine's clock, then snapshot it into the mutex state.
		// This establishes that this Unlock HB any subsequent Lock.
		g.VClock.Tick(gid)
		ms.locked = false
		ms.lastUnlockClock = make(map[int64]uint64, len(g.VClock.Clocks))
		for k, v := range g.VClock.Clocks {
			ms.lastUnlockClock[k] = v
		}

	case "Done":
		// WaitGroup.Done(): equivalent to Add(-1). Tick clock and snapshot it,
		// then decrement the counter and check for negative (#57).
		g.VClock.Tick(gid)
		ms.lastUnlockClock = make(map[int64]uint64, len(g.VClock.Clocks))
		for k, v := range g.VClock.Clocks {
			ms.lastUnlockClock[k] = v
		}
		ms.wgCounter--
		if ms.wgCounter < 0 {
			interp.recordViolation(gid, &shadow.WaitGroupNegativeError{
				Site:    site,
				GID:     gid,
				Counter: ms.wgCounter,
			})
		}

	case "Wait":
		// WaitGroup.Wait(): merge the Done-time clock (mirrors Lock semantics).
		if ms.lastUnlockClock != nil {
			for id, t := range ms.lastUnlockClock {
				if t > g.VClock.Clocks[id] {
					g.VClock.Clocks[id] = t
				}
			}
		}

	case "Add":
		// WaitGroup.Add(delta): update the counter; negative → violation (#57).
		if len(args) >= 2 {
			delta := int(toInt64(args[1]))
			ms.wgCounter += delta
			if ms.wgCounter < 0 {
				interp.recordViolation(gid, &shadow.WaitGroupNegativeError{
					Site:    site,
					GID:     gid,
					Counter: ms.wgCounter,
				})
			}
		}

	case "Store", "Delete", "Swap", "CompareAndSwap", "CompareAndDelete":
		// sync.Map write methods (#46): modeled as noop — the map is already
		// tracked separately; intercepting here prevents false-positive races.

	case "Load", "LoadOrStore", "LoadAndDelete":
		// sync.Map read methods (#46): return (Value{}, false) tuple.
		return Value{Raw: []Value{{}, {Raw: false}}}

	case "Range":
		// sync.Map.Range (#46): noop (can't iterate without concrete values).
	}

	return Value{}
}

// --- Helpers ---

// typeSizeOf returns the size in bytes of t for the target platform.
func (interp *Interpreter) typeSizeOf(t types.Type) int {
	if t == nil {
		return 8
	}
	return int(interp.sizes.Sizeof(t))
}
