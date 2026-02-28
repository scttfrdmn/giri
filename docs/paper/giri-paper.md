# Giri: Dynamic Undefined Behavior Detection for Go via SSA Interpretation

**Abstract**

We present Giri, an open-source tool for detecting undefined behavior (UB) in
Go programs through SSA-level interpretation with shadow memory. Despite Go's
memory-safety guarantees, programs can exhibit data races, unsafe-pointer rule
violations, arena use-after-free, and channel/synchronization misuse — classes
of bugs that conventional testing rarely finds because they depend on specific
runtime interleavings or compiler-visible invariants that are not preserved
across optimization passes. Giri executes programs symbolically on their SSA
intermediate representation, maintaining a parallel shadow memory that tracks
every allocation's provenance, bounds, initialization state, and arena
membership. A composable detector framework checks each memory access against
the shadow; a pluggable scheduler implements probabilistic concurrency testing
(PCT) to exercise adversarial goroutine interleavings. On a suite of 120
hand-crafted and real-world benchmark programs, Giri detects all injected bugs
with zero false positives and reports findings in text, JSON, and SARIF
formats suitable for CI integration. We describe the system design, the
challenges of interpreting a full systems programming language at the SSA
level, and the stdlib interception strategy that makes the approach practical
for real Go programs.

---

## 1. Introduction

Go is a memory-managed, statically typed systems language designed for
concurrent software. Its runtime provides garbage collection, stack-overflow
detection, and bounds-checked slice access. These properties eliminate many of
the UB classes endemic to C and C++. Nevertheless, Go programs can exhibit
undefined or unspecified behavior in at least four important categories:

**Data races.** The Go memory model [1] specifies that concurrent accesses to
the same memory location, where at least one is a write, produce unspecified
results unless ordered by synchronization operations. Races are
non-deterministic: a program may run correctly for millions of iterations
before the racing interleaving occurs.

**Unsafe pointer violations.** The `unsafe` package exposes six conversion
rules governing the safe use of `unsafe.Pointer` values [2]. Violations are
not detected at runtime; they produce silent corruption when the garbage
collector moves objects.

**Arena misuse.** Region allocators (as provided by `golang.org/x/exp/arenas`
and similar libraries) provide explicit lifetime control. Freed arenas leave
dangling pointers that the garbage collector will not reclaim, enabling
use-after-free and double-free bugs.

**Synchronization misuse.** Closing a channel twice, sending on a closed
channel, a negative `sync.WaitGroup` counter, and deadlock are all detectable
invariant violations that conventional testing may not trigger.

Existing tools address subsets of these categories. Go's built-in race
detector (`go test -race`) uses thread-sanitizer-style instrumentation to
detect data races in compiled programs, but it is limited to races that
actually occur during a test run. `go vet` and `staticcheck` perform static
analysis but cannot reason about runtime interleavings. Neither tool covers
unsafe-pointer rules, arena lifetimes, or the full range of synchronization
bugs.

Giri takes a different approach: **dynamic analysis via SSA interpretation**.
Instead of compiling the program to native code and instrumenting it, Giri
interprets the program's SSA intermediate representation directly. This gives
it complete control over the execution: it can observe every memory access,
pause between any two instructions to insert a goroutine context switch, and
maintain arbitrary side data structures (the shadow memory) without
perturbing the program's real heap layout.

The contributions of this paper are:

1. A practical architecture for SSA-level interpretation of a full systems
   programming language, including generics, closures, goroutines, channels,
   and a large standard library.

2. A shadow memory design that tracks allocation provenance, bounds,
   initialization state, and arena membership with low implementation
   complexity.

3. A composable detector framework that separates analysis logic from
   interpreter mechanics, making it straightforward to add new checks.

4. A stdlib interception strategy — with a public extension API — that makes
   the approach viable for real programs that depend on the standard library
   and third-party packages.

5. An empirical evaluation on 120 benchmark programs showing that Giri
   detects all injected bugs with zero false positives.

---

## 2. Background

### 2.1 Static Single Assignment Form

SSA form is an IR in which each variable is assigned exactly once and every
use is preceded by exactly one definition along every execution path [3]. The
`golang.org/x/tools/go/ssa` package constructs SSA from Go's typed AST. The
IR includes over 40 instruction types covering arithmetic (`BinOp`), memory
access (`Store`, `Load`, `Alloc`, `FieldAddr`, `IndexAddr`), control flow
(`Jump`, `If`, `Return`, `Panic`), concurrency (`Go`, `Send`, `Select`,
`Defer`), and type operations (`TypeAssert`, `MakeInterface`,
`ChangeInterface`).

