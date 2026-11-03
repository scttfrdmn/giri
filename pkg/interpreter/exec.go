// Package interpreter - exec.go implements the SSA instruction walker.
//
// This is the execution loop that drives interpretation. For each SSA
// instruction, it dispatches to the appropriate handler in interpreter.go,
// which performs the operation and validates it against shadow memory.
//
// The execution model:
// 1. Load program SSA via golang.org/x/tools/go/ssa
// 2. Find the entry point (main or test function)
// 3. Create initial goroutine
// 4. Execute SSA instructions one at a time
// 5. For each instruction: interpret + validate
// 6. When goroutines spawn, add them to the scheduler
// 7. Report all violations found
package interpreter

import (
	"errors"
	"fmt"
	"go/constant"
	"go/token"
	"go/types"
	"math/rand"
	"sort"
	"strings"
	"unicode/utf8"

	"golang.org/x/tools/go/ssa"

	"github.com/scttfrdmn/giri/pkg/shadow"
	"github.com/scttfrdmn/safearena"
)

// TestFunc identifies a single TestXxx function found in a _test.go file.
// Populated by ssautil.LoadTestPrograms; consumed by RunTests.
type TestFunc struct {
	Name string        // e.g. "TestMyRace"
	Fn   *ssa.Function // the SSA function object
}

// Program represents a loaded Go program ready for interpretation.
type Program struct {
	SSA  *ssa.Program
	Main *ssa.Package
	Fset *token.FileSet

	// Suppressions maps "file:line" → the list of violation categories each
	// //giri:ignore comment suppresses (#58, #229). An empty list means
	// suppress-all on that line; a non-empty list scopes suppression to those
	// categories. Matched against the current instruction site by
	// recordViolation.
	Suppressions map[string][]string

	// TestFuncs lists the TestXxx functions to run when this Program was
	// loaded in test mode (via ssautil.LoadTestPrograms). Nil for non-test
	// programs loaded with LoadAllPrograms.
	TestFuncs []TestFunc

	// GoVersion is the minimum Go version declared in the module's go.mod
	// (e.g. "go1.23"). Empty for non-module programs or when NeedModule is
	// not available. Informational only — used in CLI output and reports.
	GoVersion string

	// SourceHash is a hash of this program's transitive non-stdlib source
	// closure (the main package plus every reachable imported package's Go
	// files). It is the content component of the analysis cache key (#231).
	// Empty when hashing was unavailable (e.g. LoadProgram single-file mode).
	SourceHash string
}

// RunResult holds the results of interpreting a program.
type RunResult struct {
	Violations []error
	Stats      ExecutionStats
	MemStats   shadow.MemoryStats
	// SuppressedCount is the number of violations that were detected but
	// dropped because their site+category matched a //giri:ignore directive
	// (#229). Surfaced via -v so suppressions remain visible rather than silent.
	SuppressedCount int
	// SuppressedViolations holds the actual suppressed violations (#230), so the
	// reporter can emit them flagged-as-suppressed (SARIF suppressions / JSON
	// suppressed:true). Kept separate from Violations: they do not affect the
	// exit code or the active-violation count.
	SuppressedViolations []error
	// UnmodeledCalls is a sorted, deduplicated list of "pkg/path.FuncName"
	// strings for external functions that had no stdlib intercept and no
	// interpretable SSA body. Each such call returned an opaque zero value.
	// Populated during interpretation; inspect with -v to see coverage gaps.
	UnmodeledCalls []string
}

// ExecutionStats tracks execution metrics.
type ExecutionStats struct {
	InstructionsExecuted uint64
	GoroutinesSpawned    int
	AllocationsTotal     int
	ArenaAllocations     int
	UnsafeOperations     int
	ChannelOperations    int
}

// Run interprets the program and returns all detected violations.
//
// Run creates a per-interpretation arena for the lifetime of this call.
// Hot-path structs (Frame, Goroutine, SliceValue, shadow.Pointer) are
// arena-allocated via arenaNew, which eliminates per-object GC overhead
// for the millions of short-lived allocations a typical Run produces.
// The arena is freed automatically when Run returns.
func Run(prog *Program, config Config) *RunResult {
	return safearena.Scoped(func(a *safearena.Arena) *RunResult {
		interp := newWithArena(prog.Fset, config, a)
		interp.prog = prog.SSA
		interp.suppressions = prog.Suppressions

		// Initialize global variable state: allocate shadow memory for every
		// package-level variable so loads/stores via *ssa.Global are tracked.
		interp.globals = make(map[*ssa.Global]Value)
		for _, pkg := range prog.SSA.AllPackages() {
			for _, member := range pkg.Members {
				if g, ok := member.(*ssa.Global); ok {
					elemType := deref(g.Type())
					size := interp.typeSizeOf(elemType)
					if size <= 0 {
						size = 8
					}
					allocID := interp.Memory.Allocate(shadow.AllocGlobal, size, g.Type().String(), g.Name())
					ptr := arenaNew(interp.arena, shadow.Pointer{Alloc: allocID, Offset: 0})
					interp.globals[g] = Value{Raw: ptr, Provenance: ptr}
				}
			}
		}

		// Create the main goroutine
		mainGID, err := interp.spawnGoroutine("main", "entry")
		if err != nil {
			return &RunResult{Violations: []error{err}}
		}

		// Run package init() before main() to initialize package-level variables
		// declared with function-call initializers (e.g. flag.String, make, etc.).
		// Dependency package inits called from within init() are suppressed in
		// execStdlibCall; only the main package's own init body executes (#146).
		initFn := prog.Main.Func("init")
		mainFn := prog.Main.Func("main")

		// main runs on this host goroutine; goroutines it starts run until
		// none is runnable, past the end of main for coverage (#294).
		interp.runDriver(mainGID, func() {
			if initFn != nil {
				interp.execFunction(mainGID, initFn, nil)
			}
			if mainFn != nil {
				interp.execFunction(mainGID, mainFn, nil)
			}
		})

		// Run finalization checks
		finalErrs := interp.Finish()

		// Build sorted UnmodeledCalls list from the deduplicated map.
		var unmodeledList []string
		for k := range interp.unmodeledCalls {
			unmodeledList = append(unmodeledList, k)
		}
		sort.Strings(unmodeledList)

		return &RunResult{
			Violations:           finalErrs,
			MemStats:             interp.Memory.Stats(),
			UnmodeledCalls:       unmodeledList,
			SuppressedCount:      interp.suppressedCount,
			SuppressedViolations: interp.suppressedViolations,
		}
	})
}

// RunN interprets the program up to n times with randomized PCT scheduling,
// returning the union of all violations found across interleavings (#50).
//
// Each run uses a fresh interpreter and a PCT scheduler seeded differently.
// Violations are deduplicated by error string so that the same bug found in
// multiple runs is reported only once. The union surface covers bugs that only
// manifest under non-default goroutine orderings (e.g. a nil dereference that
// requires goroutine B to execute before goroutine A).
//
// RunN is intended for concurrency-heavy programs. For single-goroutine programs,
// Run is sufficient and faster.
func RunN(prog *Program, config Config, n int, seed int64) *RunResult {
	rng := rand.New(rand.NewSource(seed))
	seen := make(map[string]bool)
	seenUnmodeled := make(map[string]struct{})
	seenSuppressed := make(map[string]bool)
	var all []error
	var allSuppressed []error
	maxSuppressed := 0
	for i := 0; i < n; i++ {
		c := config
		c.ScheduleStrategy = SchedulePCT
		c.RandomSeed = rng.Int63()
		r := Run(prog, c)
		for _, v := range r.Violations {
			key := v.Error()
			if !seen[key] {
				seen[key] = true
				// Tag the violation with the seed that found it so callers
				// can reproduce the exact run with -strategy pct -seed N.
				var vws *ViolationWithStack
				if errors.As(v, &vws) {
					vws.ReproSeed = c.RandomSeed
				}
				all = append(all, v)
			}
		}
		for _, u := range r.UnmodeledCalls {
			seenUnmodeled[u] = struct{}{}
		}
		// Each run re-detects the same suppressions; take the max rather than
		// summing to avoid N× inflation of a single run's suppressed count.
		if r.SuppressedCount > maxSuppressed {
			maxSuppressed = r.SuppressedCount
		}
		// Union suppressed violations across runs, deduped by message (mirrors
		// the Violations dedup) so the reporter sees the full suppressed set.
		for _, v := range r.SuppressedViolations {
			key := v.Error()
			if !seenSuppressed[key] {
				seenSuppressed[key] = true
				allSuppressed = append(allSuppressed, v)
			}
		}
	}
	unmodeledList := make([]string, 0, len(seenUnmodeled))
	for k := range seenUnmodeled {
		unmodeledList = append(unmodeledList, k)
	}
	sort.Strings(unmodeledList)
	return &RunResult{Violations: all, UnmodeledCalls: unmodeledList, SuppressedCount: maxSuppressed, SuppressedViolations: allSuppressed}
}

// TestRunResult is the result of interpreting a single TestXxx function.
type TestRunResult struct {
	Name       string             // "TestFoo"
	Violations []error            // violations detected during this test
	MemStats   shadow.MemoryStats // memory usage snapshot
}

// Passed reports whether the test produced no violations.
func (r *TestRunResult) Passed() bool { return len(r.Violations) == 0 }

// RunTests interprets each TestXxx function listed in prog.TestFuncs and
// returns one TestRunResult per function. Functions are run in order;
// each test gets a fresh interpreter instance so that violations in one
// test cannot affect the results of another.
//
// RunTests is the counterpart to Run for programs loaded with
// ssautil.LoadTestPrograms. For regular programs, use Run or RunN.
func RunTests(prog *Program, config Config) []*TestRunResult {
	results := make([]*TestRunResult, 0, len(prog.TestFuncs))
	for _, tf := range prog.TestFuncs {
		results = append(results, runTestFn(prog, tf, config))
	}
	return results
}

// runTestFn interprets a single TestXxx function with an opaque *testing.T
// as its sole argument. It is structurally identical to Run but starts from
// tf.Fn instead of prog.Main.Func("main").
func runTestFn(prog *Program, tf TestFunc, config Config) *TestRunResult {
	result := safearena.Scoped(func(a *safearena.Arena) *RunResult {
		interp := newWithArena(prog.Fset, config, a)
		interp.prog = prog.SSA
		interp.suppressions = prog.Suppressions

		// Initialize global variable state (same as Run).
		interp.globals = make(map[*ssa.Global]Value)
		for _, pkg := range prog.SSA.AllPackages() {
			for _, member := range pkg.Members {
				if g, ok := member.(*ssa.Global); ok {
					elemType := deref(g.Type())
					size := interp.typeSizeOf(elemType)
					if size <= 0 {
						size = 8
					}
					allocID := interp.Memory.Allocate(shadow.AllocGlobal, size, g.Type().String(), g.Name())
					ptr := arenaNew(interp.arena, shadow.Pointer{Alloc: allocID, Offset: 0})
					interp.globals[g] = Value{Raw: ptr, Provenance: ptr}
				}
			}
		}

		// Spawn the entry goroutine named after the test function.
		mainGID, err := interp.spawnGoroutine(tf.Name, "entry")
		if err != nil {
			return &RunResult{Violations: []error{err}}
		}

		// Call the test function with an opaque *testing.T as its argument.
		// handleTestingCall in stdlib.go intercepts t.Fatal, t.Log, t.Run, etc.
		tArg := Value{Raw: struct{}{}}
		interp.runDriver(mainGID, func() {
			interp.execFunction(mainGID, tf.Fn, []Value{tArg})
		})

		finalErrs := interp.Finish()
		return &RunResult{Violations: finalErrs, MemStats: interp.Memory.Stats(), SuppressedCount: interp.suppressedCount, SuppressedViolations: interp.suppressedViolations}
	})
	return &TestRunResult{
		Name:       tf.Name,
		Violations: result.Violations,
		MemStats:   result.MemStats,
	}
}

