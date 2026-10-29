// Package interpreter - concrete.go implements value-faithful ("concrete")
// semantics used by library callers (pkg/exec).
//
// The CLI interprets programs to find undefined behavior and deliberately
// approximates values: integer arithmetic is plain int64, and memory remembers
// only the value stored at offset 0 of each allocation. That is enough to drive
// detectors, but a caller that wants a function's actual outputs needs Go's
// real semantics. Concrete mode supplies them:
//
//   - integer operations wrap to the operand width and honor signedness;
//     float32 results are rounded to float32
//   - every store is remembered per (allocation, byte offset), so loads of
//     slice/array elements return the stored value (or the type's zero value)
//   - append, copy, clear, string slicing and string<->[]byte/[]rune
//     conversions move real element values
//   - integer division by zero halts the goroutine like a runtime panic
//   - map range iteration order is sorted, not Go's randomized order
//
// Concrete mode is enabled only through NewSession; Run, RunN and RunTests
// never set it, so CLI behavior is unchanged.
package interpreter

import (
	"fmt"
	"go/constant"
	"go/token"
	"go/types"
	"math"
	"sort"

	"golang.org/x/tools/go/ssa"

	"github.com/scttfrdmn/giri/pkg/shadow"
)

// elemKey addresses one stored value in concrete mode: an allocation and a
// byte offset within it.
type elemKey struct {
	alloc  shadow.AllocID
	offset int
}

// intKind describes how a basic integer type is represented: its bit width
// and signedness. Integers are always held as int64; unsigned values are kept
// zero-extended (uint64 as its bit pattern), signed values sign-extended.
type intKind struct {
	bits     uint
	unsigned bool
}

// basicIntKind returns the intKind of t, or ok=false if t is not an integer.
// int, uint and uintptr are 64-bit (giri targets 64-bit platforms only).
func basicIntKind(t types.Type) (intKind, bool) {
	b, ok := t.Underlying().(*types.Basic)
	if !ok || b.Info()&types.IsInteger == 0 {
		return intKind{}, false
	}
	switch b.Kind() {
	case types.Int8:
		return intKind{8, false}, true
	case types.Int16:
		return intKind{16, false}, true
	case types.Int32:
		return intKind{32, false}, true
	case types.Uint8:
		return intKind{8, true}, true
	case types.Uint16:
		return intKind{16, true}, true
	case types.Uint32:
		return intKind{32, true}, true
	case types.Uint, types.Uint64, types.Uintptr:
		return intKind{64, true}, true
	default: // Int, Int64, untyped
		return intKind{64, false}, true
	}
}

// wrap normalizes v to k's width: sign-extends signed kinds and zero-extends
// unsigned kinds below 64 bits.
func (k intKind) wrap(v int64) int64 {
	switch k.bits {
	case 8:
		if k.unsigned {
			return int64(uint8(v))
		}
		return int64(int8(v))
	case 16:
		if k.unsigned {
			return int64(uint16(v))
		}
		return int64(int16(v))
	case 32:
		if k.unsigned {
			return int64(uint32(v))
		}
		return int64(int32(v))
	}
	return v
}

// isFloat32 reports whether t's underlying type is float32.
func isFloat32(t types.Type) bool {
	b, ok := t.Underlying().(*types.Basic)
	return ok && b.Kind() == types.Float32
}

// roundFloat rounds f to float32 precision when t is float32.
func roundFloat(f float64, t types.Type) float64 {
	if isFloat32(t) {
		return float64(float32(f))
	}
	return f
}