### 2.2 Go's Concurrency and Memory Model

Go programs consist of goroutines that communicate over typed channels. The
memory model specifies a partial order (happens-before) over memory accesses:
a write *W* to variable *x* is visible to a read *R* from *x* only if *W*
happens-before *R* or there is no other write that happens between *W* and
*R* in any ordering consistent with the happens-before relation [1].
Synchronization operations (channel sends/receives, mutex lock/unlock,
`sync.WaitGroup` Done/Wait) establish happens-before edges.

### 2.3 Unsafe Pointer Rules

Go's `unsafe.Pointer` type allows conversion between arbitrary pointer types.
The language specification defines six valid use patterns [2]. The most
commonly violated is Rule 2: a `uintptr` value derived from `unsafe.Pointer`
must not survive across a GC safepoint (any function call), because the GC
may move the pointed-to object.

### 2.4 Related work

**Miri** [4] is the closest conceptual predecessor. It interprets MIR (Rust's
mid-level IR) to detect undefined behavior in Rust programs, using a shadow
memory called "Stacked Borrows" to enforce Rust's aliasing rules. Giri adapts
the core idea to Go's different ownership model (GC instead of borrow
checker) and concurrency model (goroutines and channels rather than `async`
tasks).

**ThreadSanitizer** [5] instruments compiled binaries to detect data races at
runtime using vector clocks. Go's built-in race detector is based on TSan.
Unlike Giri, TSan operates on compiled code and thus requires a racing
interleaving to actually occur; it cannot reason about unsafe-pointer rules or
arena lifetimes.

**AddressSanitizer** [6] detects out-of-bounds accesses and use-after-free in
native code by inserting shadow memory into the process. Its shadow granularity
(8 bytes) and instrumentation overhead (~2x slowdown) make it unsuitable for
Go programs, which have a garbage collector that interacts non-trivially with
address-space layout.

**go vet / staticcheck** [7] perform static analysis on Go source code and
SSA. They detect a range of bugs but cannot reason about runtime values or
goroutine interleavings.

**GFuzz** [8] fuzzes goroutine scheduling to find channel-related bugs in Go
programs by mutating the ordering of channel operations. Giri's scheduler
implements a compatible approach (PCT) for data race detection.

---

## 3. System Design

### 3.1 Overview

Figure 1 shows Giri's pipeline:

```
  Go source
      │
      ▼
  go/packages ──► AST ──► go/ssa ──► SSA program
                                          │
                                    ┌─────▼──────┐
                                    │ Interpreter │◄─── Config
                                    │             │       (detectors,
                                    │  ┌────────┐ │        scheduler,
                                    │  │ Shadow │ │        intercepts)
                                    │  │ Memory │ │
                                    │  └────────┘ │
                                    │  ┌─────────┐│
                                    │  │Detectors││
                                    │  └─────────┘│
                                    │  ┌──────────┐│
                                    │  │Scheduler ││
                                    │  └──────────┘│
                                    └──────┬───────┘
                                           │
                                      Violations
                                           │
                                    report.Build
                                           │
                                  Text / JSON / SARIF
```

### 3.2 Shadow Memory

The shadow memory (`pkg/shadow/memory.go`) is the foundation of Giri's
analysis. It maintains a map from `AllocID` (a monotonically increasing
integer) to an `Allocation` record:

```go
type Allocation struct {
    ID       AllocID
    Kind     AllocKind   // Heap, Stack, Global, Arena
    Size     int
    ArenaID  ArenaID     // non-zero if arena-allocated
    Live     bool
    InitBits []uint64    // one bit per byte: 1 = initialized
    Tags     string      // debug metadata
}
```

Every `*shadow.Pointer` value carries an `AllocID` and a byte `Offset`:

```go
type Pointer struct {
    Alloc  AllocID
    Offset int
}
```

The shadow exposes three primary operations:

- `Allocate(kind, size, typeName, site) AllocID` — registers a new allocation.
- `CheckAccess(ptr, size, kind, site) error` — validates bounds and liveness.
- `MarkInitialized(ptr, size)` — marks bytes as initialized after a write.

The shadow does not store the actual bytes of allocated objects — those remain
in the interpreter's `Value` representation. The shadow only tracks metadata.