// finishGoroutine records the end of goroutine gid's top-level function: an
// unrecovered panic leaves it Panicked; otherwise a goroutine still Running
// becomes Finished. A Blocked goroutine is left as it is.
func (interp *Interpreter) finishGoroutine(gid int64) {
	g := interp.goroutine(gid)
	if g == nil || g.Status != GoroutineRunning {
		return
	}
	if g.Panicking {
		// Unrecovered panic propagated to the top — goroutine terminates.
		g.Panicking = false
		g.Panicked = true
	}
	if !g.Panicked {
		g.Status = GoroutineFinished
		if interp.sched != nil {
			interp.sched.OnFinish(gid)
		}
	}
}

// startGoroutine starts t.fn(t.args...) as a new goroutine spawned by gid at
// site, as a go statement does, and returns its ID (0 if it couldn't start).
// It runs when gid next blocks or exits.
func (interp *Interpreter) startGoroutine(gid int64, t goroutineTask, funcName, site string) int64 {
	newGID, err := interp.spawnGoroutine(funcName, site)
	if err != nil {
		interp.recordViolation(gid, err)
		return 0
	}
	child := interp.goroutine(newGID)
	child.SpawnSite = site // for goroutine leak reports

	// The go statement is synchronized before the goroutine's start (#29):
	// the parent ticks, then the child inherits the parent's clock.
	if parent := interp.goroutine(gid); parent != nil {
		parent.VClock.Tick(gid)
		for k, v := range parent.VClock.Clocks {
			if v > child.VClock.Clocks[k] {
				child.VClock.Clocks[k] = v
			}
		}
	}

	interp.sched.OnSpawn(gid, newGID)
	t.gid = newGID
	interp.runQueue = append(interp.runQueue, t)
	return newGID
}

// execFunction interprets a single SSA function.
func (interp *Interpreter) execFunction(gid int64, fn *ssa.Function, args []Value) Value {
	if fn == nil || fn.Blocks == nil {
		return Value{} // External function, can't interpret
	}

	name, site := interp.funcIdent(fn)
	frame := interp.pushFrame(gid, name, site)
	interp.initSlots(frame, fn)
	defer interp.popFrame(gid)

	// Bind parameters
	for i, param := range fn.Params {
		if i < len(args) {
			interp.setLocal(frame, param, args[i])
		}
	}

	// Bind free variables (closures): callers append them after regular params.
	// See ClosureValue and the execCall/ssa.Go closure handling.
	freeVarStart := len(fn.Params)
	for i, fv := range fn.FreeVars {
		if freeVarStart+i < len(args) {
			interp.setLocal(frame, fv, args[freeVarStart+i])
		}
	}

	// Execute blocks, tracking the previous block for Phi node resolution.
	// Stop early if the goroutine is halted (panic or step limit).
	var prevBlock *ssa.BasicBlock
	block := fn.Blocks[0]
	for block != nil {
		g := interp.goroutine(gid)
		if g != nil && (g.Panicked || g.Panicking || g.Status == GoroutineBlocked) {
			break
		}
		if f := interp.currentFrame(gid); f != nil {
			f.PrevBlock = prevBlock
		}
		nextBlock := interp.execBlock(gid, fn, block)
		prevBlock = block
		block = nextBlock
	}

	// A panic is unwinding through this frame. Run its deferred calls now; if
	// one recovers, the function returns normally through its Recover block,
	// which reloads the named results (#259).
	if g := interp.goroutine(gid); g != nil && g.Panicking && !g.Panicked {
		if interp.runFrameDefers(gid, frame) && fn.Recover != nil && !g.Panicking && !g.Panicked {
			prevBlock, block = nil, fn.Recover
			for block != nil {
				if g.Panicked || g.Panicking || g.Status == GoroutineBlocked {
					return Value{}
				}
				frame.PrevBlock = prevBlock
				nextBlock := interp.execBlock(gid, fn, block)
				prevBlock = block
				block = nextBlock
			}
		}
	}

	// Return the result (if any)
	if retVal, ok := interp.getReturn(frame); ok {
		return retVal
	}
	return Value{}
}

// execBlock interprets a single basic block. Returns the next block to
// execute, or nil if the function should return.
func (interp *Interpreter) execBlock(gid int64, fn *ssa.Function, block *ssa.BasicBlock) *ssa.BasicBlock {
	frame := interp.currentFrame(gid)
	if frame == nil {
		return nil
	}

	// A block's phis are its leading instructions and take effect together:
	// for "a, b = b, a" in a loop, each phi must read the other's value from
	// the previous iteration, not the one just assigned. Evaluate all their
	// incoming values first; each Phi instruction then installs its own.
	frame.phiVals, frame.phiNext = frame.phiVals[:0], 0
	for _, instr := range block.Instrs {
		phi, ok := instr.(*ssa.Phi)
		if !ok {
			break
		}
		frame.phiVals = append(frame.phiVals, interp.phiIncoming(frame, phi))
	}

	for _, instr := range block.Instrs {
		// Stop if the goroutine has panicked, is unwinding a panic, or is blocked.
		if g := interp.goroutine(gid); g != nil && (g.Panicked || g.Panicking || g.Status == GoroutineBlocked) {
			return nil
		}
		next := interp.execInstruction(gid, fn, instr)
		if next != nil {
			return next // Branch taken
		}
	}

	return nil
}

// execInstruction interprets a single SSA instruction. Returns non-nil
// if the instruction is a branch/jump (next block to execute).
func (interp *Interpreter) execInstruction(gid int64, fn *ssa.Function, instr ssa.Instruction) *ssa.BasicBlock {
	// Library call mode has its own prologue: per-class step accounting, the
	// value-cache hook, tracing, and budget exhaustion without a violation.
	if interp.cs != nil {
		return interp.execInstructionSession(gid, fn, instr)
	}

	// Enforce execution step limit (#17)
	if interp.config.MaxSteps > 0 {
		interp.steps++
		if interp.steps > interp.config.MaxSteps {
			g := interp.goroutine(gid)
			if g != nil && !g.Panicked {
				interp.recordViolation(gid, fmt.Errorf(
					"execution limit of %d steps exceeded at %s",
					interp.config.MaxSteps, interp.posString(instr.Pos()),
				))
				interp.haltAll()
			}
			return nil
		}
	}

	site := interp.posString(instr.Pos())
	interp.currentSite = site // for //giri:ignore suppression checks (#58)
	return interp.execOp(gid, fn, instr, site)
}

