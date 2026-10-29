# pkg/exec — giri as a library

`github.com/scttfrdmn/giri/pkg/exec` interprets an arbitrary Go function on
caller-supplied arguments and returns its outputs along with deterministic
execution instrumentation. It was built for [tumble](https://github.com/scttfrdmn/tumble),
which uses giri as the fitness evaluator for a genetic algorithm over Go
kernels. The requirements are in [GIRI_REQUIREMENTS.md](GIRI_REQUIREMENTS.md),
and departures from them, with reasons, are in [decisions.md](decisions.md).
Performance numbers are in [perf.md](perf.md).

```go
pkg, err := exec.LoadDir("./gen")          // one package, all functions built
fn := pkg.Func("Candidate017")
res, err := exec.Run(fn, [][]any{
	{[]uint32{1, 2, 3}, uint8(7)},          // argument set 0
	{[]uint32(nil), uint8(0)},              // argument set 1
}, exec.Options{MaxSteps: 1_000_000, Trace: true, Seed: 1})
// res.Outputs[i]  — []any of Go values, one per result
// res.Outcomes[i] — Returned / Panicked / Exhausted / Opaque
// res.Steps, res.StepsByClass, res.Violations, res.Traces
```

**Build requirement:** like all of giri, a program importing `pkg/exec` must
be built with `GOEXPERIMENT=arenas`.

## Stability and versioning

- `exec.Version` is the giri release the API ships with (`report.Version`).
  The API is versioned with giri. There is no separate API version.
- Changes are **additive** within giri's 0.x series. Exported names,
  signatures, field meanings and constant values are not removed or changed.
  New fields, options, classes, outcomes and concrete functions may be added.
  Code that constructs `Options{...}` and `Result` with field names keeps
  compiling.
- `Class`, `Outcome` and `TraceKind` values are append-only: new values are
  added before the end, existing values never change.
- The set of concrete functions (`ConcreteFuncs`) only grows.
- Nothing in `pkg/exec` changes giri's CLI: its behavior, output formats, exit
  codes and detector semantics are those of the CLI entry points, which never
  use the concrete mode described below.

## Surface

### Loading

`LoadDir(dir string) (*ssa.Package, error)` loads the single package in `dir`
(which must be inside a module) and builds SSA for all of its functions in one
call: generic instantiations built, `ssa.SanityCheckFunctions` and
`ssa.BuildSerially` off. Only the target package gets function bodies.
Standard-library calls are served by giri's intercepts, and non-stdlib
dependencies are not interpreted in v0. Any module path works, including
dotless ones such as `module gen`.

### Running

`Run(fn *ssa.Function, args [][]any, opts Options) (Result, error)` runs `fn`
once per argument set, in order, in one interpreter. Package-level variables
are allocated on first use and persist between sets, exactly as across
successive native calls. The package's `init` runs once before the first set
and is not counted.

**Supported types (v0):** every integer width (`int`…`int64`,
`uint`…`uint64`, `uintptr`), `bool`, `float32`, `float64`, `string`, and
slices of those; `nil` for slice parameters. A parameter or result of any
other type is rejected before anything runs with `*UnsupportedTypeError`.
Argument values must have exactly the parameter's kind (no implicit
conversion). Otherwise `Run` returns `*ArgError`. Named basic types are
accepted by kind and returned as their underlying Go type.

**Options**

| Field | Meaning |
|---|---|
| `MaxSteps` | One step budget for the whole Run, shared by all sets in order. 0 = `DefaultMaxSteps` (10M). |
| `TrackInit` | Enable the uninitialized-read detector (off by default, as in the CLI). |
| `Trace`, `TraceCap` | Record per-value traces; cap per value (0 = `DefaultTraceCap`, 1024). |
| `Cache` | Value-cache hook (`CacheHook`), consulted only for pure instructions. |
| `SingleGoroutine` | max_goroutines = 1 and no scheduler; a `go` statement is a violation. |
| `SkipDetectors` | No shadow-memory checks or detectors, for already-verified functions. Outputs, counters and traces are unchanged. Worth using for functions that load or store; no effect on pure arithmetic (see perf.md). |
| `Seed` | Seeds `math/rand`, `math/rand/v2` and the Random/PCT schedulers. |