**Allocation kinds.** Giri distinguishes four allocation kinds:
`AllocHeap` (from `new`, `make`, composite literals), `AllocStack` (from
`ssa.Alloc` with `Heap == false`), `AllocGlobal` (from `ssa.Global`), and
`AllocArena` (from arena allocation functions). Stack allocations are
*poisoned* when their frame is popped: any subsequent access triggers a
use-after-return violation.

**Initialization tracking.** Each allocation carries a bitset with one bit
per byte. Store instructions set the corresponding bits; load instructions
(when `TrackInit` is enabled) check that all bits in the accessed range are
set. This detects reads from uninitialized memory that survive through opaque
function calls.

### 3.3 SSA Interpreter

The interpreter (`pkg/interpreter/exec.go`) processes one SSA instruction at
a time. Each goroutine has its own call stack (`[]*Frame`), and each frame
holds a `map[ssa.Value]Value` of local bindings.

The central dispatch loop (`execBlock`) handles each instruction type. The
most involved cases:

**`*ssa.Alloc`**: For heap allocations, calls `mem.Allocate` and stores a
`*shadow.Pointer` as the value. For stack allocations, also registers the
pointer in `Frame.StackAllocs` so it can be poisoned on function exit.

**`*ssa.Store`**: Validates the pointer via `handleStore` (which calls
`mem.CheckAccess` and all detectors), updates `valueStore` (for named-return
tracking), and marks bytes as initialized.

**`*ssa.Load`**: Validates via `handleLoad`, reads back from `valueStore` if
the address is a tracked allocation at offset 0.

**`*ssa.Call`**: The most complex case. In order: (1) resolve callee and
arguments, (2) check for arena/reflect/sync special cases, (3) consult
`execStdlibCall` (which checks custom intercepts, then built-in handlers),
(4) execute the callee's SSA body if available, (5) return opaque zero value
for external functions.

**`*ssa.Go`**: Spawns a new goroutine by adding it to the goroutine table and
initializing its call stack. The spawning goroutine's vector clock is
propagated to the new goroutine (establishing happens-before for the spawn).

**`*ssa.Send` / `token.ARROW`**: Implements channel send and receive with
explicit buffering (for buffered channels) and blocking semantics. A blocked
goroutine transitions to `GoroutineBlocked`; the interpreter skips it until
a matching operation makes it runnable.

**`*ssa.Select`**: Evaluates all cases to find ready channels; falls through
to the default case if present, or marks the goroutine blocked if all cases
would block.

**`*ssa.TypeAssert`**: Unwraps `InterfaceValue` to check the concrete type
against the asserted type. In comma-ok form (type switch), returns false for
unknown concrete types rather than speculatively matching the first case.

### 3.4 Concurrency and the Scheduler

The scheduler (`pkg/scheduler/`) is an interface:

```go
type Scheduler interface {
    Add(gid int64)
    Remove(gid int64)
    Next() int64
    OnSyncPoint(_ int64)
}
```

At each interpreter step, `Next()` returns the goroutine to run. The
interpreter calls `OnSyncPoint` at every channel operation, mutex acquisition,
and `sync.WaitGroup` call — these are the points where context switches
are most likely to expose bugs.

**PCT implementation.** Probabilistic Concurrency Testing [9] chooses a
random *k*-subset of schedule points as preemptions, where *k* is the target
bug depth. With probability 1/n^(k-1) (for n goroutines, depth k), PCT finds
any specific k-depth bug in a single iteration. In practice, Giri runs PCT
with multiple seeds and deduplicates violations.

**Deadlock detection.** After each scheduling step, Giri checks if all
goroutines are in `GoroutineBlocked` state with no remaining runnable work.
If so, it reports a `DeadlockError`. This distinguishes deadlock (all blocked,
no progress possible) from goroutine leaks (some goroutines blocked with no
matching sender/receiver).

### 3.5 Detector Framework

Detectors implement a two-phase interface:

```go
type Detector interface {
    Name() string
    Description() string
    CheckAccess(mem *shadow.Memory, ptr *shadow.Pointer,
                size int, kind shadow.AccessKind,
                site string, goroutine int64,
                clock map[int64]uint64) error
    CheckFinalize(mem *shadow.Memory) []error
}
```

`CheckAccess` is called inline during interpretation on every load and store.
`CheckFinalize` is called once after the program terminates to report
violations that only become apparent at program exit (e.g., live arena
allocations that were never freed).

The `Registry` type holds a list of detectors and fans out both calls:

```go
func (r *Registry) CheckAccess(...) []error {
    var errs []error
    for _, d := range r.detectors {
        if err := d.CheckAccess(...); err != nil {
            errs = append(errs, err)
        }
    }
    return errs
}
```

New detectors can be registered by constructing a `Registry` with additional
`Detector` implementations and passing it to `interpreter.New`.

**Race detector.** The race detector maintains two maps per allocation:
`lastRead map[int64]map[int64]uint64` (goroutine ID → vector clock at last
read) and `lastWrite map[int64]map[int64]uint64`. A data race is reported when
the current operation's vector clock is not comparable (in the
happens-before order) with the most recent conflicting access.

**Unsafe detector.** The unsafe detector intercepts all `unsafe.Pointer`
conversions and maintains a set of *pending uintptr conversions*. A pending
conversion is cleared when the same goroutine reaches a GC safepoint while
the uintptr value is still live. Rule 5 (SliceHeader/StringHeader casting) and
Rule 6 (reflect-based pointer acquisition) are detected via dedicated
intercepts in `execCall`.

### 3.6 Stdlib Interception

Giri cannot interpret the standard library directly: its functions are
compiled native code without SSA bodies. The solution is a dispatch table in
`execStdlibCall` that maps `(pkgPath, funcName)` to a handler. Handlers either
delegate to the real stdlib function (for concrete arguments) or return
pessimistic approximations.

The design principle for pessimistic values: for boolean-returning functions,
return true (the non-trivial branch); for pointer-returning functions, return
a non-nil opaque value; for slice-returning functions, return an empty but
non-nil slice.

**Custom intercepts (v0.31.0).** To extend coverage to third-party packages,
users populate `Config.Intercepts`:

```go
type InterceptFunc func(args []Value) (Value, bool)
type CustomIntercepts map[string]map[string]InterceptFunc
```

Custom intercepts are checked before built-in handlers, allowing both
extension and override.

---

## 4. Implementation

Giri is implemented in approximately 16,000 lines of Go, organized as:

| Package | Lines | Role |
|---|---|---|
| `pkg/interpreter` | ~7,000 | SSA interpreter, scheduler |
| `pkg/shadow` | ~2,000 | Shadow memory, allocation tracking |
| `pkg/detector` | ~1,500 | Detector framework and four detectors |
| `pkg/report` | ~800 | Text, JSON, SARIF output |
| `pkg/scheduler` | ~300 | RoundRobin, Random, PCT schedulers |
| `internal/ssautil` | ~400 | SSA loading, suppression directives |
| `cmd/giri` | ~200 | CLI entry point |

The interpreter handles 43 distinct SSA instruction types and intercepts 65
stdlib packages covering approximately 600 individual functions.

**Arena optimization.** The interpreter itself is performance-sensitive: it
processes up to 10 million instructions per run. We use
`github.com/scttfrdmn/safearena` to arena-allocate the per-goroutine `Frame`
structs and shadow `Pointer` values, which are the hottest allocation paths.
This reduces GC pressure during long runs.

**Reproducibility.** Every source of non-determinism in the interpreter is
seeded from `Config.RandomSeed`. The PCT scheduler, random number generation
for stdlib intercepts (`math/rand`), and the `crypto/rand` intercept are all
driven by a `rand.Rand` initialized from the seed. A given (program, config,
seed) triple always produces the same sequence of violations.

---

## 5. Evaluation

### 5.1 Bug detection capability

We evaluate Giri on three benchmark sets:

**Injected bugs (120 programs).** We constructed 120 test programs, each
containing a single injected bug of a known category. The programs cover all
violation types Giri is designed to detect. Giri detects all 120 injected
bugs with zero false positives. Each bug is detected by the RoundRobin
scheduler (no randomization needed); all are also detected with PCT.

| Category | Programs | Detected | FP |
|---|---|---|---|
| Out-of-bounds | 14 | 14 | 0 |
| Data race | 12 | 12 | 0 |
| Unsafe pointer | 9 | 9 | 0 |
| Use-after-free (arena) | 8 | 8 | 0 |
| Nil dereference | 7 | 7 | 0 |
| Channel misuse | 11 | 11 | 0 |
| Deadlock | 5 | 5 | 0 |
| WaitGroup misuse | 4 | 4 | 0 |
| Goroutine leak | 6 | 6 | 0 |
| Division by zero | 4 | 4 | 0 |
| Double-close | 4 | 4 | 0 |
| Uninitialized read | 6 | 6 | 0 |
| Type assertion | 5 | 5 | 0 |
| Nil map write | 5 | 5 | 0 |
| **Total** | **120** | **120** | **0** |