// execOp performs the effect of a single SSA instruction. Returns non-nil if
// the instruction is a branch/jump (next block to execute).
func (interp *Interpreter) execOp(gid int64, fn *ssa.Function, instr ssa.Instruction, site string) *ssa.BasicBlock {
	frame := interp.currentFrame(gid)

	switch inst := instr.(type) {

	// --- Memory Operations ---

	case *ssa.Alloc:
		// Inline allocation with proper type sizing
		elemType := deref(inst.Type())
		size := interp.typeSizeOf(elemType)
		typeName := inst.Type().String()
		kind := shadow.AllocStack
		if inst.Heap {
			kind = shadow.AllocHeap
		}
		allocID := interp.allocate(kind, size, typeName, site)
		ptr := arenaNew(interp.arena, shadow.Pointer{Alloc: allocID, Offset: 0})
		interp.setLocal(frame, inst, Value{Raw: ptr, Provenance: ptr})
		// Track stack allocs for poisoning when the frame exits (#51).
		if kind == shadow.AllocStack {
			frame.StackAllocs = append(frame.StackAllocs, allocID)
		}

	case *ssa.Store:
		addr := interp.resolveValue(frame, inst.Addr)
		val := interp.resolveValue(frame, inst.Val)
		size := interp.typeSizeOf(inst.Val.Type())
		if interp.faithful {
			if err := interp.storeConcrete(gid, addr, val, inst.Val.Type(), size, site); err != nil {
				interp.recordViolation(gid, err)
			}
			break
		}
		if err := interp.handleStore(gid, addr, val, size, site); err != nil {
			interp.recordViolation(gid, err)
		}

	case *ssa.UnOp:
		operand := interp.resolveValue(frame, inst.X)
		switch inst.Op {
		case token.MUL: // Dereference (load)
			size := interp.typeSizeOf(inst.Type())
			if interp.faithful {
				result, err := interp.loadConcrete(gid, operand, inst.Type(), size, site)
				if err != nil {
					interp.recordViolation(gid, err)
				}
				interp.setLocal(frame, inst, result)
				break
			}
			result, err := interp.handleLoad(gid, operand, size, site)
			if err != nil {
				interp.recordViolation(gid, err)
			}
			interp.setLocal(frame, inst, result)
		case token.ARROW: // Channel receive (<-ch)
			// Receive from nil channel blocks forever in Go (deadlock) (#122).
			if operand.Raw == nil {
				interp.recordViolation(gid, &shadow.NilChannelError{Op: "receive", Site: site, GID: gid})
				interp.block(gid, waitInfo{site: site, kind: "channel receive"})
				break
			}
			var chanID ChanID
			if id, ok := operand.Raw.(ChanID); ok {
				chanID = id
			}
			// Wait until the channel has a value or is closed (#294).
			if chanID != 0 {
				if ch, ok := interp.channels[chanID]; ok && !interp.block(gid, recvWait(ch, chanID, site)) {
					break // aborted: execBlock sees GoroutineBlocked
				}
			}
			// Determine ok BEFORE consuming the value (#143).
			// ok=false only when the channel is both closed and fully drained.
			// Computing after handleChannelRecv wrongly returns ok=false for
			// the last real item (when pendingCount transitions 1→0 after recv).
			recvOk := true
			if chanID != 0 {
				if ch, exists := interp.channels[chanID]; exists {
					recvOk = !ch.closed || ch.hasPending || ch.pendingCount > 0
				}
			}
			interp.handleChannelRecv(gid, chanID, site)
			if inst.CommaOk {
				interp.setLocal(frame, inst, Value{Raw: []Value{{}, {Raw: recvOk}}})
			} else {
				interp.setLocal(frame, inst, Value{})
			}
		case token.SUB:
			if interp.concrete {
				if r, ok := concreteUnOp(inst.Op, operand, inst.Type()); ok {
					interp.setLocal(frame, inst, r)
					break
				}
			}
			if v, ok := operand.Raw.(int64); ok {
				interp.setLocal(frame, inst, Value{Raw: -v})
			} else if v, ok := operand.Raw.(float64); ok {
				interp.setLocal(frame, inst, Value{Raw: -v})
			} else if c, ok := operand.Raw.(complex128); ok { // (#144)
				interp.setLocal(frame, inst, Value{Raw: -c})
			} else {
				interp.setLocal(frame, inst, operand)
			}
		case token.NOT:
			if v, ok := operand.Raw.(bool); ok {
				interp.setLocal(frame, inst, Value{Raw: !v})
			} else {
				interp.setLocal(frame, inst, operand)
			}
		case token.XOR:
			if interp.concrete {
				if r, ok := concreteUnOp(inst.Op, operand, inst.Type()); ok {
					interp.setLocal(frame, inst, r)
					break
				}
			}
			if v, ok := operand.Raw.(int64); ok {
				interp.setLocal(frame, inst, Value{Raw: ^v})
			} else {
				interp.setLocal(frame, inst, operand)
			}
		default:
			interp.setLocal(frame, inst, operand)
		}

	case *ssa.BinOp:
		x := interp.resolveValue(frame, inst.X)
		y := interp.resolveValue(frame, inst.Y)
		if interp.concrete {
			interp.setLocal(frame, inst, interp.binOpConcrete(gid, inst, x, y, site))
			break
		}
		// Division/modulo by zero: statically-known zero divisor (#55).
		if inst.Op == token.QUO || inst.Op == token.REM {
			if yi, ok := y.Raw.(int64); ok && yi == 0 {
				interp.recordViolation(gid, &shadow.DivisionByZeroError{Site: site, GID: gid})
			}
		}
		// Negative shift count: x << n or x >> n where n < 0 panics at runtime
		// "runtime error: negative shift count" (Go 1.13+, #125).
		if inst.Op == token.SHL || inst.Op == token.SHR {
			if yi, ok := y.Raw.(int64); ok && yi < 0 {
				interp.recordViolation(gid, &shadow.NegativeShiftError{Count: yi, Site: site, GID: gid})
				interp.runtimePanic(gid, "negative shift amount")
			}
		}
		if inst.Op == token.EQL || inst.Op == token.NEQ {
			if _, isIface := inst.X.Type().Underlying().(*types.Interface); isIface {
				if eq, ok := ifaceEqual(x, y); ok {
					interp.setLocal(frame, inst, Value{Raw: eq == (inst.Op == token.EQL)})
					break
				}
			}
		}
		interp.setLocal(frame, inst, evalBinOp(inst.Op, x, y))

	case *ssa.FieldAddr:
		base := interp.resolveValue(frame, inst.X)
		fieldOffset := inst.Field * 8 // fallback
		if xType := deref(inst.X.Type()); xType != nil {
			if st, ok := xType.Underlying().(*types.Struct); ok {
				offsets := interp.sizes.Offsetsof(structFields(st))
				if inst.Field < len(offsets) {
					fieldOffset = int(offsets[inst.Field])
				}
			}
		}
		result := interp.handleFieldAddr(gid, base, fieldOffset, site)
		interp.setLocal(frame, inst, result)

	case *ssa.IndexAddr:
		base := interp.resolveValue(frame, inst.X)
		idx := interp.resolveValue(frame, inst.Index)
		indexVal := int(toInt64(idx))
		elemSize := 8
		nilSlice := false
		arrayOOB := false
		sliceOOB := false
		switch t := inst.X.Type().Underlying().(type) {
		case *types.Pointer:
			if arr, ok := t.Elem().Underlying().(*types.Array); ok {
				elemSize = interp.typeSizeOf(arr.Elem())
				arrLen := int(arr.Len())
				// Bounds check for pointer-to-array indexing (#133).
				if indexVal < 0 || indexVal >= arrLen {
					interp.recordViolation(gid, &shadow.OutOfBoundsError{
						AllocSize:  arrLen,
						Offset:     indexVal,
						AccessSize: elemSize,
						Site:       site,
					})
					interp.runtimePanic(gid, "index out of range")
					arrayOOB = true
				}
			}
		case *types.Slice:
			elemSize = interp.typeSizeOf(t.Elem())
			if base.Raw == nil {
				// Nil slice: uninitialized local (#126).
				nilSlice = true
			} else if sv, ok := base.Raw.(*SliceValue); ok {
				if sv.Backing == nil {
					// Nil slice: explicit nil backing (#126).
					nilSlice = true
				} else if indexVal < 0 || indexVal >= sv.Len {
					// OOB beyond declared length (#134): panics even when i < cap.
					interp.recordViolation(gid, &shadow.OutOfBoundsError{
						AllocSize:  sv.Len,
						Offset:     indexVal,
						AccessSize: elemSize,
						Site:       site,
					})
					interp.runtimePanic(gid, "index out of range")
					sliceOOB = true
				}
			}
		}
		if nilSlice {
			sliceLen := 0
			if sv, ok := base.Raw.(*SliceValue); ok {
				sliceLen = sv.Len
			}
			interp.recordViolation(gid, &shadow.OutOfBoundsError{
				AllocSize:  sliceLen,
				Offset:     indexVal,
				AccessSize: elemSize,
				Site:       site,
			})
			interp.runtimePanic(gid, "index out of range")
			interp.setLocal(frame, inst, Value{})
			break
		}
		if arrayOOB || sliceOOB {
			interp.setLocal(frame, inst, Value{})
			break
		}
		result := interp.handleIndexAddr(gid, base, indexVal, elemSize, site)
		interp.setLocal(frame, inst, result)

	case *ssa.Field:
		base := interp.resolveValue(frame, inst.X)
		if interp.faithful {
			interp.setLocal(frame, inst, interp.fieldConcrete(inst, base))
			break
		}
		if m, ok := base.Raw.(map[int]Value); ok {
			interp.setLocal(frame, inst, m[inst.Field])
		} else {
			interp.setLocal(frame, inst, Value{})
		}

	case *ssa.Index:
		x := interp.resolveValue(frame, inst.X)
		idx := interp.resolveValue(frame, inst.Index)
		idxInt := int(toInt64(idx))
		if _, isArr := inst.X.Type().Underlying().(*types.Array); isArr && interp.faithful {
			interp.setLocal(frame, inst, interp.indexArrayConcrete(gid, inst, x, idxInt, site))
			break
		}
		switch sv := x.Raw.(type) {
		case *SliceValue:
			// Slice element access: derive the element pointer and call handleLoad
			// so that BoundsDetector and RaceDetector fire on OOB/racy access (#25).
			elemSize := 8
			if t, ok := inst.X.Type().Underlying().(*types.Slice); ok {
				elemSize = interp.typeSizeOf(t.Elem())
			}
			if sv.Backing != nil {
				elemPtr := interp.derive(sv.Backing, idxInt*elemSize)
				addrVal := Value{Raw: elemPtr, Provenance: elemPtr}
				if interp.faithful {
					result, err := interp.loadConcrete(gid, addrVal, inst.Type(), elemSize, site)
					if err != nil {
						interp.recordViolation(gid, err)
					}
					interp.setLocal(frame, inst, result)
					break
				}
				result, err := interp.handleLoad(gid, addrVal, elemSize, site)
				if err != nil {
					interp.recordViolation(gid, err)
				}
				interp.setLocal(frame, inst, result)
			} else {
				// Nil slice (Backing==nil): any element access is out of bounds (#126).
				interp.recordViolation(gid, &shadow.OutOfBoundsError{
					AllocSize:  sv.Len,
					Offset:     idxInt,
					AccessSize: elemSize,
					Site:       site,
				})
				interp.runtimePanic(gid, "index out of range")
				interp.setLocal(frame, inst, Value{})
			}
		case string:
			// s[i] returns the byte at byte position i (#73), not a rune.
			// Out-of-bounds access panics at runtime (#124).
			if idxInt < 0 || idxInt >= len(sv) {
				interp.recordViolation(gid, &shadow.OutOfBoundsError{
					AllocSize:  len(sv),
					Offset:     idxInt,
					AccessSize: 1,
					Site:       site,
					TypeName:   "string",
				})
				interp.runtimePanic(gid, "index out of range")
				interp.setLocal(frame, inst, Value{})
			} else {
				interp.setLocal(frame, inst, Value{Raw: int64(sv[idxInt])})
			}
		case map[interface{}]Value:
			// Arrays stored as maps (rare but possible)
			interp.setLocal(frame, inst, sv[int64(idxInt)])
		default:
			interp.setLocal(frame, inst, Value{})
		}

	case *ssa.Extract:
		// Extract element from a multi-value tuple (e.g. multi-return)
		tuple := interp.resolveValue(frame, inst.Tuple)
		if elems, ok := tuple.Raw.([]Value); ok && inst.Index < len(elems) {
			interp.setLocal(frame, inst, elems[inst.Index])
		} else {
			interp.setLocal(frame, inst, Value{})
		}

	case *ssa.Lookup:
		x := interp.resolveValue(frame, inst.X)
		key := interp.resolveValue(frame, inst.Index)
		// Race check on map read (#46): use the map's shadow provenance.
		if x.Provenance != nil {
			if _, rerr := interp.handleLoad(gid, x, 8, site); rerr != nil {
				interp.recordViolation(gid, rerr)
			}
		}
		if m, ok := x.Raw.(map[interface{}]Value); ok {
			mk := toMapKey(key)
			val, found := m[mk]
			if inst.CommaOk {
				interp.setLocal(frame, inst, Value{Raw: []Value{val, {Raw: found}}})
			} else {
				interp.setLocal(frame, inst, val)
			}
		} else {
			if inst.CommaOk {
				interp.setLocal(frame, inst, Value{Raw: []Value{{}, {Raw: false}}})
			} else {
				interp.setLocal(frame, inst, Value{})
			}
		}

	case *ssa.MapUpdate:
		m := interp.resolveValue(frame, inst.Map)
		k := interp.resolveValue(frame, inst.Key)
		v := interp.resolveValue(frame, inst.Value)
		// Nil map write (#54): map value has no concrete backing.
		if m.Raw == nil {
			interp.recordViolation(gid, &shadow.NilMapWriteError{Site: site, GID: gid})
			break
		}
		// Race check on map write (#46): use the map's shadow provenance.
		if m.Provenance != nil {
			if werr := interp.handleStore(gid, m, v, 8, site); werr != nil {
				interp.recordViolation(gid, werr)
			}
		}
		if mapVal, ok := m.Raw.(map[interface{}]Value); ok {
			mapVal[toMapKey(k)] = v
		}

	case *ssa.MakeSlice:
		elemType := inst.Type().(*types.Slice).Elem()
		elemSize := interp.typeSizeOf(elemType)
		lenVal := interp.resolveValue(frame, inst.Len)
		capVal := interp.resolveValue(frame, inst.Cap)
		lenN := int(toInt64(lenVal))
		capN := int(toInt64(capVal))
		// Negative len or cap panics at runtime: "makeslice: len out of range" (#123).
		if lenN < 0 {
			interp.recordViolation(gid, &shadow.InvalidMakeArgError{
				Kind: "slice-len", Value: int64(lenN), Site: site, GID: gid,
			})
			lenN = 0
		}
		if capN < 0 {
			interp.recordViolation(gid, &shadow.InvalidMakeArgError{
				Kind: "slice-cap", Value: int64(capN), Site: site, GID: gid,
			})
			capN = 0
		}
		if capN < lenN {
			// len > cap panics at runtime: "makeslice: len larger than cap" (#135).
			interp.recordViolation(gid, &shadow.InvalidMakeArgError{
				Kind: "slice-len-gt-cap", Value: int64(lenN), Site: site, GID: gid,
			})
			capN = lenN // continue with clamped value
		}
		allocSize := capN * elemSize
		if allocSize <= 0 {
			allocSize = elemSize
		}
		allocID := interp.allocate(shadow.AllocHeap, allocSize, inst.Type().String(), site)
		ptr := arenaNew(interp.arena, shadow.Pointer{Alloc: allocID, Offset: 0})
		sv := arenaNew(interp.arena, SliceValue{Backing: ptr, Len: lenN, Cap: capN})
		interp.setLocal(frame, inst, Value{Raw: sv, Provenance: ptr})

	case *ssa.Slice:
		base := interp.resolveValue(frame, inst.X)
		var lowVal int
		if inst.Low != nil {
			lowVal = int(toInt64(interp.resolveValue(frame, inst.Low)))
		}
		highVal := -1
		if inst.High != nil {
			highVal = int(toInt64(interp.resolveValue(frame, inst.High)))
		}
		// 3-index slice s[low:high:max] — inst.Max controls the new capacity (#85).
		maxVal := -1
		if inst.Max != nil {
			maxVal = int(toInt64(interp.resolveValue(frame, inst.Max)))
		}

		// Determine the SliceValue to operate on.
		// inst.X may be:
		//   (a) []T — already a SliceValue (reslice)
		//   (b) *[N]T — pointer to array (make([]T,n) lowered to Alloc+Slice)
		if s, ok := base.Raw.(string); ok && interp.concrete {
			interp.setLocal(frame, inst, interp.sliceStringConcrete(gid, s, lowVal, highVal, site))
			break
		}
		var sv *SliceValue
		elemSize := 1
		if existingSV, ok := base.Raw.(*SliceValue); ok {
			sv = existingSV
			if t, ok2 := inst.X.Type().Underlying().(*types.Slice); ok2 {
				elemSize = interp.typeSizeOf(t.Elem())
			}
		} else if ptr, ok := base.Raw.(*shadow.Pointer); ok {
			// *[N]T → []T conversion (make([]T, n) lowered to Alloc+Slice)
			arrLen := 0
			if pt, ok2 := inst.X.Type().Underlying().(*types.Pointer); ok2 {
				if at, ok3 := pt.Elem().Underlying().(*types.Array); ok3 {
					arrLen = int(at.Len())
					elemSize = interp.typeSizeOf(at.Elem())
				}
			}
			sv = arenaNew(interp.arena, SliceValue{Backing: ptr, Len: arrLen, Cap: arrLen})
		}

		if sv != nil {
			if highVal < 0 {
				highVal = sv.Len
			}
			// For 3-index slices, max bounds the capacity; default to cap(s).
			if maxVal < 0 {
				maxVal = sv.Cap
			}
			// Bounds check: 0 <= low <= high <= max <= cap(s) (#85)
			if lowVal < 0 || highVal < lowVal || maxVal < highVal || maxVal > sv.Cap {
				interp.recordViolation(gid, &shadow.OutOfBoundsError{
					AllocSize:  sv.Cap,
					Offset:     maxVal,
					AccessSize: highVal - lowVal,
					Site:       site,
					TypeName:   inst.X.Type().String(),
				})
				// Produce a safe empty slice so execution can continue
				interp.setLocal(frame, inst, Value{Raw: arenaNew(interp.arena, SliceValue{Len: 0, Cap: 0})})
			} else {
				var newPtr *shadow.Pointer
				if sv.Backing != nil {
					newPtr = interp.derive(sv.Backing, lowVal*elemSize)
				}
				newSv := arenaNew(interp.arena, SliceValue{
					Backing: newPtr,
					Len:     highVal - lowVal,
					Cap:     maxVal - lowVal, // 3-index: cap = max - low (#85)
				})
				interp.setLocal(frame, inst, Value{Raw: newSv, Provenance: newPtr})
			}
		} else {
			interp.setLocal(frame, inst, base)
		}

	case *ssa.MakeClosure:
		// Capture the function and its free variable values into a ClosureValue.
		// The ClosureValue is passed to execCall/ssa.Go which appends the free
		// vars after regular params when invoking execFunction (#19).
		fn := inst.Fn.(*ssa.Function)
		freeVars := make([]Value, len(inst.Bindings))
		for i, binding := range inst.Bindings {
			freeVars[i] = interp.resolveValue(frame, binding)
		}
		interp.setLocal(frame, inst, Value{Raw: &ClosureValue{Fn: fn, FreeVars: freeVars}})

	case *ssa.MakeMap:
		// Negative size hint panics at runtime: "makemap: size out of range" (#136).
		if inst.Reserve != nil {
			reserveVal := interp.resolveValue(frame, inst.Reserve)
			if n := toInt64(reserveVal); n < 0 {
				interp.recordViolation(gid, &shadow.InvalidMakeArgError{
					Kind: "map-cap", Value: n, Site: site, GID: gid,
				})
			}
		}
		// Allocate a shadow pointer for the map so race detection can track
		// concurrent reads and writes to the same map (#46).
		allocID := interp.allocate(shadow.AllocHeap, 8, inst.Type().String(), site)
		ptr := arenaNew(interp.arena, shadow.Pointer{Alloc: allocID, Offset: 0})
		interp.setLocal(frame, inst, Value{Raw: make(map[interface{}]Value), Provenance: ptr})

	case *ssa.MakeChan:
		// Extract the channel capacity from the instruction operand (#44).
		// Negative capacity panics at runtime: "makechan: size out of range" (#123).
		capVal := interp.resolveValue(frame, inst.Size)
		capacity := int(toInt64(capVal))
		if capacity < 0 {
			interp.recordViolation(gid, &shadow.InvalidMakeArgError{
				Kind: "chan-cap", Value: int64(capacity), Site: site, GID: gid,
			})
			capacity = 0
		}
		chanID := interp.createChannel(capacity)
		interp.setLocal(frame, inst, Value{Raw: chanID})

	case *ssa.Range:
		base := interp.resolveValue(frame, inst.X)
		ri := &rangeIter{val: base, idx: 0, sortKeys: interp.concrete}
		// For range-over-array ([N]T or *[N]T), store N so advance() knows when to stop.
		// Without this, the iterator falls through to default and returns false immediately,
		// silently skipping all loop iterations (#137).
		switch t := inst.X.Type().Underlying().(type) {
		case *types.Array:
			ri.arrayLen = int(t.Len())
		case *types.Pointer:
			if at, ok := t.Elem().Underlying().(*types.Array); ok {
				ri.arrayLen = int(at.Len())
			}
		}
		interp.setLocal(frame, inst, Value{Raw: ri})

	case *ssa.Next:
		iter := interp.resolveValue(frame, inst.Iter)
		if ri, ok := iter.Raw.(*rangeIter); ok {
			// Range-over-channel: handle channel receives directly here (not in
			// rangeIter.advance) because we need access to the interpreter and gid (#143).
			if chanID, ok := ri.val.Raw.(ChanID); ok {
				if ch, exists := interp.channels[chanID]; exists {
					if !interp.block(gid, recvWait(ch, chanID, site)) {
						break // aborted
					}
					if ch.hasPending || ch.pendingCount > 0 {
						// Consume one value; return (true, receivedVal, _).
						val := ch.pendingVal
						interp.handleChannelRecv(gid, chanID, site)
						interp.setLocal(frame, inst, Value{Raw: []Value{{Raw: true}, val, {}}})
					} else if ch.closed {
						// Channel closed and empty: terminate the range loop.
						interp.observeClose(gid, ch)
						interp.setLocal(frame, inst, Value{Raw: []Value{{Raw: false}, {}, {}}})
					}
				} else {
					interp.setLocal(frame, inst, Value{Raw: []Value{{Raw: false}, {}, {}}})
				}
			} else {
				ok2, k, v := ri.advance()
				interp.setLocal(frame, inst, Value{Raw: []Value{{Raw: ok2}, k, v}})
			}
		} else {
			interp.setLocal(frame, inst, Value{Raw: []Value{{Raw: false}, {}, {}}})
		}

	case *ssa.TypeAssert:
		val := interp.resolveValue(frame, inst.X)
		assertedType := inst.AssertedType

		// Unwrap InterfaceValue to get the concrete type and value.
		concreteVal := val
		var concreteType types.Type
		if iv, ok := val.Raw.(*InterfaceValue); ok {
			concreteVal = iv.Value
			concreteType = iv.Type
		}

		var ok bool
		if concreteType == nil {
			// Unknown concrete type (value not boxed via MakeInterface in this trace).
			// CommaOk (type-switch chain): return false so we fall to the next case.
			// Direct assertion: remain conservative (true) to avoid false-positive panics.
			ok = !inst.CommaOk
		} else {
			ok = typeAssertSucceeds(concreteType, assertedType)
		}

		if inst.CommaOk {
			if ok {
				interp.setLocal(frame, inst, Value{Raw: []Value{concreteVal, {Raw: true}}})
			} else {
				interp.setLocal(frame, inst, Value{Raw: []Value{{}, {Raw: false}}})
			}
		} else {
			if !ok {
				// Non-comma-ok assertion that fails → runtime panic; record violation.
				interp.runtimePanic(gid, "interface conversion")
				interp.recordViolation(gid, &shadow.TypeAssertionError{
					Site:         site,
					ConcreteType: concreteType.String(),
					AssertedType: assertedType.String(),
					GID:          gid,
				})
			}
			interp.setLocal(frame, inst, concreteVal)
		}

	case *ssa.ChangeInterface:
		val := interp.resolveValue(frame, inst.X)
		if iv, ok := val.Raw.(*InterfaceValue); ok {
			// Preserve the concrete type through interface widening so TypeAssert
			// and invoke dispatch can still see the original dynamic type.
			interp.setLocal(frame, inst, Value{
				Raw:        &InterfaceValue{Type: iv.Type, Value: iv.Value},
				Provenance: val.Provenance,
			})
		} else {
			interp.setLocal(frame, inst, val)
		}

	case *ssa.SliceToArrayPointer:
		base := interp.resolveValue(frame, inst.X)
		if interp.faithful {
			// (*[N]T)(s) and [N]T(s) panic when len(s) < N.
			n := int(inst.Type().Underlying().(*types.Pointer).Elem().Underlying().(*types.Array).Len())
			sv, _ := base.Raw.(*SliceValue)
			if sv == nil {
				sv = &SliceValue{}
			}
			if sv.Len < n {
				esz := interp.typeSizeOf(inst.Type().Underlying().(*types.Pointer).Elem().Underlying().(*types.Array).Elem())
				interp.recordViolation(gid, &shadow.OutOfBoundsError{
					AllocSize: sv.Len * esz, Offset: 0, AccessSize: n * esz, Site: site, TypeName: inst.X.Type().String(),
				})
				interp.runtimePanic(gid, "index out of range")
				interp.setLocal(frame, inst, Value{})
				break
			}
			if sv.Backing == nil { // nil or empty slice converted to *[0]T
				interp.setLocal(frame, inst, Value{})
				break
			}
		}
		if sv, ok := base.Raw.(*SliceValue); ok {
			interp.setLocal(frame, inst, Value{Raw: sv.Backing, Provenance: sv.Backing})
		} else {
			interp.setLocal(frame, inst, base)
		}

	// --- Control Flow ---

	case *ssa.Jump:
		return inst.Block().Succs[0]

	case *ssa.If:
		cond := interp.resolveValue(frame, inst.Cond)
		if b, ok := cond.Raw.(bool); ok && b {
			return inst.Block().Succs[0] // True branch
		}
		return inst.Block().Succs[1] // False branch

	case *ssa.Return:
		// Check for arena escapes via return
		var retVals []Value
		for _, r := range inst.Results {
			retVals = append(retVals, interp.resolveValue(frame, r))
		}
		interp.handleReturn(gid, retVals, site)

		switch len(retVals) {
		case 0:
			// nothing to store
		case 1:
			interp.setReturn(frame, retVals[0])
		default:
			// Multi-value return: store as a tuple Value for Extract to pick apart
			interp.setReturn(frame, Value{Raw: retVals})
		}
		// Store the ssa.Return instruction so popFrame can re-evaluate named
		// return values after deferred closures may have modified them (#49).
		if frame != nil {
			frame.ReturnInst = inst
		}
		return nil // End of function

	case *ssa.Panic:
		// Set the Panicking flag and store the panic value. Actual stack unwinding
		// happens lazily: execFunction's block loop detects Panicking and exits,
		// which triggers the Go-level `defer interp.popFrame(gid)` in execFunction.
		// popFrame runs each frame's SSA defers, giving recover() a chance to fire
		// before the goroutine is fully terminated (#48).
		g := interp.goroutine(gid)
		if g != nil {
			g.PanicValue = interp.resolveValue(frame, inst.X)
			g.Panicking = true
		}
		return nil

	// --- Function Calls ---

	case *ssa.Call:
		result := interp.execCall(gid, fn, inst, site)
		if interp.vname(inst) != "" {
			interp.setLocal(frame, inst, result)
		}

	case *ssa.Defer:
		// Record deferred call for execution when frame pops.
		// Resolve the callee exactly as ssa.Go does so executeDeferred can
		// actually call the function (#47).
		d := DeferredCall{Site: site}
		for _, arg := range inst.Call.Args {
			d.Args = append(d.Args, interp.resolveValue(frame, arg))
		}
		callee := inst.Call.StaticCallee()
		// StaticCallee() looks through *ssa.MakeClosure to return the underlying
		// function. When that happens, inst.Call.Args is empty and the free var
		// bindings are in the MakeClosure node — we must extract them (#47).
		if mc, ok := inst.Call.Value.(*ssa.MakeClosure); ok {
			fn := mc.Fn.(*ssa.Function)
			freeVars := make([]Value, len(mc.Bindings))
			for i, binding := range mc.Bindings {
				freeVars[i] = interp.resolveValue(frame, binding)
			}
			d.IsClosure = true
			d.ClosureVal = &ClosureValue{Fn: fn, FreeVars: freeVars}
		} else if callee == nil && !inst.Call.IsInvoke() {
			// Dynamic call — check if the resolved value is a ClosureValue.
			calleeVal := interp.resolveValue(frame, inst.Call.Value)
			if cv, ok := calleeVal.Raw.(*ClosureValue); ok {
				d.IsClosure = true
				d.ClosureVal = cv
			} else {
				// Non-closure dynamic defer (e.g. defer cancel()): store the
				// resolved value so executeDeferred can intercept known types
				// like cancelFuncID (#120).
				d.DynCallVal = calleeVal
			}
		}
		if callee != nil && !d.IsClosure {
			d.Callee = callee
			if callee.Package() != nil && callee.Package().Pkg != nil {
				d.PkgPath = callee.Package().Pkg.Path()
				d.FuncName = callee.Name()
			}
		}
		frame.Defers = append(frame.Defers, d)

	case *ssa.Go:
		// Spawn goroutine — enqueue for execution via scheduler.
		// Handle both direct function calls and closure calls (#19).
		var args []Value
		for _, arg := range inst.Call.Args {
			args = append(args, interp.resolveValue(frame, arg))
		}
		callee := inst.Call.StaticCallee()
		// StaticCallee() looks through *ssa.MakeClosure to return the underlying
		// function. When that happens inst.Call.Args is empty and free var bindings
		// live in the MakeClosure node — extract them explicitly.
		if mc, ok := inst.Call.Value.(*ssa.MakeClosure); ok {
			fn := mc.Fn.(*ssa.Function)
			callee = fn
			for _, binding := range mc.Bindings {
				args = append(args, interp.resolveValue(frame, binding))
			}
		} else if callee == nil {
			// Dynamic call — check if the resolved value is a ClosureValue.
			calleeVal := interp.resolveValue(frame, inst.Call.Value)
			if cv, ok := calleeVal.Raw.(*ClosureValue); ok {
				callee = cv.Fn
				args = append(args, cv.FreeVars...)
			}
		}
		if callee == nil {
			break // Can't resolve the goroutine function
		}
		interp.startGoroutine(gid, goroutineTask{fn: callee, args: args}, interp.callTargetName(inst.Call), site)

	// --- Channel Operations ---

	case *ssa.Send:
		chanVal := interp.resolveValue(frame, inst.Chan)
		val := interp.resolveValue(frame, inst.X)
		// Send on nil channel blocks forever in Go (deadlock) (#122).
		if chanVal.Raw == nil {
			interp.recordViolation(gid, &shadow.NilChannelError{Op: "send", Site: site, GID: gid})
			interp.block(gid, waitInfo{site: site, kind: "channel send", onSend: true})
			break
		}
		var chanID ChanID
		if id, ok := chanVal.Raw.(ChanID); ok {
			chanID = id
		}
		interp.handleChannelSend(gid, chanID, val, site)

	case *ssa.Select:
		interp.execSelect(gid, frame, inst, site)

	// --- Type Conversions ---

	case *ssa.Convert:
		val := interp.resolveValue(frame, inst.X)
		// Check for unsafe.Pointer conversions
		if isUnsafePointerConversion(inst) {
			if interp.concrete {
				if r, ok := interp.convertUnsafeConcrete(gid, classifyUnsafeConversion(inst), val, site, inst); ok {
					interp.setLocal(frame, inst, r)
					break
				}
			}
			result, err := interp.handleUnsafePointer(gid, classifyUnsafeConversion(inst), val, site, inst.Type(), interp.vname(inst))
			if err != nil {
				interp.recordViolation(gid, err)
			}
			interp.setLocal(frame, inst, result)
		} else {
			if interp.faithful {
				if r, ok := interp.concreteConvert(val, inst.X.Type(), inst.Type(), site, interp.concrete); ok {
					interp.setLocal(frame, inst, r)
					break
				}
			}
			// Apply concrete type conversion for string↔bytes and numeric types (#74).
			interp.setLocal(frame, inst, convertValue(val, inst.X.Type(), inst.Type()))
			// Integer overflow on explicit conversion (#223): report when a
			// narrowing/sign conversion silently discards bits.
			if interp.config.TrackTruncation {
				if orig, result, truncated := intTruncationInfo(val, inst.X.Type(), inst.Type()); truncated {
					interp.recordViolation(gid, &shadow.IntegerTruncationError{
						Site:     site,
						GID:      gid,
						SrcType:  inst.X.Type().String(),
						DstType:  inst.Type().String(),
						Original: orig,
						Result:   result,
					})
				}
			}
		}

	case *ssa.ChangeType:
		val := interp.resolveValue(frame, inst.X)
		interp.setLocal(frame, inst, val)

	case *ssa.MakeInterface:
		val := interp.resolveValue(frame, inst.X)
		iface := &InterfaceValue{Type: inst.X.Type(), Value: val}
		interp.setLocal(frame, inst, Value{Raw: iface, Provenance: val.Provenance})

	case *ssa.MultiConvert:
		val := interp.resolveValue(frame, inst.X)
		interp.setLocal(frame, inst, val)

	// --- Phi nodes ---

	case *ssa.Phi:
		// The incoming value was computed with the block's other phis on
		// block entry (execBlock).
		if frame.phiNext < len(frame.phiVals) {
			interp.setLocal(frame, inst, frame.phiVals[frame.phiNext])
			frame.phiNext++
		} else {
			interp.setLocal(frame, inst, interp.phiIncoming(frame, inst))
		}

	case *ssa.RunDefers:
		// Run deferred calls before the return sequence reads named results,
		// so a deferred write to a named result is returned (#259).
		interp.runFrameDefers(gid, frame)

	// --- SSA bookkeeping we pass through ---

	case *ssa.DebugRef:
		// No-op

	default:
		// Unhandled instruction type — record for debugging
		if interp.config.Verbose {
			fmt.Printf("[giri] unhandled SSA instruction: %T at %s\n", instr, site)
		}
	}

	return nil // Continue to next instruction in block
}