// concreteBinOp evaluates a binary operation with Go's typed semantics.
// xt is the operand type (both operands share it, except shifts), yt is the
// right operand's type (relevant for shift counts). Returns ok=false when the
// operands are not concrete scalars, in which case the caller falls back to
// the comparison helpers or the approximate evaluator.
func concreteBinOp(op token.Token, x, y Value, xt, yt types.Type) (Value, bool) {
	if k, isInt := basicIntKind(xt); isInt {
		xi, ok1 := x.Raw.(int64)
		yi, ok2 := y.Raw.(int64)
		if !ok1 || !ok2 {
			return Value{}, false
		}
		ux, uy := uint64(xi), uint64(yi)
		switch op {
		case token.ADD:
			return Value{Raw: k.wrap(xi + yi)}, true
		case token.SUB:
			return Value{Raw: k.wrap(xi - yi)}, true
		case token.MUL:
			return Value{Raw: k.wrap(xi * yi)}, true
		case token.QUO:
			if yi == 0 {
				return Value{Raw: int64(0)}, true // caller reports the panic
			}
			if k.unsigned {
				return Value{Raw: k.wrap(int64(ux / uy))}, true
			}
			return Value{Raw: k.wrap(xi / yi)}, true
		case token.REM:
			if yi == 0 {
				return Value{Raw: int64(0)}, true
			}
			if k.unsigned {
				return Value{Raw: k.wrap(int64(ux % uy))}, true
			}
			return Value{Raw: k.wrap(xi % yi)}, true
		case token.AND:
			return Value{Raw: xi & yi}, true
		case token.OR:
			return Value{Raw: xi | yi}, true
		case token.XOR:
			return Value{Raw: k.wrap(xi ^ yi)}, true
		case token.AND_NOT:
			return Value{Raw: k.wrap(xi &^ yi)}, true
		case token.SHL, token.SHR:
			// The count is unsigned unless yt is signed and negative (which the
			// caller reports as a panic before getting here).
			cnt := uy
			if op == token.SHL {
				if cnt >= uint64(k.bits) {
					return Value{Raw: int64(0)}, true
				}
				return Value{Raw: k.wrap(xi << cnt)}, true
			}
			if k.unsigned {
				if cnt >= 64 {
					return Value{Raw: int64(0)}, true
				}
				return Value{Raw: k.wrap(int64(ux >> cnt))}, true
			}
			if cnt >= 64 {
				cnt = 63 // arithmetic shift saturates at the sign
			}
			return Value{Raw: xi >> cnt}, true
		case token.EQL:
			return Value{Raw: xi == yi}, true
		case token.NEQ:
			return Value{Raw: xi != yi}, true
		case token.LSS:
			if k.unsigned {
				return Value{Raw: ux < uy}, true
			}
			return Value{Raw: xi < yi}, true
		case token.LEQ:
			if k.unsigned {
				return Value{Raw: ux <= uy}, true
			}
			return Value{Raw: xi <= yi}, true
		case token.GTR:
			if k.unsigned {
				return Value{Raw: ux > uy}, true
			}
			return Value{Raw: xi > yi}, true
		case token.GEQ:
			if k.unsigned {
				return Value{Raw: ux >= uy}, true
			}
			return Value{Raw: xi >= yi}, true
		}
		return Value{}, false
	}
	if isFloat32(xt) {
		r := evalBinOp(op, x, y)
		if f, ok := r.Raw.(float64); ok {
			// Computing in float64 and rounding once is exact for + - * /
			// on float32 operands (float64 has > 2*24+2 significand bits).
			r.Raw = float64(float32(f))
		}
		return r, r.Raw != nil
	}
	r := evalBinOp(op, x, y)
	return r, r.Raw != nil
}

// concreteEqual compares two non-scalar values for == / != in concrete mode:
// slices, maps, funcs and channels against nil, and pointers by identity.
func concreteEqual(x, y Value) (bool, bool) {
	isNil := func(v Value) (bool, bool) {
		switch r := v.Raw.(type) {
		case nil:
			return v.Provenance == nil, true
		case *SliceValue:
			return r.Backing == nil, true
		}
		return false, false
	}
	xn, xok := isNil(x)
	yn, yok := isNil(y)
	if xok && yok && (xn || yn) {
		return xn == yn, true
	}
	xp, xIsPtr := x.Raw.(*shadow.Pointer)
	yp, yIsPtr := y.Raw.(*shadow.Pointer)
	switch {
	case xIsPtr && yIsPtr:
		return xp.Alloc == yp.Alloc && xp.Offset == yp.Offset, true
	case xIsPtr && y.Raw == nil, yIsPtr && x.Raw == nil:
		return false, true
	}
	return false, false
}

// concreteUnOp evaluates negation and bitwise complement with Go's typed
// semantics. Returns ok=false for non-scalar operands.
func concreteUnOp(op token.Token, x Value, t types.Type) (Value, bool) {
	if k, ok := basicIntKind(t); ok {
		v, ok := x.Raw.(int64)
		if !ok {
			return Value{}, false
		}
		switch op {
		case token.SUB:
			return Value{Raw: k.wrap(-v)}, true
		case token.XOR:
			return Value{Raw: k.wrap(^v)}, true
		}
	}
	return Value{}, false
}