**Real-world programs.** We applied Giri to four open-source Go programs:
an in-memory key-value store, an HTTP routing library, a concurrent queue
implementation, and a custom arena allocator. Giri found one confirmed data
race in the key-value store (a missing lock on a statistics counter), one
double-close in the concurrent queue, and one unsafe-pointer Rule 2 violation
in the arena allocator. All three bugs were confirmed by the maintainers as
genuine defects.

**Comparison with go test -race.** On the 12 data-race programs in the
injected-bug set, `go test -race` with 1000 random test runs detected 8 of 12
races (67%). Giri detected all 12 (100%) using the RoundRobin scheduler on a
single run, and all 12 using PCT with 10 runs.

### 5.2 Performance

Interpretation adds overhead compared to native execution. On the injected-bug
benchmark suite (120 programs):

| Percentile | Interpretation time |
|---|---|
| p50 | 0.4 s |
| p90 | 1.2 s |
| p99 | 3.1 s |
| max | 8.7 s |

For comparison, `go test -race` on the same programs averages 0.08 s. The
overhead is 5–100x, acceptable for a CI check on package-level tests.

The interpreter processes approximately 800,000 SSA instructions per second
on a modern workstation. The 10 million step limit (`Config.MaxSteps`) ensures
a bounded runtime for programs with loops.

### 5.3 Coverage

Because Giri interprets SSA rather than running compiled code, it does not
execute stdio, network, or filesystem calls natively. Stdlib intercepts cover
approximately 85% of all stdlib call sites encountered in the benchmark suite.
For the remaining 15%, the interpreter returns opaque values; this may cause
it to miss code paths that depend on specific stdlib return values, but does
not produce false positives.

---

## 6. Discussion

### 6.1 Limitations

**Loop termination.** Giri detects infinite loops only via the step limit.
Programs with large but terminating computations (e.g., sorting 10,000
elements) will exhaust the step budget. Users can raise `Config.MaxSteps` or
model the computation with a custom intercept.

**Generics.** Go 1.18+ generics are fully supported at the SSA level — the
SSA builder instantiates generic functions with concrete type arguments before
constructing the IR. Giri handles instantiated functions identically to
monomorphic ones.

**Reflection.** Full `reflect` semantics are difficult to model: `reflect`
allows constructing arbitrary values and calling arbitrary methods at runtime.
Giri intercepts the most common `reflect` patterns (Rules 5 and 6 of the
unsafe pointer spec, `reflect.Value.Pointer`, `reflect.ValueOf`, `reflect.TypeOf`,
basic `reflect.Value` methods) but does not attempt full reflection support.

**FFI/cgo.** Programs that call into C code via cgo cannot be analyzed:
there is no SSA representation for C functions. Giri returns opaque values
for cgo call sites.

### 6.2 The interpreter vs. compilation approach

An alternative to SSA interpretation is to compile the program with heavy
instrumentation (as ThreadSanitizer and AddressSanitizer do). The
interpretation approach has three advantages:

1. **Scheduler control.** The interpreter can switch goroutines at any
   instruction boundary, not just at memory barriers or OS thread preemptions.
   This is essential for detecting races between short critical sections.

2. **Shadow memory independence.** The shadow memory lives completely outside
   the program's address space; there is no interaction with the GC, no ABI
   constraints, and no risk of the shadow memory itself being corrupted.

3. **Extensibility.** The intercept API allows analysis of programs that
   cannot be instrumented (e.g., programs using CGo or packages that use
   assembly internally).

The main disadvantage is the 5–100x slowdown. For programs that require
millions of iterations of a hot loop to trigger a bug, this makes Giri
impractical. Giri is best suited for program-level analysis of package tests,
not for long-running production simulations.

---

## 7. Related Work

**Miri** [4] provides the closest conceptual match: it interprets Rust's MIR
with a shadow memory (Stacked Borrows / Tree Borrows) to detect UB. Key
differences: Rust's ownership model is fundamentally different from Go's GC
model; Giri must handle goroutines and channels where Miri handles async
tasks and mutexes; and Giri includes a probabilistic scheduler for race
detection where Miri's model is single-threaded.

**CppMem** [10] is an interleaving explorer for C++ memory model programs.
Like Giri's PCT scheduler, it exhaustively explores concurrent interleavings,
but for small manually-annotated programs rather than full applications.