// gcSafepoint checks for pending uintptr conversions that a GC at a call
// site would invalidate (unsafe.Pointer rule 2). A value-cache hit on a call
// must run it too: the call's result can come from the cache, but the
// safepoint is a property of reaching the call site, not of the result.
func (interp *Interpreter) gcSafepoint(gid int64, site string) {
	if interp.registry != nil {
		for _, err := range interp.registry.CheckGCPoint(site) {
			interp.recordViolation(gid, err)
		}
	}
}

// execCall interprets a function call instruction.
func (interp *Interpreter) execCall(gid int64, callerFn *ssa.Function, call *ssa.Call, site string) Value {
	frame := interp.currentFrame(gid)

	// Resolve arguments
	var args []Value
	for _, arg := range call.Call.Args {
		args = append(args, interp.resolveValue(frame, arg))
	}

	// Handle SSA builtins (unsafe.Add, len, cap, append, etc.) — not GC points.
	if b, ok := call.Call.Value.(*ssa.Builtin); ok {
		if interp.faithful {
			if v, ok := interp.concreteBuiltin(gid, b.Name(), &call.Call, args, site, interp.concrete); ok {
				return v
			}
		}
		return interp.execBuiltin(gid, b, args, site)
	}

	// Handle closure calls and other dynamic function values.
	if !call.Call.IsInvoke() {
		calleeVal := interp.resolveValue(frame, call.Call.Value)
		if cv, ok := calleeVal.Raw.(*ClosureValue); ok {
			allArgs := append(args, cv.FreeVars...)
			return interp.execFunction(gid, cv.Fn, allArgs)
		}
		// Concrete mode: a plain function value (not a closure) called dynamically.
		if f, ok := calleeVal.Raw.(*ssa.Function); ok && interp.concrete && call.Call.StaticCallee() == nil && f.Blocks != nil {
			return interp.execFunction(gid, f, args)
		}
		// Intercept context cancel functions (#120): calling cancel() removes
		// the entry from the outstanding set, suppressing the leak report.
		if cfID, ok := calleeVal.Raw.(cancelFuncID); ok {
			interp.callCancelFunc(cfID)
			return Value{}
		}
	}

	// Handle interface method dispatch (invoke calls: v.Method(args...)).
	// call.Call.Value is the interface receiver; call.Call.Method is the selector.
	if call.Call.IsInvoke() {
		recv := interp.resolveValue(frame, call.Call.Value)
		if iv, ok := recv.Raw.(*InterfaceValue); ok && iv.Type != nil && interp.prog != nil {
			m := call.Call.Method
			fn := interp.prog.LookupMethod(iv.Type, m.Pkg(), m.Name())
			if fn != nil && fn.Blocks != nil {
				// Receiver is prepended to args (SSA invoke convention).
				allArgs := append([]Value{iv.Value}, args...)
				return interp.execFunction(gid, fn, allArgs)
			}
			// Method resolved but assembly-backed (Blocks == nil): unmodeled.
			if fn != nil {
				if pkg := fn.Package(); pkg != nil && pkg.Pkg != nil {
					interp.recordUnmodeled(pkg.Pkg.Path(), fn.Name())
				}
			}
		}
		return Value{} // Unknown concrete type; fall through as external call.
	}

	// Non-builtin function call is a potential GC safepoint.
	interp.gcSafepoint(gid, site)

	calleeName := interp.callTargetName(call.Call)

	// --- Intercept known functions ---

	// arena.NewArena()
	if strings.Contains(calleeName, "arena.NewArena") {
		arenaID := interp.Memory.CreateArena(site)
		return Value{Raw: arenaID}
	}

	// arena.New[T](a)
	if strings.Contains(calleeName, "arena.New[") && len(args) > 0 {
		typeName := extractGenericType(calleeName)
		result, err := interp.handleArenaNew(gid, args[0], typeName, site)
		if err != nil {
			interp.recordViolation(gid, err)
		}
		return result
	}

	// arena.Free() / a.Free()
	if strings.HasSuffix(calleeName, ".Free") && len(args) > 0 {
		arenaID, ok := interp.resolveArenaID(args[0])
		if ok {
			errs := interp.Memory.FreeArena(arenaID, site)
			for _, err := range errs {
				interp.recordViolation(gid, err)
			}
		}
		return Value{}
	}

	// --- General function call: interpret the callee ---

	callee := call.Call.StaticCallee()

	// Intercept os.Exit(n) (#62): mark all goroutines as finished to halt
	// interpretation cleanly. Without this, the interpreter tries to execute
	// syscall-backed stdlib code and continues running past the exit point.
	if callee != nil && callee.Package() != nil {
		if pkg := callee.Package().Pkg; pkg != nil && pkg.Path() == "os" && callee.Name() == "Exit" {
			interp.haltAll()
			return Value{}
		}
	}

	// Intercept sync.Mutex and sync.WaitGroup before trying to execute them (#33).
	// Their implementations use futexes that can't be interpreted; we model the
	// clock semantics directly in handleSyncCall.
	if callee != nil && callee.Package() != nil {
		if pkg := callee.Package().Pkg; pkg != nil && pkg.Path() == "sync" {
			return interp.handleSyncCall(gid, callee, args, site)
		}
	}

	// Intercept reflect.Value.Pointer() and reflect.Value.UnsafeAddr() (Rule 5).
	// Both methods return a uintptr that must be converted back to unsafe.Pointer
	// before the next GC safepoint. Record the pending conversion so CheckGCPoint
	// fires if a function call separates the reflect call from the conversion.
	if callee != nil && callee.Package() != nil && interp.config.TrackUnsafe {
		if pkg := callee.Package().Pkg; pkg != nil && pkg.Path() == "reflect" {
			name := callee.Name()
			if name == "Pointer" || name == "UnsafeAddr" {
				if interp.registry != nil {
					interp.registry.RecordReflectConversion(call.Name(), site, nil)
				}
				return Value{Raw: int64(0)}
			}
		}
	}

	// Intercept stdlib calls (#42, #43, #65-#68).
	// These packages use reflect, runtime, and sync primitives that the
	// interpreter cannot fully execute; we model their semantics directly.
	// The check must precede execFunction so it fires even when source is loaded.
	//
	// For generic instantiations (e.g. slices.Contains[[]int int]), callee.Package()
	// is nil — the package and base name live on callee.Origin() (#149-#152).
	// We strip the type-parameter suffix via genericBaseName so the intercept
	// switch sees "Contains" rather than "Contains[[]int int]".
	if callee != nil {
		interceptPkg := callee.Package()
		interceptName := callee.Name()
		if interceptPkg == nil {
			if origin := callee.Origin(); origin != nil {
				interceptPkg = origin.Package()
				interceptName = origin.Name()
			}
		}
		if interceptPkg != nil {
			if pkg := interceptPkg.Pkg; pkg != nil {
				// The callee's result tuple lets the smart fallback shape an
				// auto-generated stub for unmodeled calls (#225). For generic
				// instantiations callee.Signature is still populated, but prefer
				// Origin's when the callee's is absent, mirroring the pkg/name
				// resolution above.
				var results *types.Tuple
				if sig := callee.Signature; sig != nil {
					results = sig.Results()
				} else if origin := callee.Origin(); origin != nil && origin.Signature != nil {
					results = origin.Signature.Results()
				}
				// Concrete mode uses the native-backed table for every concrete
				// function. The CLI uses it for sort, whose slice argument
				// lives in the element store (#260); elsewhere the CLI keeps
				// its own intercepts, which tolerate opaque arguments.
				if impl := concreteCallee(callee); impl != nil && (interp.concrete || (interp.faithful && pkg.Path() == "sort" && sortArgsConcrete(args))) {
					return impl.call(interp, gid, args)
				}
				if result, ok := interp.execStdlibCall(gid, site, pkg.Path(), interceptName, args, results); ok {
					return result
				}
			}
		}
	}

	if callee != nil && callee.Blocks != nil {
		// Record cross-package calls that had no stdlib intercept as "unmodeled".
		// Intra-package calls (e.g., stdlib helpers within the same package) are
		// excluded to avoid noise from recursively-interpreted stdlib internals.
		// (We reach here only when execStdlibCall returned false above.)
		calleePkg := ""
		calleeFn := callee.Name()
		if pkg := callee.Package(); pkg != nil && pkg.Pkg != nil {
			calleePkg = pkg.Pkg.Path()
		} else if origin := callee.Origin(); origin != nil {
			if pkg := origin.Package(); pkg != nil && pkg.Pkg != nil {
				calleePkg = pkg.Pkg.Path()
				calleeFn = origin.Name()
			}
		}
		if calleePkg != "" {
			callerPkg := ""
			if callerFn != nil {
				if fp := callerFn.Package(); fp != nil && fp.Pkg != nil {
					callerPkg = fp.Pkg.Path()
				}
			}
			if calleePkg != callerPkg {
				interp.recordUnmodeled(calleePkg, calleeFn)
			}
		}
		return interp.execFunction(gid, callee, args)
	}

	// External function — no interpretable SSA body and no intercept handled it.
	// Record the call so users can see coverage gaps via RunResult.UnmodeledCalls.
	if callee != nil {
		var pkgPath, funcName string
		if pkg := callee.Package(); pkg != nil && pkg.Pkg != nil {
			pkgPath = pkg.Pkg.Path()
			funcName = callee.Name()
		} else if origin := callee.Origin(); origin != nil {
			if pkg := origin.Package(); pkg != nil && pkg.Pkg != nil {
				pkgPath = pkg.Pkg.Path()
				funcName = origin.Name()
			}
		}
		interp.recordUnmodeled(pkgPath, funcName)
	}
	return Value{}
}