Detectors otherwise run with giri's CLI defaults: unsafe-pointer rules,
arena, bounds and race detection.

**Result**

| Field | Meaning |
|---|---|
| `Outputs [][]any` | Per set run: one Go value per result. `nil` unless the outcome is `Returned`. |
| `Outcomes []Outcome` | `Returned`; `Panicked` (unrecovered panic, including giri-raised index-out-of-range, nil dereference and divide-by-zero); `Exhausted`; `Opaque` (returned a value no concrete model produced). |
| `ArgsAfter [][]any` | Per set run: each slice argument's contents after the call, typed like `Outputs` (a `nil` slice is a typed nil, as natively); `nil` for non-slice parameters and for sets that didn't return. A function that doesn't write into an argument reports its input. The caller's slices are never modified. Each slice argument has its own backing array, so aliasing between arguments isn't modelled. |
| `Steps`, `StepsPerSet` | Instructions executed, callees included, in total and per set. |
| `StepsByClass map[Class]uint64` | `Steps` by instruction class; every class present; sums to `Steps`. |
| `Violations []Violation` | Detector findings in detection order: `Set`, `Category` (report category), `Message`, `Err`. |
| `Exhausted` | `MaxSteps` ran out. `Outputs`/`Outcomes` stop at the exhausted set (last outcome `Exhausted`); everything recorded so far is returned. Not an error, and no violation is recorded for it. |
| `Traces []Traces` | Per set run, when `Trace` is on. |
| `CacheHits`, `PureExecuted` | Pure instructions served by the cache / actually executed (with a cache). |

Determinism: with `SingleGoroutine` (or no goroutines), equal SSA, arguments
and `Options` give byte-identical `Result`s. Map iteration is in sorted key
order, randomness is seeded, and time intercepts are constant. Across
machines, violation messages carry absolute source paths, and `math`
functions with architecture-specific assembly follow the host. Floating-point
arithmetic is never fused, so it is identical on every host, though native
arm64 code may fuse `x*y+z`.

### Instruction classes

| Class | SSA instructions |
|---|---|
| `arithmetic` | `BinOp` other than comparisons; `UnOp` `-`, `^`, `!` |
| `compare` | `BinOp` `==` `!=` `<` `<=` `>` `>=` |
| `load` | `UnOp` `*` (dereference); map `Lookup` |
| `store` | `Store`, `MapUpdate` |
| `index` | `IndexAddr`, `FieldAddr`, `Index`, `Field`, `Slice` |
| `call-intercepted` | Calls giri models: stdlib intercepts, functions without a body, unresolved dynamic calls |
| `call-interpreted` | Calls into an interpreted function body (static, closure, function value, resolved interface method) |
| `builtin` | `len`, `cap`, `append`, `copy`, `min`, `max`, `clear`, … |
| `branch` | `If`, `Jump` |
| `phi` | `Phi` |
| `alloc` | `Alloc`, `MakeSlice`, `MakeMap`, `MakeChan`, `MakeClosure`, `MakeInterface` |
| `other` | Everything else: `Convert`, `ChangeType`, `Extract`, `Return`, `Panic`, … |

`interpreter.ClassOf(instr)` gives the static class of an instruction.

### Traces

`Traces` is `map[ssa.Value]*ValueTrace`, holding every value-producing
instruction of the run function itself (callees are not traced) that
executed. A `ValueTrace` holds one entry per execution, in order, in one
per-type slice chosen from the value's static type: `Ints` (signed),
`Uints`, `Floats`, `Bools` or `Strings`. Values of any other type
(`TraceOther`: pointers, slices, tuples) record only `Count`. `Count` is the
total number of executions. `Truncated` means `Count` exceeded the cap, and
`Opaque` means some execution produced a value with no concrete
representation. Treat either as "no fingerprint".

### Value cache and the purity classification

`CacheHook` has two methods:

```go
Lookup(v ssa.Value, operands []Value) (result Value, steps uint64, ok bool)
Store(v ssa.Value, operands []Value, result Value, steps uint64)
```

