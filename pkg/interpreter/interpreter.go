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
}

// DeferredCall represents a deferred function invocation.
type DeferredCall struct {
	Fn   string  // Function name
	Args []Value // Captured arguments
	Site string  // Where defer was declared
}

// Goroutine represents a single goroutine's execution state.
type Goroutine struct {
	ID     int64
	Stack  []*Frame // Call stack, top of stack = last element
	Status GoroutineStatus

	// Panicked is set when this goroutine has panicked or been halted
	// (e.g. execution step limit exceeded). All subsequent instructions
	// are skipped until the goroutine is removed from the run queue.
	Panicked bool

	// PanicValue holds the value passed to panic(), used by recover() (#34).
	PanicValue Value

	// Vector clock for happens-before tracking
	VClock *VectorClock
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
}

// mutexState tracks synchronization state for sync.Mutex and sync.WaitGroup (#33).
type mutexState struct {
	locked          bool
	lockGoroutine   int64
	lastUnlockClock map[int64]uint64 // clock snapshot at last Unlock/Done
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

	// Sync primitive state for sync.Mutex and sync.WaitGroup (#33).
	// Key is the AllocID of the mutex/waitgroup's shadow memory allocation.
	mutexes map[shadow.AllocID]*mutexState

	// Total SSA instructions executed (checked against Config.MaxSteps)
	steps uint64

	// Global variable state: maps each ssa.Global to its shadow-memory pointer.
	// Initialized in Run() by iterating all packages before main executes.
	globals map[*ssa.Global]Value
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
		Memory:     shadow.NewMemory(memOpts...),
		Fset:       fset,
		goroutines: make(map[int64]*Goroutine),
		channels:   make(map[ChanID]*chanEntry),
		mutexes:    make(map[shadow.AllocID]*mutexState),
		config:     config,
		sizes:      types.SizesFor("gc", runtime.GOARCH),
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

// Violations returns all UB violations detected during interpretation.
func (interp *Interpreter) Violations() []error {
	return interp.violations
}

// recordViolation adds a detected violation.
func (interp *Interpreter) recordViolation(err error) {
	interp.violations = append(interp.violations, err)
}

// --- Goroutine Management ---

// spawnGoroutine creates a new goroutine and returns its ID.
func (interp *Interpreter) spawnGoroutine(funcName, site string) (int64, error) {
	id := interp.nextGID.Add(1)

	if interp.config.MaxGoroutines > 0 && len(interp.goroutines) >= interp.config.MaxGoroutines {
		return 0, fmt.Errorf("goroutine limit reached (%d)", interp.config.MaxGoroutines)
	}

	g := &Goroutine{
		ID:     id,
		Status: GoroutineRunning,
		VClock: NewVectorClock(id),
		Stack: []*Frame{{
			FuncName: funcName,
			Site:     site,
			Locals:   make(map[string]Value),
		}},
	}

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

	frame := &Frame{
		FuncName: funcName,
		Site:     site,
		Locals:   make(map[string]Value),
		Caller:   caller,
	}
	g.Stack = append(g.Stack, frame)
	return frame
}

// popFrame pops the call frame, running deferred calls in LIFO order.
func (interp *Interpreter) popFrame(gid int64) {
	g := interp.goroutines[gid]
	if len(g.Stack) == 0 {
		return
	}

	frame := g.Stack[len(g.Stack)-1]

	// Execute deferred calls in LIFO order
	for i := len(frame.Defers) - 1; i >= 0; i-- {
		d := frame.Defers[i]
		interp.executeDeferred(gid, d)
	}

	g.Stack = g.Stack[:len(g.Stack)-1]
}

// executeDeferred runs a single deferred call.
func (interp *Interpreter) executeDeferred(gid int64, d DeferredCall) {
	// Key: if this is arena.Free(), we need to poison all arena allocations
	if strings.HasSuffix(d.Fn, ".Free") && len(d.Args) > 0 {
		interp.handleArenaFree(gid, d)
	}
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

	ptr := &shadow.Pointer{
		Alloc:  allocID,
		Offset: 0,
	}

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
			interp.recordViolation(rerr)
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
			interp.recordViolation(rerr)
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
				interp.recordViolation(&shadow.EscapedPointerError{
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
		// unsafe.Pointer → *T: legal if the resulting pointer's alignment is satisfied.
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
		interp.recordViolation(err)
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
func (interp *Interpreter) createChannel() ChanID {
	id := ChanID(interp.nextChanID.Add(1))
	interp.channels[id] = &chanEntry{}
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
			interp.recordViolation(&shadow.EscapedPointerError{
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
			interp.recordViolation(fmt.Errorf("close of already-closed channel at %s (goroutine %d)", site, gid))
			return
		}
		ch.closed = true
	}
}

// handleChannelSend interprets a channel send and checks for escapes.
// It records the sender's vector clock in the channel state for happens-before propagation.
func (interp *Interpreter) handleChannelSend(gid int64, chanID ChanID, val Value, site string) {
	g := interp.goroutines[gid]

	// Check for send on closed channel (#31)
	if ch, ok := interp.channels[chanID]; ok && ch.closed {
		if g != nil {
			g.Panicked = true
		}
		interp.recordViolation(fmt.Errorf("send on closed channel at %s (goroutine %d)", site, gid))
		return
	}

	// Synchronize vector clocks and record sender clock in channel state
	if interp.config.TrackRaces {
		if g != nil {
			g.VClock.Tick(gid)
		}
		if ch, ok := interp.channels[chanID]; ok {
			ch.lastSenderGID = gid
			ch.lastSenderClock = make(map[int64]uint64, len(g.VClock.Clocks))
			for k, v := range g.VClock.Clocks {
				ch.lastSenderClock[k] = v
			}
			ch.hasPending = true
			ch.pendingVal = val
		}
	}

	// Check for arena pointer escape via channel
	if interp.config.TrackArenas && val.Provenance != nil {
		alloc, ok := interp.Memory.GetAllocation(val.Provenance.Alloc)
		if ok && alloc.Kind == shadow.AllocArena {
			interp.recordViolation(&shadow.EscapedPointerError{
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
func (interp *Interpreter) handleChannelRecv(gid int64, chanID ChanID, site string) {
	if !interp.config.TrackRaces {
		return
	}

	g := interp.goroutines[gid]
	if ch, ok := interp.channels[chanID]; ok && ch.lastSenderGID != 0 {
		for id, t := range ch.lastSenderClock {
			if t > g.VClock.Clocks[id] {
				g.VClock.Clocks[id] = t
			}
		}
		ch.hasPending = false // value consumed
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

// Finish is called when interpretation completes. Runs detector finalization
// checks (leak detection, pending uintptr checks, etc.).
func (interp *Interpreter) Finish() []error {
	var errs []error

	// Run detector finalization checks (arena leaks, pending uintptrs, etc.)
	if interp.registry != nil {
		errs = append(errs, interp.registry.Finalize(interp.Memory)...)
	}

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
		// WaitGroup.Done(): tick clock, store snapshot (mirrors Unlock semantics).
		g.VClock.Tick(gid)
		ms.lastUnlockClock = make(map[int64]uint64, len(g.VClock.Clocks))
		for k, v := range g.VClock.Clocks {
			ms.lastUnlockClock[k] = v
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
		// WaitGroup.Add(): no clock effect — just tracks the counter.
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