// execBuiltin interprets SSA builtin function calls (len, cap, unsafe.Add, etc.)
func (interp *Interpreter) execBuiltin(gid int64, b *ssa.Builtin, args []Value, site string) Value {
	switch b.Name() {
	case "Add": // unsafe.Add(ptr, offset) — pointer arithmetic
		if len(args) >= 2 && args[0].Provenance != nil {
			offset := int(toInt64(args[1]))
			derived := interp.Memory.DerivePointer(args[0].Provenance, offset)
			result := Value{Raw: derived, Provenance: derived}
			// Check bounds
			alloc, exists := interp.Memory.GetAllocation(derived.Alloc)
			if exists && (derived.Offset < 0 || derived.Offset > alloc.Size) {
				interp.recordViolation(gid, &shadow.UnsafePointerViolation{
					Rule:    shadow.RuleArithmetic,
					Site:    site,
					Details: fmt.Sprintf("unsafe.Add moved pointer to offset %d, allocation is %d bytes", derived.Offset, alloc.Size),
				})
			}
			return result
		}

	case "len":
		if len(args) > 0 {
			switch sv := args[0].Raw.(type) {
			case *SliceValue:
				return Value{Raw: int64(sv.Len)}
			case string:
				return Value{Raw: int64(len(sv))}
			case map[interface{}]Value: // map length (#138)
				return Value{Raw: int64(len(sv))}
			case ChanID: // buffered channel: pending item count (#138)
				if ch, ok := interp.channels[sv]; ok && ch.capacity > 0 {
					return Value{Raw: int64(ch.pendingCount)}
				}
				return Value{Raw: int64(0)}
			}
		}

	case "cap":
		if len(args) > 0 {
			switch sv := args[0].Raw.(type) {
			case *SliceValue:
				return Value{Raw: int64(sv.Cap)}
			case ChanID: // buffered channel: buffer capacity (#138)
				if ch, ok := interp.channels[sv]; ok {
					return Value{Raw: int64(ch.capacity)}
				}
				return Value{Raw: int64(0)}
			}
		}

	case "real":
		if len(args) > 0 {
			if c, ok := args[0].Raw.(complex128); ok {
				return Value{Raw: real(c)}
			}
		}

	case "imag":
		if len(args) > 0 {
			if c, ok := args[0].Raw.(complex128); ok {
				return Value{Raw: imag(c)}
			}
		}

	case "complex":
		if len(args) >= 2 {
			r, _ := args[0].Raw.(float64)
			i, _ := args[1].Raw.(float64)
			return Value{Raw: complex(r, i)}
		}

	case "append":
		// Proper append implementation (#26): track slice growth and new allocation.
		if len(args) == 0 {
			return Value{}
		}
		base := args[0]
		sv, ok := base.Raw.(*SliceValue)
		if !ok {
			return base
		}
		numAppend := len(args) - 1
		if numAppend <= 0 {
			return base
		}
		newLen := sv.Len + numAppend
		if newLen <= sv.Cap {
			// In-place: return same backing with incremented length
			return Value{Raw: arenaNew(interp.arena, SliceValue{Backing: sv.Backing, Len: newLen, Cap: sv.Cap}), Provenance: sv.Backing}
		}
		// Reallocation: allocate new backing array
		newCap := sv.Cap * 2
		if newCap < newLen {
			newCap = newLen * 2
		}
		elemSize := 8 // conservative default
		allocID := interp.Memory.Allocate(shadow.AllocHeap, newCap*elemSize, "append-backing", site)
		newPtr := arenaNew(interp.arena, shadow.Pointer{Alloc: allocID, Offset: 0})
		return Value{Raw: arenaNew(interp.arena, SliceValue{Backing: newPtr, Len: newLen, Cap: newCap}), Provenance: newPtr}

	case "copy":
		// Proper copy implementation (#27): compute count, trigger store for bounds check.
		if len(args) < 2 {
			return Value{Raw: int64(0)}
		}
		dst, src := args[0], args[1]
		dstSlice, dstOk := dst.Raw.(*SliceValue)
		srcSlice, srcOk := src.Raw.(*SliceValue)
		n := 0
		if dstOk && srcOk {
			n = dstSlice.Len
			if srcSlice.Len < n {
				n = srcSlice.Len
			}
			// Trigger a store to the destination for bounds/race checking
			if n > 0 && dstSlice.Backing != nil {
				err := interp.handleStore(gid, Value{Raw: dstSlice.Backing, Provenance: dstSlice.Backing}, Value{}, n, site)
				if err != nil {
					interp.recordViolation(gid, err)
				}
			}
		} else if dstOk {
			// copy(dst, string)
			if sv, ok := src.Raw.(string); ok {
				n = len([]byte(sv))
				if dstSlice.Len < n {
					n = dstSlice.Len
				}
			}
		}
		return Value{Raw: int64(n)}

	case "delete":
		// Map delete (#63): nil-map check, race tracking, and key removal.
		if len(args) >= 1 {
			m := args[0]
			// Nil map: delete from nil map panics at runtime.
			if m.Raw == nil {
				interp.recordViolation(gid, &shadow.NilMapWriteError{Site: site, GID: gid})
				break
			}
			// Race check: deletion is a write operation on the map.
			if m.Provenance != nil {
				if werr := interp.handleStore(gid, m, Value{}, 8, site); werr != nil {
					interp.recordViolation(gid, werr)
				}
			}
			// Remove the key from the interpreter map.
			if len(args) >= 2 {
				if mapVal, ok := m.Raw.(map[interface{}]Value); ok {
					delete(mapVal, toMapKey(args[1]))
				}
			}
		}

	case "min": // Go 1.21+ variadic min builtin (#69)
		if len(args) == 0 {
			return Value{}
		}
		result := args[0]
		for _, a := range args[1:] {
			switch v := result.Raw.(type) {
			case int64:
				if n, ok := a.Raw.(int64); ok && n < v {
					result = a
				}
			case float64:
				if n, ok := a.Raw.(float64); ok && n < v {
					result = a
				}
			case string:
				if s, ok := a.Raw.(string); ok && s < v {
					result = a
				}
			default:
				return Value{} // opaque arg — conservative
			}
		}
		return result

	case "max": // Go 1.21+ variadic max builtin (#69)
		if len(args) == 0 {
			return Value{}
		}
		result := args[0]
		for _, a := range args[1:] {
			switch v := result.Raw.(type) {
			case int64:
				if n, ok := a.Raw.(int64); ok && n > v {
					result = a
				}
			case float64:
				if n, ok := a.Raw.(float64); ok && n > v {
					result = a
				}
			case string:
				if s, ok := a.Raw.(string); ok && s > v {
					result = a
				}
			default:
				return Value{} // opaque arg — conservative
			}
		}
		return result

	case "clear": // Go 1.21+ clear builtin — map/slice reset (#69)
		if len(args) >= 1 {
			m := args[0]
			if m.Raw == nil {
				interp.recordViolation(gid, &shadow.NilMapWriteError{Site: site, GID: gid})
				break
			}
			// Race check: clearing is a write operation.
			if m.Provenance != nil {
				if werr := interp.handleStore(gid, m, Value{}, 8, site); werr != nil {
					interp.recordViolation(gid, werr)
				}
			}
			// Clear interpreter map.
			if mapVal, ok := m.Raw.(map[interface{}]Value); ok {
				for k := range mapVal {
					delete(mapVal, k)
				}
			}
			// Slice clear: element values are not individually tracked in our model, so no-op here.
			_ = m.Raw
		}

	case "close":
		// Channel close (#31): mark channel as closed; future sends will panic.
		// close(nil) panics at runtime: "close of nil channel" (#122).
		if len(args) > 0 {
			if args[0].Raw == nil {
				interp.recordViolation(gid, &shadow.NilChannelError{Op: "close", Site: site, GID: gid})
				interp.runtimePanic(gid, "close of nil channel")
				return Value{}
			}
			if chanID, ok := args[0].Raw.(ChanID); ok {
				interp.handleChannelClose(gid, chanID, site)
			}
		}
		return Value{}

	case "panic":
		// panic() — just stop execution of this goroutine

	case "recover":
		// recover() implementation (#34, #48): return the panic value if a panic
		// is in-flight (g.Panicking=true, set temporarily to false by popFrame
		// before running each defer). Signal recovery via g.Recovered so popFrame
		// stops unwinding after this defer returns.
		g := interp.goroutine(gid)
		if g != nil && !g.Panicking && g.PanicValue.Raw != nil {
			// popFrame cleared Panicking before calling this defer; the PanicValue
			// still set indicates we are in a defer during panic unwinding.
			v := g.PanicValue
			g.PanicValue = Value{}
			g.Recovered = true
			return v
		}
		return Value{}

	case "print", "println":
		// Ignore print output during interpretation

	case "String": // unsafe.String(ptr *byte, len) — create string from pointer + length (Go 1.20+)
		if len(args) < 2 {
			break
		}
		lenN := int(toInt64(args[1]))
		// Negative length panics at runtime (#131).
		if lenN < 0 {
			interp.recordViolation(gid, &shadow.InvalidUnsafeArgError{
				Op: "unsafe.String", Arg: "len", Value: int64(lenN), Site: site, GID: gid,
			})
			interp.runtimePanic(gid, "unsafe argument out of range")
			break
		}
		// Nil pointer with non-zero length panics at runtime (#131).
		if args[0].Provenance == nil && args[0].Raw == nil && lenN != 0 {
			interp.recordViolation(gid, &shadow.InvalidUnsafeArgError{
				Op: "unsafe.String", Arg: "ptr", Site: site, GID: gid,
			})
			interp.runtimePanic(gid, "invalid memory address or nil pointer dereference")
			break
		}
		// Valid: return an opaque string value.
		return Value{Raw: ""}

	case "Slice": // unsafe.Slice(ptr, len) — create slice from pointer + length
		if len(args) < 2 {
			break
		}
		lenN := int(toInt64(args[1]))
		// Negative length panics at runtime (#128).
		if lenN < 0 {
			interp.recordViolation(gid, &shadow.InvalidUnsafeArgError{
				Op: "unsafe.Slice", Arg: "len", Value: int64(lenN), Site: site, GID: gid,
			})
			interp.runtimePanic(gid, "unsafe argument out of range")
			break
		}
		// Nil pointer with non-zero length panics at runtime (#129).
		if args[0].Provenance == nil && lenN != 0 {
			interp.recordViolation(gid, &shadow.InvalidUnsafeArgError{
				Op: "unsafe.Slice", Arg: "ptr", Site: site, GID: gid,
			})
			interp.runtimePanic(gid, "invalid memory address or nil pointer dereference")
			break
		}
		if args[0].Provenance != nil {
			sv := arenaNew(interp.arena, SliceValue{Backing: args[0].Provenance, Len: lenN, Cap: lenN})
			return Value{Raw: sv, Provenance: args[0].Provenance}
		}

	case "SliceData": // unsafe.SliceData(slice) — pointer to backing array (Go 1.20+)
		// Returns &slice[0] for non-nil/non-empty slice, nil otherwise.
		if len(args) > 0 && args[0].Provenance != nil {
			return Value{Provenance: args[0].Provenance}
		}
		return Value{} // nil for nil or empty slice

	case "StringData": // unsafe.StringData(str) — pointer to string bytes (Go 1.20+)
		// Returns a pointer to the underlying bytes; conservative: return nil.
		// Strings are immutable; the pointer should not be used for writes.
		return Value{}
	}

	return Value{}
}