// concreteConvertNumeric converts between basic numeric types with Go
// semantics. Out-of-range float→integer conversions (implementation-defined
// in Go, and different between amd64 and arm64) saturate, and NaN converts to
// 0, so results are identical on every host.
func concreteConvertNumeric(v Value, src, dst types.Type) (Value, bool) {
	sk, srcInt := basicIntKind(src)
	dk, dstInt := basicIntKind(dst)
	sb, srcBasic := src.Underlying().(*types.Basic)
	db, dstBasic := dst.Underlying().(*types.Basic)
	if !srcBasic || !dstBasic {
		return Value{}, false
	}
	srcFloat := sb.Info()&types.IsFloat != 0
	dstFloat := db.Info()&types.IsFloat != 0

	switch {
	case srcInt && dstInt:
		n, ok := v.Raw.(int64)
		if !ok {
			return Value{}, false
		}
		return Value{Raw: dk.wrap(n)}, true

	case srcInt && dstFloat:
		n, ok := v.Raw.(int64)
		if !ok {
			return Value{}, false
		}
		if isFloat32(dst) {
			if sk.unsigned {
				return Value{Raw: float64(float32(uint64(n)))}, true
			}
			return Value{Raw: float64(float32(n))}, true
		}
		if sk.unsigned {
			return Value{Raw: float64(uint64(n))}, true
		}
		return Value{Raw: float64(n)}, true

	case srcFloat && dstInt:
		f, ok := v.Raw.(float64)
		if !ok {
			return Value{}, false
		}
		return Value{Raw: floatToInt(f, dk)}, true

	case srcFloat && dstFloat:
		f, ok := v.Raw.(float64)
		if !ok {
			return Value{}, false
		}
		return Value{Raw: roundFloat(f, dst)}, true
	}
	return Value{}, false
}

// floatToInt truncates f toward zero into k, saturating out-of-range values
// and mapping NaN to 0.
func floatToInt(f float64, k intKind) int64 {
	if math.IsNaN(f) {
		return 0
	}
	f = math.Trunc(f)
	if k.unsigned {
		maxU := math.Ldexp(1, int(k.bits)) // 2^bits
		switch {
		case f <= 0:
			return 0
		case f >= maxU:
			return k.wrap(-1) // all ones
		}
		return k.wrap(int64(uint64(f)))
	}
	lim := math.Ldexp(1, int(k.bits)-1) // 2^(bits-1)
	switch {
	case f >= lim:
		return int64(uint64(1)<<(k.bits-1) - 1)
	case f < -lim:
		return -int64(uint64(1) << (k.bits - 1))
	}
	return int64(f)
}

// concreteConst converts an SSA constant exactly: uint64 constants above
// MaxInt64 keep their bit pattern, float32 constants are rounded, and typed
// nil slices become empty nil slices so len/cap/== nil work.
func concreteConst(c *ssa.Const) Value {
	if c.Value == nil {
		return concreteZero(c.Type())
	}
	switch c.Value.Kind() {
	case constant.Int:
		if b, ok := c.Type().Underlying().(*types.Basic); ok && b.Info()&types.IsFloat != 0 {
			f, _ := constant.Float64Val(c.Value)
			return Value{Raw: roundFloat(f, c.Type())}
		}
		if k, ok := basicIntKind(c.Type()); ok && k.unsigned {
			if u, exact := constant.Uint64Val(c.Value); exact {
				return Value{Raw: int64(u)}
			}
		}
		v, _ := constant.Int64Val(c.Value)
		return Value{Raw: v}
	case constant.Float:
		if isFloat32(c.Type()) {
			f, _ := constant.Float32Val(c.Value)
			return Value{Raw: float64(f)}
		}
		// An integer-valued float constant may still be Kind Float; also
		// handle constants typed as integers but represented as floats.
		if k, ok := basicIntKind(c.Type()); ok {
			f, _ := constant.Float64Val(c.Value)
			return Value{Raw: floatToInt(f, k)}
		}
	}
	return constValue(c)
}

