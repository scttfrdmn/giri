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
	"go/token"
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
		// Try init functions
		mainFn = prog.Main.Func("init")
	}

	if mainFn != nil {
		interp.execFunction(mainGID, mainFn, nil)
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

	// Execute blocks starting from the entry block
	block := fn.Blocks[0]
	for block != nil {
		block = interp.execBlock(gid, fn, block)
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
	site := interp.posString(instr.Pos())
	frame := interp.currentFrame(gid)

	switch inst := instr.(type) {

	// --- Memory Operations ---

	case *ssa.Alloc:
		// Alloc: allocate memory for a local variable or heap object
		typeName := inst.Type().String()
		kind := shadow.AllocStack
		if inst.Heap {
			kind = shadow.AllocHeap
		}
		size := estimateTypeSize(typeName)
		allocID := interp.Memory.Allocate(kind, size, typeName, site)
		ptr := &shadow.Pointer{Alloc: allocID, Offset: 0}
		frame.Locals[inst.Name()] = Value{Raw: ptr, Provenance: ptr}

	case *ssa.Store:
		// Store: *addr = val
		addr := interp.resolveValue(frame, inst.Addr)
		val := interp.resolveValue(frame, inst.Val)
		size := estimateTypeSize(inst.Val.Type().String())

		if err := interp.handleStore(gid, addr, val, size, site); err != nil {
			interp.recordViolation(err)
		}

	case *ssa.UnOp:
		// UnOp: includes dereference (*x), address-of (&x), negation, etc.
		operand := interp.resolveValue(frame, inst.X)

		switch inst.Op.String() {
		case "*": // Dereference (load)
			size := estimateTypeSize(inst.Type().String())
			result, err := interp.handleLoad(gid, operand, size, site)
			if err != nil {
				interp.recordViolation(err)
			}
			frame.Locals[inst.Name()] = result
		default:
			// Other unary ops: -, ^, etc. — pass through
			frame.Locals[inst.Name()] = operand
		}

	case *ssa.FieldAddr:
		// FieldAddr: &s.Field
		base := interp.resolveValue(frame, inst.X)
		// In real implementation: compute field offset from types.Sizes
		fieldOffset := inst.Field * 8 // Simplified
		result := interp.handleFieldAddr(gid, base, fieldOffset, site)
		frame.Locals[inst.Name()] = result

	case *ssa.IndexAddr:
		// IndexAddr: &a[i]
		base := interp.resolveValue(frame, inst.X)
		idx := interp.resolveValue(frame, inst.Index)
		indexVal := 0
		if i, ok := idx.Raw.(int64); ok {
			indexVal = int(i)
		}
		elemSize := 8 // Simplified; real impl uses types.Sizes
		result := interp.handleIndexAddr(gid, base, indexVal, elemSize, site)
		frame.Locals[inst.Name()] = result

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

		if len(retVals) > 0 {
			frame.Locals["__return__"] = retVals[0]
		}
		return nil // End of function

	case *ssa.Panic:
		// Program panicked — record where
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
		// Spawn goroutine
		funcName := interp.callTargetName(inst.Call)
		var args []Value
		for _, arg := range inst.Call.Args {
			args = append(args, interp.resolveValue(frame, arg))
		}
		newGID, err := interp.spawnGoroutine(funcName, site)
		if err != nil {
			interp.recordViolation(err)
		} else {
			// In a full implementation, the scheduler would handle execution.
			// For now, we note the spawn for later scheduling.
			_ = newGID
			_ = args
		}

	// --- Channel Operations ---

	case *ssa.Send:
		val := interp.resolveValue(frame, inst.X)
		interp.handleChannelSend(gid, val, site)

	// --- Type Conversions ---

	case *ssa.Convert:
		val := interp.resolveValue(frame, inst.X)
		// Check for unsafe.Pointer conversions
		if isUnsafePointerConversion(inst) {
			result, err := interp.handleUnsafePointer(gid, classifyUnsafeConversion(inst), val, site)
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

	// --- Phi nodes ---

	case *ssa.Phi:
		// Phi: value depends on which predecessor block we came from.
		// For now, take the first available value.
		for _, edge := range inst.Edges {
			val := interp.resolveValue(frame, edge)
			if val.Raw != nil {
				frame.Locals[inst.Name()] = val
				break
			}
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
	calleeName := interp.callTargetName(call.Call)

	// Resolve arguments
	var args []Value
	for _, arg := range call.Call.Args {
		args = append(args, interp.resolveValue(frame, arg))
	}

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

	// unsafe.Add
	if calleeName == "unsafe.Add" && len(args) >= 2 {
		base := args[0]
		if base.Provenance != nil {
			offset := 0
			if i, ok := args[1].Raw.(int64); ok {
				offset = int(i)
			}
			derived := interp.Memory.DerivePointer(base.Provenance, offset)
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
	}

	// new(T)
	if calleeName == "new" {
		typeName := "unknown"
		if len(args) > 0 {
			typeName = fmt.Sprintf("*%v", args[0].Raw)
		}
		return interp.handleAlloc(gid, typeName, site, false, 0)
	}

	// make([]T, len, cap)
	if calleeName == "make" {
		typeName := "[]unknown"
		return interp.handleAlloc(gid, typeName, site, false, 0)
	}

	// --- General function call: interpret the callee ---

	callee := call.Call.StaticCallee()
	if callee != nil && callee.Blocks != nil {
		return interp.execFunction(gid, callee, args)
	}

	// External function — can't interpret, return opaque value
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

// constToValue converts an SSA constant to an interpreted Value.
func (interp *Interpreter) constToValue(c *ssa.Const) Value {
	if c.Value == nil {
		return Value{Raw: nil} // nil constant
	}
	// Extract the concrete value
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