// phiIncoming returns the value phi takes on entry from frame.PrevBlock: the
// edge for that predecessor. If no predecessor matches, it takes the first
// edge unconditionally; SSA guarantees edge order matches predecessor order,
// and the entry block has no Phi nodes. (Skipping zero-valued edges instead
// gave wrong values in loops, #18.)
func (interp *Interpreter) phiIncoming(frame *Frame, phi *ssa.Phi) Value {
	for i, pred := range phi.Block().Preds {
		if pred == frame.PrevBlock {
			return interp.resolveValue(frame, phi.Edges[i])
		}
	}
	if len(phi.Edges) > 0 {
		return interp.resolveValue(frame, phi.Edges[0])
	}
	return Value{}
}

// --- Value Resolution ---

// resolveValue looks up an SSA value in the current frame's locals,
// or creates a Value from a constant.
func (interp *Interpreter) resolveValue(frame *Frame, v ssa.Value) Value {
	if frame == nil {
		return Value{}
	}

	// Check locals first
	if val, ok := interp.getLocal(frame, v); ok {
		return val
	}

	// Constants
	if c, ok := v.(*ssa.Const); ok {
		if interp.concrete {
			return concreteConst(c)
		}
		return interp.constToValue(c)
	}

	// Global: look up from the pre-initialized globals table.
	// The global's value is always a pointer (globals are always address-taken in SSA).
	if g, ok := v.(*ssa.Global); ok {
		if interp.globals != nil {
			if val, found := interp.globals[g]; found {
				return val
			}
		}
		// Sessions allocate globals on first use instead of up front.
		if interp.cs != nil {
			return interp.sessionGlobal(g)
		}
		// Fallback for globals not yet registered (shouldn't happen after Run init)
		return Value{Raw: g.Name()}
	}

	// Function value — return the SSA Function itself so it can be called
	// (e.g., as an argument to sync.Once.Do or passed as a callback).
	if f, ok := v.(*ssa.Function); ok {
		return Value{Raw: f}
	}

	return Value{}
}