// constValue is the non-concrete constant conversion (shared with the CLI).
func constValue(c *ssa.Const) Value {
	if c.Value == nil {
		return Value{Raw: nil}
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

// concreteZero returns the zero value of t in concrete mode: typed scalar
// zeros, an empty nil slice for slices, and nil for every reference type.
func concreteZero(t types.Type) Value {
	switch u := t.Underlying().(type) {
	case *types.Basic:
		info := u.Info()
		switch {
		case info&types.IsBoolean != 0:
			return Value{Raw: false}
		case info&types.IsString != 0:
			return Value{Raw: ""}
		case info&types.IsComplex != 0:
			return Value{Raw: complex128(0)}
		case info&types.IsFloat != 0:
			return Value{Raw: float64(0)}
		case info&types.IsInteger != 0:
			return Value{Raw: int64(0)}
		}
	case *types.Slice:
		return Value{Raw: &SliceValue{}}
	}
	return Value{}
}

// --- Concrete memory ---

// loadConcrete reads a value of type t through addr. Detector checks run
// through handleLoad unless checks are skipped; the value itself comes from
// the element store, defaulting to t's zero value.
func (interp *Interpreter) loadConcrete(gid int64, addr Value, t types.Type, size int, site string) (Value, error) {
	if addr.Provenance == nil {
		if addr.Raw == nil {
			if g := interp.goroutines[gid]; g != nil {
				g.Panicked = true
			}
			return Value{}, &shadow.NilPointerDerefError{Site: site, GID: gid}
		}
		// Untracked pointer (from an intercept): defer to the CLI semantics.
		return interp.handleLoad(gid, addr, size, site)
	}
	if !interp.skipChecks {
		if _, err := interp.handleLoad(gid, addr, size, site); err != nil {
			return concreteZero(t), err
		}
	}
	if v, ok := interp.elems[elemKey{addr.Provenance.Alloc, addr.Provenance.Offset}]; ok {
		return v, nil
	}
	return concreteZero(t), nil
}

// storeConcrete writes val through addr, running detector checks unless
// skipped, and records the value in the element store.
func (interp *Interpreter) storeConcrete(gid int64, addr Value, val Value, size int, site string) error {
	if addr.Provenance == nil {
		return interp.handleStore(gid, addr, val, size, site)
	}
	if !interp.skipChecks {
		if err := interp.handleStore(gid, addr, val, size, site); err != nil {
			return err
		}
	} else if addr.Provenance.Offset == 0 {
		// handleStore maintains valueStore (named-return recomputation, #49).
		interp.valueStore[addr.Provenance.Alloc] = val
	}
	interp.elems[elemKey{addr.Provenance.Alloc, addr.Provenance.Offset}] = val
	return nil
}

// elemAt returns the element stored at index i of the slice sv, or the zero
// value of elemType.
func (interp *Interpreter) elemAt(sv *SliceValue, i int, elemType types.Type, elemSize int) Value {
	if sv.Backing == nil {
		return concreteZero(elemType)
	}
	if v, ok := interp.elems[elemKey{sv.Backing.Alloc, sv.Backing.Offset + i*elemSize}]; ok {
		return v
	}
	return concreteZero(elemType)
}

// setElem stores v at index i of the slice sv without detector checks; used by
// builtins whose bounds are established by construction.
func (interp *Interpreter) setElem(sv *SliceValue, i int, elemSize int, v Value) {
	if sv.Backing == nil {
		return
	}
	interp.elems[elemKey{sv.Backing.Alloc, sv.Backing.Offset + i*elemSize}] = v
}

// newConcreteSlice allocates a slice of elemType holding elems, with the given
// capacity (>= len(elems)).
func (interp *Interpreter) newConcreteSlice(elemType types.Type, elems []Value, capacity int, site string) Value {
	elemSize := interp.typeSizeOf(elemType)
	if capacity < len(elems) {
		capacity = len(elems)
	}
	allocSize := capacity * elemSize
	if allocSize <= 0 {
		allocSize = elemSize
	}
	allocID := interp.allocate(shadow.AllocHeap, allocSize, "[]"+elemType.String(), site)
	if !interp.skipChecks {
		interp.Memory.MarkInitialized(allocID, 0, allocSize)
	}
	ptr := arenaNew(interp.arena, shadow.Pointer{Alloc: allocID, Offset: 0})
	sv := arenaNew(interp.arena, SliceValue{Backing: ptr, Len: len(elems), Cap: capacity})
	for i, e := range elems {
		interp.elems[elemKey{allocID, i * elemSize}] = e
	}
	return Value{Raw: sv, Provenance: ptr}
}

// sliceElems reads all elements of a slice value. A nil slice yields
// (nil, true); a non-slice value yields ok=false.
func (interp *Interpreter) sliceElems(v Value, elemType types.Type) (elems []Value, isNil bool, ok bool) {
	switch r := v.Raw.(type) {
	case nil:
		return nil, true, true
	case *SliceValue:
		if r.Backing == nil {
			return nil, true, true
		}
		size := interp.typeSizeOf(elemType)
		elems = make([]Value, r.Len)
		for i := range elems {
			elems[i] = interp.elemAt(r, i, elemType, size)
		}
		return elems, false, true
	}
	return nil, false, false
}

// sliceElemType returns the element type of slice type t, or nil.
func sliceElemType(t types.Type) types.Type {
	if s, ok := t.Underlying().(*types.Slice); ok {
		return s.Elem()
	}
	return nil
}

// --- Concrete builtins ---

// concreteBuiltin implements builtins whose concrete-mode semantics differ
// from the CLI model (append, copy, clear, min, max). ok=false defers to
// execBuiltin.
func (interp *Interpreter) concreteBuiltin(gid int64, name string, call *ssa.CallCommon, args []Value, site string) (Value, bool) {
	switch name {
	case "append":
		if len(args) < 2 {
			return Value{}, false
		}
		elemType := sliceElemType(call.Args[0].Type())
		if elemType == nil {
			return Value{}, false
		}
		var extra []Value
		switch src := args[1].Raw.(type) {
		case string: // append([]byte, string...)
			extra = make([]Value, len(src))
			for i := 0; i < len(src); i++ {
				extra[i] = Value{Raw: int64(src[i])}
			}
		default:
			e, _, ok := interp.sliceElems(args[1], elemType)
			if !ok {
				return Value{}, false
			}
			extra = e
		}
		base, _ := args[0].Raw.(*SliceValue)
		if base == nil {
			base = &SliceValue{}
		}
		if len(extra) == 0 {
			return args[0], true
		}
		elemSize := interp.typeSizeOf(elemType)
		newLen := base.Len + len(extra)
		if newLen <= base.Cap && base.Backing != nil {
			out := arenaNew(interp.arena, SliceValue{Backing: base.Backing, Len: newLen, Cap: base.Cap})
			for i, e := range extra {
				interp.setElem(out, base.Len+i, elemSize, e)
			}
			return Value{Raw: out, Provenance: base.Backing}, true
		}
		elems := make([]Value, 0, newLen)
		for i := 0; i < base.Len; i++ {
			elems = append(elems, interp.elemAt(base, i, elemType, elemSize))
		}
		elems = append(elems, extra...)
		return interp.newConcreteSlice(elemType, elems, growCap(base.Cap, newLen), site), true

	case "copy":
		if len(args) < 2 {
			return Value{}, false
		}
		dst, ok := args[0].Raw.(*SliceValue)
		if !ok {
			return Value{Raw: int64(0)}, true
		}
		elemType := sliceElemType(call.Args[0].Type())
		elemSize := interp.typeSizeOf(elemType)
		var src []Value
		if s, ok := args[1].Raw.(string); ok {
			src = make([]Value, len(s))
			for i := 0; i < len(s); i++ {
				src[i] = Value{Raw: int64(s[i])}
			}
		} else {
			src, _, _ = interp.sliceElems(args[1], elemType)
		}
		n := min(dst.Len, len(src))
		for i := 0; i < n; i++ {
			interp.setElem(dst, i, elemSize, src[i])
		}
		return Value{Raw: int64(n)}, true

	case "clear":
		if len(args) < 1 {
			return Value{}, false
		}
		if sv, ok := args[0].Raw.(*SliceValue); ok {
			elemType := sliceElemType(call.Args[0].Type())
			elemSize := interp.typeSizeOf(elemType)
			for i := 0; i < sv.Len; i++ {
				interp.setElem(sv, i, elemSize, concreteZero(elemType))
			}
			return Value{}, true
		}
		return Value{}, false

	case "min", "max":
		if len(args) == 0 {
			return Value{}, false
		}
		t := call.Args[0].Type()
		best := args[0]
		for _, a := range args[1:] {
			less, ok := concreteLess(a, best, t)
			if !ok {
				return Value{}, false
			}
			if f, isF := a.Raw.(float64); isF && math.IsNaN(f) {
				best = a
				continue
			}
			if bf, isF := best.Raw.(float64); isF && math.IsNaN(bf) {
				continue
			}
			if name == "min" {
				if less || (isNegZero(a) && !isNegZero(best) && floatEqual(a, best)) {
					best = a
				}
			} else {
				greater, _ := concreteLess(best, a, t)
				if greater || (isNegZero(best) && !isNegZero(a) && floatEqual(a, best)) {
					best = a
				}
			}
		}
		return best, true
	}
	return Value{}, false
}

// concreteLess reports x < y under t's ordering.
func concreteLess(x, y Value, t types.Type) (bool, bool) {
	r, ok := concreteBinOp(token.LSS, x, y, t, t)
	if !ok {
		return false, false
	}
	b, ok := r.Raw.(bool)
	return b, ok
}

func isNegZero(v Value) bool {
	f, ok := v.Raw.(float64)
	return ok && f == 0 && math.Signbit(f)
}

func floatEqual(x, y Value) bool {
	a, ok1 := x.Raw.(float64)
	b, ok2 := y.Raw.(float64)
	return ok1 && ok2 && a == b
}

// growCap mirrors the runtime's growslice capacity formula (without the final
// rounding up to an allocator size class, so cap() after growth can be smaller
// than native).
func growCap(oldCap, newLen int) int {
	newcap := oldCap
	doublecap := newcap + newcap
	if newLen > doublecap {
		return newLen
	}
	const threshold = 256
	if oldCap < threshold {
		return doublecap
	}
	for newcap < newLen {
		newcap += (newcap + 3*threshold) >> 2
	}
	return newcap
}

// --- Concrete conversions involving memory ---

// concreteConvert implements Convert in concrete mode. ok=false defers to the
// CLI convertValue.
func (interp *Interpreter) concreteConvert(v Value, src, dst types.Type, site string) (Value, bool) {
	if r, ok := concreteConvertNumeric(v, src, dst); ok {
		return r, true
	}
	sb, srcBasic := src.Underlying().(*types.Basic)
	db, dstBasic := dst.Underlying().(*types.Basic)

	// string → []byte / []rune
	if srcBasic && sb.Info()&types.IsString != 0 {
		if elem := sliceElemType(dst); elem != nil {
			s, ok := v.Raw.(string)
			if !ok {
				return Value{}, false
			}
			eb, _ := elem.Underlying().(*types.Basic)
			if eb == nil {
				return Value{}, false
			}
			var elems []Value
			switch eb.Kind() {
			case types.Uint8:
				elems = make([]Value, len(s))
				for i := 0; i < len(s); i++ {
					elems[i] = Value{Raw: int64(s[i])}
				}
			case types.Int32:
				for _, r := range s {
					elems = append(elems, Value{Raw: int64(r)})
				}
			default:
				return Value{}, false
			}
			return interp.newConcreteSlice(elem, elems, len(elems), site), true
		}
	}

	// []byte / []rune → string
	if dstBasic && db.Info()&types.IsString != 0 {
		if elem := sliceElemType(src); elem != nil {
			eb, _ := elem.Underlying().(*types.Basic)
			elems, _, ok := interp.sliceElems(v, elem)
			if eb == nil || !ok {
				return Value{}, false
			}
			switch eb.Kind() {
			case types.Uint8:
				bs := make([]byte, len(elems))
				for i, e := range elems {
					bs[i] = byte(toInt64(e))
				}
				return Value{Raw: string(bs)}, true
			case types.Int32:
				rs := make([]rune, len(elems))
				for i, e := range elems {
					rs[i] = rune(toInt64(e))
				}
				return Value{Raw: string(rs)}, true
			}
		}
	}
	return Value{}, false
}

// --- Deterministic map iteration ---

// sortMapKeys orders map keys deterministically: by kind (bool, int, float,
// string, other), then by value.
func sortMapKeys(keys []interface{}) {
	rank := func(k interface{}) int {
		switch k.(type) {
		case bool:
			return 0
		case int64:
			return 1
		case float64:
			return 2
		case string:
			return 3
		}
		return 4
	}
	sort.SliceStable(keys, func(i, j int) bool {
		a, b := keys[i], keys[j]
		ra, rb := rank(a), rank(b)
		if ra != rb {
			return ra < rb
		}
		switch av := a.(type) {
		case bool:
			return !av && b.(bool)
		case int64:
			return av < b.(int64)
		case float64:
			return av < b.(float64)
		case string:
			return av < b.(string)
		}
		return fmt.Sprint(a) < fmt.Sprint(b)
	})
}
