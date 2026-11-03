// Package interpreter - slots.go: frame-local value storage.
//
// The CLI keeps a frame's SSA values in Frame.Locals, a map keyed by SSA
// name. Sessions (pkg/exec) number each function's parameters, free
// variables and value-producing instructions once, and keep a frame's values
// in Frame.Slots indexed by that number: no per-frame map, and no string
// hashing on every operand read and result write (#271). All access goes
// through setLocal/getLocal/setReturn/getReturn, which pick the mode.
package interpreter

import (
	"unsafe"

	"golang.org/x/tools/go/ssa"
)

// valueKey is the data pointer of an ssa.Value interface. Every ssa.Value
// implementation is a pointer type, so this identifies the value, and
// hashing a pointer is much cheaper than hashing an interface.
func valueKey(v ssa.Value) unsafe.Pointer {
	return (*[2]unsafe.Pointer)(unsafe.Pointer(&v))[1]
}

// numberFunc assigns slot numbers to fn's values on first use and returns
// fn's slot count.
func (interp *Interpreter) numberFunc(fn *ssa.Function) int32 {
	if n, ok := interp.fnSlots[fn]; ok {
		return n
	}
	var n int32
	add := func(v ssa.Value) {
		k := valueKey(v)
		if _, ok := interp.slotIdx[k]; !ok {
			interp.slotIdx[k] = n
			n++
		}
	}
	for _, p := range fn.Params {
		add(p)
	}
	for _, fv := range fn.FreeVars {
		add(fv)
	}
	for _, b := range fn.Blocks {
		for _, instr := range b.Instrs {
			if v, ok := instr.(ssa.Value); ok {
				add(v)
			}
		}
	}
	// fn.Recover is one of fn.Blocks, so its values are already numbered.
	interp.fnSlots[fn] = n
	return n
}

// initSlots sizes frame's slots for fn. No-op on the CLI path.
func (interp *Interpreter) initSlots(frame *Frame, fn *ssa.Function) {
	if interp.slotIdx == nil {
		return
	}
	n := interp.numberFunc(fn)
	frame.Slots = make([]Value, n)
	frame.SlotOK = make([]bool, n)
}

// setLocal binds v's value in frame.
func (interp *Interpreter) setLocal(frame *Frame, v ssa.Value, val Value) {
	if interp.slotIdx == nil {
		frame.Locals[interp.vname(v)] = val
		return
	}
	if i, ok := interp.slotIdx[valueKey(v)]; ok && int(i) < len(frame.Slots) {
		frame.Slots[i] = val
		frame.SlotOK[i] = true
	}
}

// getLocal returns v's value in frame, if bound.
func (interp *Interpreter) getLocal(frame *Frame, v ssa.Value) (Value, bool) {
	if interp.slotIdx == nil {
		val, ok := frame.Locals[interp.vname(v)]
		return val, ok
	}
	if i, ok := interp.slotIdx[valueKey(v)]; ok && int(i) < len(frame.Slots) && frame.SlotOK[i] {
		return frame.Slots[i], true
	}
	return Value{}, false
}

// setReturn records frame's return value.
func (interp *Interpreter) setReturn(frame *Frame, val Value) {
	if interp.slotIdx == nil {
		frame.Locals["__return__"] = val
		return
	}
	frame.Ret, frame.HasRet = val, true
}

// getReturn returns frame's return value, if set.
func (interp *Interpreter) getReturn(frame *Frame) (Value, bool) {
	if interp.slotIdx == nil {
		val, ok := frame.Locals["__return__"]
		return val, ok
	}
	return frame.Ret, frame.HasRet
}

// goroutine returns the goroutine with ID gid, or nil. The last one found is
// cached: the interpreter looks the current goroutine up several times per
// instruction (#271).
func (interp *Interpreter) goroutine(gid int64) *Goroutine {
	if g := interp.gor; g != nil && g.ID == gid {
		return g
	}
	g := interp.goroutines[gid]
	if g != nil {
		interp.gor = g
	}
	return g
}

// forgetGoroutine removes goroutine gid, keeping the lookup cache coherent.
func (interp *Interpreter) forgetGoroutine(gid int64) {
	delete(interp.goroutines, gid)
	if interp.gor != nil && interp.gor.ID == gid {
		interp.gor = nil
	}
}