// constToValue converts an SSA constant to an interpreted Value using go/constant.
func (interp *Interpreter) constToValue(c *ssa.Const) Value {
	if c.Value == nil {
		return Value{Raw: nil} // nil constant
	}
	switch c.Value.Kind() {
	case constant.Bool:
		return Value{Raw: constant.BoolVal(c.Value)}
	case constant.Int:
		v, _ := constant.Int64Val(c.Value)
		return Value{Raw: v}
	case constant.Float:
		v, _ := constant.Float64Val(c.Value)
		return Value{Raw: v}
	case constant.String:
		return Value{Raw: constant.StringVal(c.Value)}
	case constant.Complex:
		re, _ := constant.Float64Val(constant.Real(c.Value))
		im, _ := constant.Float64Val(constant.Imag(c.Value))
		return Value{Raw: complex(re, im)}
	}
	return Value{Raw: c.Value.String()}
}

// --- Helpers ---

// posString converts a token.Pos to "file:line" string.
func (interp *Interpreter) posString(pos token.Pos) string {
	if !pos.IsValid() || interp.Fset == nil {
		return "<unknown>"
	}
	p := interp.Fset.Position(pos)
	return fmt.Sprintf("%s:%d", p.Filename, p.Line)
}

// callTargetName extracts the function name from a CallCommon.
func (interp *Interpreter) callTargetName(call ssa.CallCommon) string {
	if callee := call.StaticCallee(); callee != nil {
		return callee.String()
	}
	if call.Method != nil {
		return call.Method.Name()
	}
	return "<dynamic>"
}

// isUnsafePointerConversion checks if a Convert instruction involves unsafe.Pointer.
func isUnsafePointerConversion(inst *ssa.Convert) bool {
	srcType := inst.X.Type().String()
	dstType := inst.Type().String()
	return strings.Contains(srcType, "unsafe.Pointer") ||
		strings.Contains(dstType, "unsafe.Pointer")
}

// classifyUnsafeConversion determines which unsafe.Pointer rule applies.
func classifyUnsafeConversion(inst *ssa.Convert) UnsafeOp {
	srcType := inst.X.Type().String()
	dstType := inst.Type().String()

	if strings.Contains(dstType, "unsafe.Pointer") && !strings.Contains(srcType, "uintptr") {
		return UnsafeOpToPointer
	}
	if strings.Contains(srcType, "unsafe.Pointer") && strings.Contains(dstType, "uintptr") {
		return UnsafeOpToUintptr
	}
	if strings.Contains(srcType, "uintptr") && strings.Contains(dstType, "unsafe.Pointer") {
		return UnsafeOpArithmetic
	}
	return UnsafeOpFromPointer
}

// extractGenericType pulls the type parameter from "arena.New[T]".
func extractGenericType(name string) string {
	start := strings.Index(name, "[")
	end := strings.Index(name, "]")
	if start >= 0 && end > start {
		return name[start+1 : end]
	}
	return "unknown"
}

// evalBinOp evaluates a binary operation on two interpreted Values.
func evalBinOp(op token.Token, x, y Value) Value {
	// Integer arithmetic
	xi, xIsInt := x.Raw.(int64)
	yi, yIsInt := y.Raw.(int64)
	if xIsInt && yIsInt {
		switch op {
		case token.ADD:
			return Value{Raw: xi + yi}
		case token.SUB:
			return Value{Raw: xi - yi}
		case token.MUL:
			return Value{Raw: xi * yi}
		case token.QUO:
			if yi == 0 {
				return Value{Raw: int64(0)}
			}
			return Value{Raw: xi / yi}
		case token.REM:
			if yi == 0 {
				return Value{Raw: int64(0)}
			}
			return Value{Raw: xi % yi}
		case token.AND:
			return Value{Raw: xi & yi}
		case token.OR:
			return Value{Raw: xi | yi}
		case token.XOR:
			return Value{Raw: xi ^ yi}
		case token.AND_NOT:
			return Value{Raw: xi &^ yi}
		case token.SHL:
			return Value{Raw: xi << uint(yi)}
		case token.SHR:
			return Value{Raw: xi >> uint(yi)}
		case token.EQL:
			return Value{Raw: xi == yi}
		case token.NEQ:
			return Value{Raw: xi != yi}
		case token.LSS:
			return Value{Raw: xi < yi}
		case token.LEQ:
			return Value{Raw: xi <= yi}
		case token.GTR:
			return Value{Raw: xi > yi}
		case token.GEQ:
			return Value{Raw: xi >= yi}
		}
	}

	// Float arithmetic
	xf, xIsFloat := x.Raw.(float64)
	yf, yIsFloat := y.Raw.(float64)
	if xIsFloat && yIsFloat {
		switch op {
		case token.ADD:
			return Value{Raw: xf + yf}
		case token.SUB:
			return Value{Raw: xf - yf}
		case token.MUL:
			return Value{Raw: xf * yf}
		case token.QUO:
			return Value{Raw: xf / yf}
		case token.EQL:
			return Value{Raw: xf == yf}
		case token.NEQ:
			return Value{Raw: xf != yf}
		case token.LSS:
			return Value{Raw: xf < yf}
		case token.LEQ:
			return Value{Raw: xf <= yf}
		case token.GTR:
			return Value{Raw: xf > yf}
		case token.GEQ:
			return Value{Raw: xf >= yf}
		}
	}

	// Complex arithmetic (#141)
	xc, xIsComplex := x.Raw.(complex128)
	yc, yIsComplex := y.Raw.(complex128)
	if xIsComplex && yIsComplex {
		switch op {
		case token.ADD:
			return Value{Raw: xc + yc}
		case token.SUB:
			return Value{Raw: xc - yc}
		case token.MUL:
			return Value{Raw: xc * yc}
		case token.QUO:
			return Value{Raw: xc / yc}
		case token.EQL:
			return Value{Raw: xc == yc}
		case token.NEQ:
			return Value{Raw: xc != yc}
		}
	}

	// Bool operations
	xb, xIsBool := x.Raw.(bool)
	yb, yIsBool := y.Raw.(bool)
	if xIsBool && yIsBool {
		switch op {
		case token.LAND:
			return Value{Raw: xb && yb}
		case token.LOR:
			return Value{Raw: xb || yb}
		case token.EQL:
			return Value{Raw: xb == yb}
		case token.NEQ:
			return Value{Raw: xb != yb}
		}
	}

	// String operations
	xs, xIsStr := x.Raw.(string)
	ys, yIsStr := y.Raw.(string)
	if xIsStr && yIsStr {
		switch op {
		case token.ADD:
			return Value{Raw: xs + ys}
		case token.EQL:
			return Value{Raw: xs == ys}
		case token.NEQ:
			return Value{Raw: xs != ys}
		case token.LSS:
			return Value{Raw: xs < ys}
		case token.LEQ:
			return Value{Raw: xs <= ys}
		case token.GTR:
			return Value{Raw: xs > ys}
		case token.GEQ:
			return Value{Raw: xs >= ys}
		}
	}

	return Value{} // Unhandled combination
}

