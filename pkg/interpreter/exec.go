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
	"fmt"
	"go/constant"
	"go/token"
	"go/types"
	"strings"

	"golang.org/x/tools/go/ssa"

	"github.com/scttfrdmn/giri/pkg/shadow"
)

// Program represents a loaded Go program ready for interpretation.
type Program struct {
	SSA  *ssa.Program
	Main *ssa.Package
	Fset *token.FileSet
}

// RunResult holds the results of interpreting a program.
type RunResult struct {
	Violations []error
	Stats      ExecutionStats
	MemStats   shadow.MemoryStats
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
func Run(prog *Program, config Config) *RunResult {
	interp := New(prog.Fset, config)

	// Create the main goroutine
	mainGID, err := interp.spawnGoroutine("main", "entry")
	if err != nil {
		return &RunResult{Violations: []error{err}}
	}

	// Find and execute main function
	mainFn := prog.Main.Func("main")
	if mainFn == nil {
		mainFn = prog.Main.Func("init")
	}

	if mainFn != nil {
		interp.execFunction(mainGID, mainFn, nil)
	}

	// Drain the run queue: execute spawned goroutines via scheduler
	for len(interp.runQueue) > 0 {
		runnable := make([]int64, len(interp.runQueue))
		for i, t := range interp.runQueue {
			runnable[i] = t.gid
		}
		nextGID := interp.sched.Next(runnable)
		for i, t := range interp.runQueue {
			if t.gid == nextGID {
				interp.runQueue = append(interp.runQueue[:i], interp.runQueue[i+1:]...)
				interp.execFunction(t.gid, t.fn, t.args)
				break
			}
		}
	}

	// Run finalization checks
	finalErrs := interp.Finish()

	return &RunResult{
		Violations: finalErrs,
		MemStats:   interp.Memory.Stats(),
	}
}

// execFunction interprets a single SSA function.
func (interp *Interpreter) execFunction(gid int64, fn *ssa.Function, args []Value) Value {
	if fn == nil || fn.Blocks == nil {
		return Value{} // External function, can't interpret
	}

	site := interp.posString(fn.Pos())
	frame := interp.pushFrame(gid, fn.String(), site)
	defer interp.popFrame(gid)

	// Bind parameters
	for i, param := range fn.Params {
		if i < len(args) {
			frame.Locals[param.Name()] = args[i]
		}
	}

	// Bind free variables (closures): callers append them after regular params.
	// See ClosureValue and the execCall/ssa.Go closure handling.
	freeVarStart := len(fn.Params)
	for i, fv := range fn.FreeVars {
		if freeVarStart+i < len(args) {
			frame.Locals[fv.Name()] = args[freeVarStart+i]
		}
	}

	// Execute blocks, tracking the previous block for Phi node resolution.
	// Stop early if the goroutine is halted (panic or step limit).
	var prevBlock *ssa.BasicBlock
	block := fn.Blocks[0]
	for block != nil {
		g := interp.goroutines[gid]
		if g != nil && g.Panicked {
			return Value{}
		}
		if f := interp.currentFrame(gid); f != nil {
			f.PrevBlock = prevBlock
		}
		nextBlock := interp.execBlock(gid, fn, block)
		prevBlock = block
		block = nextBlock
	}

	// Return the result (if any)
	if retVal, ok := frame.Locals["__return__"]; ok {
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

	for _, instr := range block.Instrs {
		// Check for panic/halt between instructions
		if g := interp.goroutines[gid]; g != nil && g.Panicked {
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
	// Enforce execution step limit (#17)
	if interp.config.MaxSteps > 0 {
		interp.steps++
		if interp.steps > interp.config.MaxSteps {
			g := interp.goroutines[gid]
			if g != nil && !g.Panicked {
				interp.recordViolation(fmt.Errorf(
					"execution limit of %d steps exceeded at %s",
					interp.config.MaxSteps, interp.posString(instr.Pos()),
				))
				g.Panicked = true
			}
			return nil
		}
	}

	site := interp.posString(instr.Pos())
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
		allocID := interp.Memory.Allocate(kind, size, typeName, site)
		ptr := &shadow.Pointer{Alloc: allocID, Offset: 0}
		frame.Locals[inst.Name()] = Value{Raw: ptr, Provenance: ptr}

	case *ssa.Store:
		addr := interp.resolveValue(frame, inst.Addr)
		val := interp.resolveValue(frame, inst.Val)
		size := interp.typeSizeOf(inst.Val.Type())
		if err := interp.handleStore(gid, addr, val, size, site); err != nil {
			interp.recordViolation(err)
		}

	case *ssa.UnOp:
		operand := interp.resolveValue(frame, inst.X)
		switch inst.Op {
		case token.MUL: // Dereference (load)
			size := interp.typeSizeOf(inst.Type())
			result, err := interp.handleLoad(gid, operand, size, site)
			if err != nil {
				interp.recordViolation(err)
			}
			frame.Locals[inst.Name()] = result
		case token.ARROW: // Channel receive (<-ch)
			var chanID ChanID
			if id, ok := operand.Raw.(ChanID); ok {
				chanID = id
			}
			interp.handleChannelRecv(gid, chanID, site)
			if inst.CommaOk {
				frame.Locals[inst.Name()] = Value{Raw: []Value{{}, {Raw: true}}}
			} else {
				frame.Locals[inst.Name()] = Value{}
			}
		case token.SUB:
			if v, ok := operand.Raw.(int64); ok {
				frame.Locals[inst.Name()] = Value{Raw: -v}
			} else if v, ok := operand.Raw.(float64); ok {
				frame.Locals[inst.Name()] = Value{Raw: -v}
			} else {
				frame.Locals[inst.Name()] = operand
			}
		case token.NOT:
			if v, ok := operand.Raw.(bool); ok {
				frame.Locals[inst.Name()] = Value{Raw: !v}
			} else {
				frame.Locals[inst.Name()] = operand
			}
		case token.XOR:
			if v, ok := operand.Raw.(int64); ok {
				frame.Locals[inst.Name()] = Value{Raw: ^v}
			} else {
				frame.Locals[inst.Name()] = operand
			}
		default:
			frame.Locals[inst.Name()] = operand
		}

	case *ssa.BinOp:
		x := interp.resolveValue(frame, inst.X)
		y := interp.resolveValue(frame, inst.Y)
		frame.Locals[inst.Name()] = evalBinOp(inst.Op, x, y)

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
		frame.Locals[inst.Name()] = result

	case *ssa.IndexAddr:
		base := interp.resolveValue(frame, inst.X)
		idx := interp.resolveValue(frame, inst.Index)
		indexVal := int(toInt64(idx))
		elemSize := 8
		switch t := inst.X.Type().Underlying().(type) {
		case *types.Pointer:
			if arr, ok := t.Elem().Underlying().(*types.Array); ok {
				elemSize = interp.typeSizeOf(arr.Elem())
			}
		case *types.Slice:
			elemSize = interp.typeSizeOf(t.Elem())
		}
		result := interp.handleIndexAddr(gid, base, indexVal, elemSize, site)
		frame.Locals[inst.Name()] = result

	case *ssa.Field:
		base := interp.resolveValue(frame, inst.X)
		if m, ok := base.Raw.(map[int]Value); ok {
			frame.Locals[inst.Name()] = m[inst.Field]
		} else {
			frame.Locals[inst.Name()] = Value{}
		}

	case *ssa.Index:
		// Array/slice/string index — conservative: return zero value
		frame.Locals[inst.Name()] = Value{}

	case *ssa.Extract:
		// Extract element from a multi-value tuple (e.g. multi-return)
		tuple := interp.resolveValue(frame, inst.Tuple)
		if elems, ok := tuple.Raw.([]Value); ok && inst.Index < len(elems) {
			frame.Locals[inst.Name()] = elems[inst.Index]
		} else {
			frame.Locals[inst.Name()] = Value{}
		}

	case *ssa.Lookup:
		// Map key lookup — return zero value for v0.2.0
		frame.Locals[inst.Name()] = Value{}

	case *ssa.MapUpdate:
		// Map update m[k] = v — no-op for safety checking purposes in v0.2.0

	case *ssa.MakeSlice:
		elemType := inst.Type().(*types.Slice).Elem()
		elemSize := interp.typeSizeOf(elemType)
		lenVal := interp.resolveValue(frame, inst.Len)
		capVal := interp.resolveValue(frame, inst.Cap)
		lenN := int(toInt64(lenVal))
		capN := int(toInt64(capVal))
		if capN < lenN {
			capN = lenN
		}
		allocSize := capN * elemSize
		if allocSize <= 0 {
			allocSize = elemSize
		}
		allocID := interp.Memory.Allocate(shadow.AllocHeap, allocSize, inst.Type().String(), site)
		ptr := &shadow.Pointer{Alloc: allocID, Offset: 0}
		sv := &SliceValue{Backing: ptr, Len: lenN, Cap: capN}
		frame.Locals[inst.Name()] = Value{Raw: sv, Provenance: ptr}

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
		if sv, ok := base.Raw.(*SliceValue); ok {
			if highVal < 0 {
				highVal = sv.Len
			}
			elemSize := 1
			if t, ok2 := inst.X.Type().Underlying().(*types.Slice); ok2 {
				elemSize = interp.typeSizeOf(t.Elem())
			}
			var newPtr *shadow.Pointer
			if sv.Backing != nil {
				newPtr = interp.Memory.DerivePointer(sv.Backing, lowVal*elemSize)
			}
			newSv := &SliceValue{
				Backing: newPtr,
				Len:     highVal - lowVal,
				Cap:     sv.Cap - lowVal,
			}
			frame.Locals[inst.Name()] = Value{Raw: newSv, Provenance: newPtr}
		} else {
			frame.Locals[inst.Name()] = base
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
		frame.Locals[inst.Name()] = Value{Raw: &ClosureValue{Fn: fn, FreeVars: freeVars}}

	case *ssa.MakeMap:
		frame.Locals[inst.Name()] = Value{Raw: make(map[interface{}]Value)}

	case *ssa.MakeChan:
		chanID := interp.createChannel()
		frame.Locals[inst.Name()] = Value{Raw: chanID}

	case *ssa.Range:
		base := interp.resolveValue(frame, inst.X)
		frame.Locals[inst.Name()] = Value{Raw: &rangeIter{val: base, idx: 0}}

	case *ssa.Next:
		iter := interp.resolveValue(frame, inst.Iter)
		if ri, ok := iter.Raw.(*rangeIter); ok {
			ok2, k, v := ri.advance()
			frame.Locals[inst.Name()] = Value{Raw: []Value{{Raw: ok2}, k, v}}
		} else {
			frame.Locals[inst.Name()] = Value{Raw: []Value{{Raw: false}, {}, {}}}
		}

	case *ssa.TypeAssert:
		val := interp.resolveValue(frame, inst.X)
		if inst.CommaOk {
			frame.Locals[inst.Name()] = Value{Raw: []Value{val, {Raw: true}}}
		} else {
			frame.Locals[inst.Name()] = val
		}

	case *ssa.ChangeInterface:
		val := interp.resolveValue(frame, inst.X)
		frame.Locals[inst.Name()] = val

	case *ssa.SliceToArrayPointer:
		base := interp.resolveValue(frame, inst.X)
		if sv, ok := base.Raw.(*SliceValue); ok {
			frame.Locals[inst.Name()] = Value{Raw: sv.Backing, Provenance: sv.Backing}
		} else {
			frame.Locals[inst.Name()] = base
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
			frame.Locals["__return__"] = retVals[0]
		default:
			// Multi-value return: store as a tuple Value for Extract to pick apart
			frame.Locals["__return__"] = Value{Raw: retVals}
		}
		return nil // End of function

	case *ssa.Panic:
		// Program panicked: run all deferred calls across the entire goroutine
		// stack in LIFO order (innermost frame first), then halt the goroutine.
		// This prevents false arena-leak reports when defer a.Free() was set up
		// in a frame above the panic site (#20).
		g := interp.goroutines[gid]
		if g != nil {
			for i := len(g.Stack) - 1; i >= 0; i-- {
				f := g.Stack[i]
				for j := len(f.Defers) - 1; j >= 0; j-- {
					interp.executeDeferred(gid, f.Defers[j])
				}
				f.Defers = nil // prevent double-execution in popFrame
			}
			g.Stack = nil
			g.Status = GoroutineFinished
			g.Panicked = true
		}
		return nil

	// --- Function Calls ---

	case *ssa.Call:
		result := interp.execCall(gid, fn, inst, site)
		if inst.Name() != "" {
			frame.Locals[inst.Name()] = result
		}

	case *ssa.Defer:
		// Record deferred call for execution when frame pops
		d := DeferredCall{
			Fn:   interp.callTargetName(inst.Call),
			Site: site,
		}
		for _, arg := range inst.Call.Args {
			d.Args = append(d.Args, interp.resolveValue(frame, arg))
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
		// If no static callee, check if the call value is a ClosureValue
		if callee == nil {
			calleeVal := interp.resolveValue(frame, inst.Call.Value)
			if cv, ok := calleeVal.Raw.(*ClosureValue); ok {
				callee = cv.Fn
				args = append(args, cv.FreeVars...)
			}
		}
		if callee == nil {
			break // Can't resolve the goroutine function
		}
		funcName := interp.callTargetName(inst.Call)
		newGID, err := interp.spawnGoroutine(funcName, site)
		if err != nil {
			interp.recordViolation(err)
			break
		}
		interp.sched.OnSpawn(gid, newGID)
		interp.runQueue = append(interp.runQueue, goroutineTask{
			gid:  newGID,
			fn:   callee,
			args: args,
		})

	// --- Channel Operations ---

	case *ssa.Send:
		chanVal := interp.resolveValue(frame, inst.Chan)
		val := interp.resolveValue(frame, inst.X)
		var chanID ChanID
		if id, ok := chanVal.Raw.(ChanID); ok {
			chanID = id
		}
		interp.handleChannelSend(gid, chanID, val, site)

	case *ssa.Select:
		// Simplified: return sentinel (-1, false) indicating no case ready
		frame.Locals[inst.Name()] = Value{Raw: []Value{{Raw: int64(-1)}, {Raw: false}}}

	// --- Type Conversions ---

	case *ssa.Convert:
		val := interp.resolveValue(frame, inst.X)
		// Check for unsafe.Pointer conversions
		if isUnsafePointerConversion(inst) {
			result, err := interp.handleUnsafePointer(gid, classifyUnsafeConversion(inst), val, site, inst.Type(), inst.Name())
			if err != nil {
				interp.recordViolation(err)
			}
			frame.Locals[inst.Name()] = result
		} else {
			frame.Locals[inst.Name()] = val
		}

	case *ssa.ChangeType:
		val := interp.resolveValue(frame, inst.X)
		frame.Locals[inst.Name()] = val

	case *ssa.MakeInterface:
		val := interp.resolveValue(frame, inst.X)
		frame.Locals[inst.Name()] = val

	case *ssa.MultiConvert:
		val := interp.resolveValue(frame, inst.X)
		frame.Locals[inst.Name()] = val

	// --- Phi nodes ---

	case *ssa.Phi:
		// Select the edge value corresponding to the predecessor block we came from.
		for i, pred := range inst.Block().Preds {
			if pred == frame.PrevBlock {
				frame.Locals[inst.Name()] = interp.resolveValue(frame, inst.Edges[i])
				break
			}
		}
		// Fallback: if no predecessor matched, take the first edge unconditionally.
		// This is correct because SSA guarantees edge order matches predecessor order,
		// and the entry block has no Phi nodes. The previous code skipped zero-valued
		// edges (int 0, false, nil pointer), producing wrong values in loops (#18).
		if _, exists := frame.Locals[inst.Name()]; !exists && len(inst.Edges) > 0 {
			frame.Locals[inst.Name()] = interp.resolveValue(frame, inst.Edges[0])
		}

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
		return interp.execBuiltin(gid, b, args, site)
	}

	// Handle closure calls: if the call value is a ClosureValue, extract the
	// function and append free vars after regular args (#19).
	if !call.Call.IsInvoke() {
		calleeVal := interp.resolveValue(frame, call.Call.Value)
		if cv, ok := calleeVal.Raw.(*ClosureValue); ok {
			allArgs := append(args, cv.FreeVars...)
			return interp.execFunction(gid, cv.Fn, allArgs)
		}
	}

	// Non-builtin function call is a potential GC safepoint.
	// Check for pending uintptr conversions that would be invalidated.
	if interp.registry != nil {
		for _, err := range interp.registry.CheckGCPoint(site) {
			interp.recordViolation(err)
		}
	}

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
			interp.recordViolation(err)
		}
		return result
	}

	// arena.Free() / a.Free()
	if strings.HasSuffix(calleeName, ".Free") && len(args) > 0 {
		arenaID, ok := interp.resolveArenaID(args[0])
		if ok {
			errs := interp.Memory.FreeArena(arenaID, site)
			for _, err := range errs {
				interp.recordViolation(err)
			}
		}
		return Value{}
	}

	// --- General function call: interpret the callee ---

	callee := call.Call.StaticCallee()
	if callee != nil && callee.Blocks != nil {
		return interp.execFunction(gid, callee, args)
	}

	// External function — can't interpret, return opaque value
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
				interp.recordViolation(&shadow.UnsafePointerViolation{
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
			}
		}

	case "cap":
		if len(args) > 0 {
			if sv, ok := args[0].Raw.(*SliceValue); ok {
				return Value{Raw: int64(sv.Cap)}
			}
		}

	case "append":
		// Conservative: return the first argument (the slice)
		if len(args) > 0 {
			return args[0]
		}

	case "copy":
		// Return 0 copies for now
		return Value{Raw: int64(0)}

	case "delete":
		// Map delete — no-op for safety checking purposes

	case "close":
		// Channel close — no-op for v0.2.0

	case "panic":
		// panic() — just stop execution of this goroutine

	case "recover":
		// recover() — return nil (no panic in progress in our model)
		return Value{}

	case "print", "println":
		// Ignore print output during interpretation

	case "Slice": // unsafe.Slice(ptr, len) — create slice from pointer + length
		if len(args) >= 2 && args[0].Provenance != nil {
			lenN := int(toInt64(args[1]))
			sv := &SliceValue{Backing: args[0].Provenance, Len: lenN, Cap: lenN}
			return Value{Raw: sv, Provenance: args[0].Provenance}
		}
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
	if val, ok := frame.Locals[v.Name()]; ok {
		return val
	}

	// Constants
	if c, ok := v.(*ssa.Const); ok {
		return interp.constToValue(c)
	}

	// Global
	if g, ok := v.(*ssa.Global); ok {
		return Value{Raw: g.Name()}
	}

	// Function value
	if f, ok := v.(*ssa.Function); ok {
		return Value{Raw: f.String()}
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
	val Value
	idx int
}

// advance returns the next (ok, key, value) triple from the iterator.
func (ri *rangeIter) advance() (bool, Value, Value) {
	switch sv := ri.val.Raw.(type) {
	case *SliceValue:
		if ri.idx >= sv.Len {
			return false, Value{}, Value{}
		}
		k := Value{Raw: int64(ri.idx)}
		ri.idx++
		return true, k, Value{}
	case string:
		runes := []rune(sv)
		if ri.idx >= len(runes) {
			return false, Value{}, Value{}
		}
		k := Value{Raw: int64(ri.idx)}
		v := Value{Raw: int64(runes[ri.idx])}
		ri.idx++
		return true, k, v
	}
	return false, Value{}, Value{}
}