The interpreter consults the hook **only** for instructions `IsPure(instr)`
reports true. An instruction is pure when its result is a function of its
operand values alone, it reads and writes no memory (shadow or otherwise), it
performs no channel or goroutine operation and calls nothing with side
effects, and its operands and result are all scalars (bool, numeric, string;
not `unsafe.Pointer`). Concretely:

| Pure | Not pure |
|---|---|
| `BinOp` on scalars | `UnOp *` (load), `UnOp <-` (receive) |
| `UnOp` `-` `^` `!` on scalars | `Store`, `MapUpdate`, `Lookup`, `IndexAddr`, `FieldAddr`, `Index`, `Field`, `Slice` |
| `Convert`, `ChangeType` between scalar types | Conversions to or from slices, `unsafe.Pointer` |
| `Call` of a pure concrete intercept with scalar arguments and result (e.g. `math.Sqrt`, `bits.OnesCount64`) | Calls to interpreted functions, builtins, `sort.*`, any non-concrete intercept, interface calls |
| | `Phi` (depends on control flow), allocations, `Go`, `Defer`, `Send`, `Select`, `Panic` |

Operands are passed in a fixed order: `BinOp` (X, Y); `UnOp`, `Convert`,
`ChangeType` (X); `Call` (arguments). On a hit, `result` becomes the
instruction's value, `steps` is added to the counters (attributed to the
instruction's class), and the instruction is skipped. On a miss the
instruction executes. If it recorded no violation and did not panic, `Store`
is called with `steps = 1`. Because a panicking execution is never stored, a
hit can never mask a finding. Caching is per instruction: a hit saves the
instruction itself, not the computation of its operands.

Scalar `Value`s carry `Raw` as `int64` (all integer types, unsigned values
zero-extended, `uint64` as its bit pattern), `float64` (`float32` values are
exactly representable), `bool`, `string`, or `complex128`.

### Concreteness

`IsConcrete(fn *types.Func) bool` reports whether a standard-library function
is modelled concretely, meaning the interpreter computes exactly what native
code does. `ConcreteFuncs()` lists the whole set as `"pkg/path.Name"`. Each
concrete function wraps the real stdlib implementation. v0 covers `math`,
`math/bits`, `strings`, `strconv`, `unicode` and `unicode/utf8` functions
with scalar signatures, plus the typed `sort` slice helpers (`sort.Ints`,
`SearchInts`, …). Everything else giri intercepts is approximate or opaque
(placeholder results, such as the crypto key-generation intercepts) and
reports false. So do methods and functions of the loaded package itself,
which are interpreted from source.

## The concrete interpreter mode

`Run` executes in a value-faithful mode of giri's interpreter (`NewSession` in
`pkg/interpreter`). Since #260 the CLI shares the memory part (the element
store, array/struct values, element-carrying builtins and conversions, `sort`).
The typed-arithmetic part below is `pkg/exec`-only:

- integer operations wrap to the operand's width and honor signedness;
  `float32` results round to `float32`; constants are exact;
- every store is remembered per (allocation, byte offset), so slice, array
  and struct-field loads return stored values (or the zero value); arrays and
  structs are also values in their own right, so whole-value copies, field
  selection and indexing of such values, `==` on them, and `append`/`copy` of
  them behave as native; an array or struct giri can't compute is opaque, not
  zero;
- `append`, `copy`, `clear`, `min`/`max`, string slicing and string↔`[]byte`/
  `[]rune` conversions move real values;
- integer division by zero, out-of-range indexing and nil dereference halt
  the call as a run-time panic would;
- float→integer conversions that overflow saturate (NaN → 0), identically on
  every host;
- a `uintptr` converted from a pointer is an integer encoding the pointer's
  allocation and offset (`alloc<<32 + offset`), so `unsafe.Pointer(uintptr(p)
  + n)` addresses the same allocation `n` bytes on, and the unsafe-pointer
  rules still apply. Its numeric value is not a real address, and an integer
  that never came from a pointer faults when converted to one and accessed.

**Not covered in v0:** maps, arrays and structs as arguments or results,
calls into non-stdlib dependencies of the loaded package, and `cap()` of a
slice grown by `append` (it follows the runtime growth formula without size
class rounding, so it can be smaller than native).
