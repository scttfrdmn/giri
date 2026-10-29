package exec

import (
	"golang.org/x/tools/go/ssa"

	"github.com/scttfrdmn/giri/pkg/interpreter"
)

// CacheHook is a caller-supplied value cache. The interpreter consults it
// only for instructions IsPure reports as pure.
//
// Before executing a pure instruction v, the interpreter calls Lookup with
// v's operand values (BinOp: X, Y; UnOp, Convert, ChangeType: X; Call: the
// arguments). On a hit it installs result as v's value, adds steps to the
// step counters (attributed to v's class) and skips execution. On a miss it
// executes v and, if no violation was recorded and the goroutine did not
// halt, calls Store with steps = 1. The operands slice is freshly allocated
// for each instruction and may be retained.
//
// Operand and result values of pure instructions are always scalars: Raw is
// an int64 (all integer types; unsigned values zero-extended, uint64 as its
// bit pattern), float64 (float32 values are exactly representable), bool,
// string or complex128, with no provenance. A uintptr made from a pointer is
// an int64 encoding of the pointer (docs/exec-api.md). The interpreter checks
// operands and results at run time and bypasses the hook for any execution
// whose values are not scalars, whatever their static types.
type CacheHook interface {
	Lookup(v ssa.Value, operands []Value) (result Value, steps uint64, ok bool)
	Store(v ssa.Value, operands []Value, result Value, steps uint64)
}

// IsPure reports whether the value cache may be consulted for instr: its
// result depends only on its operand values, it touches no memory, no
// effectful intercept, channel or goroutine, and its operands and result are
// scalars. See interpreter.IsPure for the exact set.
func IsPure(instr ssa.Instruction) bool { return interpreter.IsPure(instr) }