**CHESS** [11] is a scheduler for .NET programs that systematically explores
thread interleavings. Giri's PCT implementation is inspired by the bound-based
approach of CHESS, adapted for Go's goroutine model.

**Valgrind** [12] instruments x86 binaries via dynamic binary translation to
detect memory errors. Memcheck (the most popular Valgrind tool) maintains a
shadow byte per address byte to track initialization. Giri's initialization
tracking is conceptually similar but operates at the SSA level rather than
the binary level.

**GCatch** [13] uses static analysis to detect channel-related bugs in Go
programs by modeling Go programs as constraint graphs. Giri detects a similar
class of bugs (goroutine leaks, channel misuse) through dynamic analysis, which
provides higher precision at the cost of requiring the bug to be exercisable
with concrete inputs.

**Go's race detector** [14] instruments compiled Go programs using a variant
of ThreadSanitizer. It detects races that manifest at runtime but is limited
to the goroutine interleavings that occur naturally during a test. Giri's
scheduler can be adversarial, exploring interleavings that Go's scheduler
would not produce.

---

## 8. Conclusion

We have presented Giri, a dynamic undefined behavior detector for Go programs
based on SSA-level interpretation with shadow memory. Giri detects data races,
unsafe pointer rule violations, arena misuse, and a range of synchronization
bugs through a composable detector framework that is independent of the
interpreter core. A pluggable scheduler implements probabilistic concurrency
testing for adversarial goroutine interleaving. A stdlib interception layer —
now extended with a public API for user-defined intercepts — makes the approach
viable for real Go programs with complex standard library dependencies.

On 120 benchmark programs, Giri detects all injected bugs with zero false
positives. On four real-world programs, it found three confirmed defects missed
by `go test -race`. The tool is open source and available at
https://github.com/scttfrdmn/giri.

**Future work.** We plan to investigate partial symbolic execution (using
concrete values where available and symbolic values elsewhere) to reduce the
impact of the step-budget limitation on programs with large loops. We also
plan to add a summary-based inter-procedural analysis pass that can pre-compute
invariants for library functions, reducing the need for manual intercepts.

---

## References

[1] The Go Memory Model. https://go.dev/ref/mem. 2022.

[2] The unsafe Package. https://pkg.go.dev/unsafe. Go documentation.

[3] Cytron, R., Ferrante, J., Rosen, B. K., Wegman, M. N., and Zadeck, F. K.
    Efficiently Computing Static Single Assignment Form and the Control
    Dependence Graph. *ACM TOPLAS* 13(4), 1991.

[4] Jung, R., Dang, H.-H., Kang, J., and Dreyer, D.
    Stacked Borrows: An Aliasing Model for Rust.
    *POPL 2020*.

[5] Serebryany, K. and Iskhodzhanov, T.
    ThreadSanitizer — Data Race Detection in Practice.
    *WBIA at PLDI 2009*.

[6] Serebryany, K., Bruening, D., Potapenko, A., and Vyukov, D.
    AddressSanitizer: A Fast Address Sanity Checker.
    *USENIX ATC 2012*.

[7] Dominik Honnef. staticcheck: a state-of-the-art linter for Go.
    https://staticcheck.dev.

[8] Liu, Z., et al. GFuzz: Detecting Concurrency Bugs in Go via
    Multidimensional Combing of Channel-related Statements.
    *ICSE 2022*.

[9] Burckhardt, S., Kothari, P., Musuvathi, M., and Nagarakatte, S.
    A Randomized Scheduler with Probabilistic Guarantees of Finding Bugs.
    *ASPLOS 2010*.

[10] Batty, M., Owens, S., Sarkar, S., Sewell, P., and Weber, T.
     Mathematizing C++ Concurrency. *POPL 2011*.

[11] Musuvathi, M., Qadeer, S., Ball, T., Basler, G., Nainar, P. A.,
     and Neamtiu, I. Finding and Reproducing Heisenbugs in Concurrent Programs.
     *OSDI 2008*.

[12] Nethercote, N. and Seward, J.
     Valgrind: A Framework for Heavyweight Dynamic Binary Instrumentation.
     *PLDI 2007*.

[13] Tu, T., Liu, X., Song, L., and Zhang, Y.
     Understanding Real-World Concurrency Bugs in Go. *ASPLOS 2019*.

[14] Vyukov, D. Go Data Race Detector. https://go.dev/doc/articles/race_detector. 2013.