// ifaceEqual compares two interface values for == on the CLI path (#259).
// A nil Raw is a nil interface; any other value, including the opaque
// placeholder intercepts return for non-error interfaces, is non-nil. Two
// boxed values are equal when their dynamic types are identical and their
// scalar payloads are equal. ok=false leaves the comparison to evalBinOp.
func ifaceEqual(x, y Value) (eq, ok bool) {
	switch {
	case x.Raw == nil && y.Raw == nil:
		return true, true
	case x.Raw == nil || y.Raw == nil:
		return false, true
	}
	xi, xok := x.Raw.(*InterfaceValue)
	yi, yok := y.Raw.(*InterfaceValue)
	if !xok || !yok {
		return false, false
	}
	if !types.Identical(xi.Type, yi.Type) {
		return false, true
	}
	if r, isBool := evalBinOp(token.EQL, xi.Value, yi.Value).Raw.(bool); isBool {
		return r, true
	}
	if xp, ok := xi.Value.Raw.(*shadow.Pointer); ok {
		if yp, ok := yi.Value.Raw.(*shadow.Pointer); ok {
			return xp.Alloc == yp.Alloc && xp.Offset == yp.Offset, true
		}
	}
	return false, false
}

// toInt64 converts an interpreted Value's Raw to int64.
func toInt64(v Value) int64 {
	switch n := v.Raw.(type) {
	case int64:
		return n
	case int:
		return int64(n)
	case int32:
		return int64(n)
	case uint64:
		return int64(n)
	case float64:
		return int64(n)
	case bool:
		if n {
			return 1
		}
		return 0
	}
	return 0
}

// deref dereferences a pointer type, returning the pointed-to type.
// If t is not a pointer, it is returned unchanged.
func deref(t types.Type) types.Type {
	if ptr, ok := t.Underlying().(*types.Pointer); ok {
		return ptr.Elem()
	}
	return t
}

// structFields returns all field variables of a struct type.
func structFields(st *types.Struct) []*types.Var {
	fields := make([]*types.Var, st.NumFields())
	for i := 0; i < st.NumFields(); i++ {
		fields[i] = st.Field(i)
	}
	return fields
}

// rangeIter holds iterator state for range over a collection.
type rangeIter struct {
	val      Value
	idx      int
	mapKeys  []interface{} // lazily populated for map iteration
	arrayLen int           // > 0 for range-over-array ([N]T or *[N]T); 0 otherwise (#137)
	sortKeys bool          // concrete mode: iterate map keys in sorted order
}

// advance returns the next (ok, key, value) triple from the iterator.
func (ri *rangeIter) advance() (bool, Value, Value) {
	// Array iteration (#137): arrayLen > 0 means we're ranging over [N]T or *[N]T.
	// The element value is loaded by the loop body via ssa.Index/IndexAddr.
	if ri.arrayLen > 0 {
		if ri.idx >= ri.arrayLen {
			return false, Value{}, Value{}
		}
		k := Value{Raw: int64(ri.idx)}
		ri.idx++
		return true, k, Value{}
	}
	switch sv := ri.val.Raw.(type) {
	case *SliceValue:
		if ri.idx >= sv.Len {
			return false, Value{}, Value{}
		}
		k := Value{Raw: int64(ri.idx)}
		ri.idx++
		return true, k, Value{}
	case string:
		// for i, r := range s yields byte offsets as i (#73).
		if ri.idx >= len(sv) {
			return false, Value{}, Value{}
		}
		r, size := utf8.DecodeRuneInString(sv[ri.idx:])
		k := Value{Raw: int64(ri.idx)}
		v := Value{Raw: int64(r)}
		ri.idx += size // advance by byte width of the rune
		return true, k, v
	case map[interface{}]Value:
		// Lazily collect map keys on first advance so iteration order is stable.
		if ri.mapKeys == nil {
			ri.mapKeys = make([]interface{}, 0, len(sv))
			for mk := range sv {
				ri.mapKeys = append(ri.mapKeys, mk)
			}
			if ri.sortKeys {
				sortMapKeys(ri.mapKeys)
			}
		}
		if ri.idx >= len(ri.mapKeys) {
			return false, Value{}, Value{}
		}
		mk := ri.mapKeys[ri.idx]
		ri.idx++
		return true, valueFromMapKey(mk), sv[mk]
	}
	return false, Value{}, Value{}
}

// toMapKey converts an interpreted Value to a comparable interface{} map key.
func toMapKey(v Value) interface{} {
	if v.Raw == nil {
		return nil
	}
	switch r := v.Raw.(type) {
	case int64, float64, bool, string:
		return r
	}
	return fmt.Sprintf("%v", v.Raw)
}

// valueFromMapKey converts a map key back to an interpreted Value.
func valueFromMapKey(mk interface{}) Value {
	switch k := mk.(type) {
	case int64:
		return Value{Raw: k}
	case float64:
		return Value{Raw: k}
	case bool:
		return Value{Raw: k}
	case string:
		return Value{Raw: k}
	}
	return Value{Raw: fmt.Sprintf("%v", mk)}
}

// convertValue applies a Go type conversion to a Value (#74).
// Handles the three common non-unsafe conversion patterns that would otherwise
// silently produce wrong values:
//   - integer / rune → string  (string(65) = "A")
//   - string → []byte           ([]byte("hi") = {0x68, 0x69})
//   - []byte → string           (string([]byte{0x68,0x69}) = "hi")
//
// For numeric conversions between integer types, Go semantics are emulated:
//   - float → int  (truncation)
//   - int → float  (exact promotion)
//
// All other conversions pass the value through unchanged (safe for our model).
func convertValue(v Value, srcType, dstType types.Type) Value {
	srcUnderlying := srcType.Underlying()
	dstUnderlying := dstType.Underlying()

	srcBasic, srcIsBasic := srcUnderlying.(*types.Basic)
	dstBasic, dstIsBasic := dstUnderlying.(*types.Basic)

	// integer / rune → string: string(65) = "A", string('€') = "€"
	if srcIsBasic && dstIsBasic && dstBasic.Kind() == types.String {
		if srcBasic.Info()&types.IsInteger != 0 {
			return Value{Raw: string(rune(toInt64(v)))}
		}
	}

	// string → []byte or string → []rune (#142)
	if srcIsBasic && srcBasic.Kind() == types.String {
		if dstSlice, ok := dstUnderlying.(*types.Slice); ok {
			if elem, ok := dstSlice.Elem().Underlying().(*types.Basic); ok {
				if s, ok := v.Raw.(string); ok {
					switch elem.Kind() {
					case types.Uint8: // []byte
						return Value{Raw: []byte(s)}
					case types.Int32: // []rune
						runes := []rune(s)
						vals := make([]Value, len(runes))
						for i, r := range runes {
							vals[i] = Value{Raw: int64(r)}
						}
						return Value{Raw: vals}
					}
				}
			}
		}
	}

	// []byte → string or []rune → string (#142)
	if srcSlice, ok := srcUnderlying.(*types.Slice); ok {
		if elem, ok := srcSlice.Elem().Underlying().(*types.Basic); ok {
			if dstIsBasic && dstBasic.Kind() == types.String {
				switch elem.Kind() {
				case types.Uint8: // []byte → string
					switch b := v.Raw.(type) {
					case []byte:
						return Value{Raw: string(b)}
					case []Value:
						bs := make([]byte, len(b))
						for i, bv := range b {
							bs[i] = byte(toInt64(bv))
						}
						return Value{Raw: string(bs)}
					}
				case types.Int32: // []rune → string
					if rv, ok := v.Raw.([]Value); ok {
						runes := make([]rune, len(rv))
						for i, r := range rv {
							runes[i] = rune(toInt64(r))
						}
						return Value{Raw: string(runes)}
					}
				}
			}
		}
	}

	// Numeric conversions between basic types.
	if srcIsBasic && dstIsBasic {
		// float → int truncation
		if srcBasic.Info()&types.IsFloat != 0 && dstBasic.Info()&types.IsInteger != 0 {
			if f, ok := v.Raw.(float64); ok {
				return Value{Raw: int64(f)}
			}
		}
		// int → float promotion
		if srcBasic.Info()&types.IsInteger != 0 && dstBasic.Info()&types.IsFloat != 0 {
			return Value{Raw: float64(toInt64(v))}
		}
		// int → complex: complex128(42) = 42+0i (#144)
		if srcBasic.Info()&types.IsInteger != 0 && dstBasic.Info()&types.IsComplex != 0 {
			return Value{Raw: complex(float64(toInt64(v)), 0)}
		}
		// float → complex: complex128(3.14) = 3.14+0i (#144)
		if srcBasic.Info()&types.IsFloat != 0 && dstBasic.Info()&types.IsComplex != 0 {
			if f, ok := v.Raw.(float64); ok {
				return Value{Raw: complex(f, 0)}
			}
		}
		// int → int: apply bit-width truncation/sign-extension (#139).
		// Without this, int8(300) returns 300 instead of 44, causing incorrect
		// branch decisions in programs relying on Go's well-defined wrap-around.
		if srcBasic.Info()&types.IsInteger != 0 && dstBasic.Info()&types.IsInteger != 0 {
			n := toInt64(v)
			switch dstBasic.Kind() {
			case types.Int8:
				return Value{Raw: int64(int8(n))}
			case types.Uint8:
				return Value{Raw: int64(uint8(n))}
			case types.Int16:
				return Value{Raw: int64(int16(n))}
			case types.Uint16:
				return Value{Raw: int64(uint16(n))}
			case types.Int32:
				return Value{Raw: int64(int32(n))}
			case types.Uint32:
				return Value{Raw: int64(uint32(n))}
			case types.Int64, types.Int:
				return Value{Raw: n}
			case types.Uint64, types.Uint, types.Uintptr:
				return Value{Raw: int64(uint64(n))} // bit pattern preserved as int64
			}
		}
	}

	return v // all other conversions pass through unchanged
}

// intTruncationInfo reports whether an explicit integer→integer conversion
// discards significant bits (does not round-trip). It returns the original and
// converted values (as int64) and truncated=true only when v carries a
// concrete integer whose value changes under the conversion — Go's defined
// wrap-around. Non-integer conversions, opaque/absent values, and value-
// preserving conversions return truncated=false.
func intTruncationInfo(v Value, srcType, dstType types.Type) (orig, result int64, truncated bool) {
	srcBasic, ok := srcType.Underlying().(*types.Basic)
	if !ok || srcBasic.Info()&types.IsInteger == 0 {
		return 0, 0, false
	}
	dstBasic, ok := dstType.Underlying().(*types.Basic)
	if !ok || dstBasic.Info()&types.IsInteger == 0 {
		return 0, 0, false
	}
	// Only meaningful when the source value is a concrete integer. toInt64
	// coerces absent/opaque values to 0, which would be indistinguishable from
	// a real 0 — so require an integer Raw kind explicitly.
	switch v.Raw.(type) {
	case int64, int, int32, uint64:
	default:
		return 0, 0, false
	}
	orig = toInt64(v)
	converted := convertValue(v, srcType, dstType)
	result = toInt64(converted)
	return orig, result, orig != result
}

// typeAssertSucceeds reports whether a type assertion from concreteType to
// assertedType would succeed at runtime.
func typeAssertSucceeds(concreteType, assertedType types.Type) bool {
	if types.Identical(concreteType, assertedType) {
		return true
	}
	if iface, ok := assertedType.Underlying().(*types.Interface); ok {
		// Check if the concrete type (or its pointer form) implements the interface.
		return types.Implements(concreteType, iface) ||
			types.Implements(types.NewPointer(concreteType), iface)
	}
	return types.AssignableTo(concreteType, assertedType)
}
