# How Giri Works: SSA Interpretation and Shadow Memory

*In Part 1 we saw what Giri can detect. This post explains the machinery
underneath: how Go programs are represented as SSA, how the interpreter
executes them, and how shadow memory tracks every pointer's story.*

---

## Starting point: Go's SSA

When the Go compiler processes your source code, it transforms it through
several intermediate representations before emitting machine code. One of the
most useful of these is **Static Single Assignment (SSA) form**: a
representation where every variable is assigned exactly once, and control flow
is made explicit through basic blocks and phi-nodes.

`golang.org/x/tools/go/ssa` exposes this representation as a Go library. Giri
uses it as its input. This has several advantages over working at the source
level:

- Implicit operations (index bounds checks, interface boxing, goroutine
  spawning) are made explicit as SSA instructions.
- Control flow is a simple graph of basic blocks — no `for`/`if`/`switch`
  syntax to parse.
- Every variable use is traceable to its definition.

Here's what a simple function looks like in SSA:

```go
// Source:
func add(a, b int) int {
    return a + b
}

// SSA:
func add(a int, b int) int:
entry:                                           (source.go:2)
    t0 = a + b                                   (source.go:3)
    return t0
```

Real programs are more interesting — `for` loops become cycles in the block
graph, `interface{}` becomes explicit boxing, and goroutine spawning becomes
a `go` instruction.

---

## The interpreter loop

Giri's interpreter (`pkg/interpreter/exec.go`) runs the SSA program one
instruction at a time. Each goroutine has its own **call stack** (a slice of
`*Frame` values), and each frame holds the local variable bindings for one
function invocation.

The main loop looks like this (simplified):

```
while active goroutines exist:
    g = scheduler.Next()        // pick a goroutine to run
    inst = g.currentInstruction()
    switch inst.(type) {
    case *ssa.Alloc:     handle allocation
    case *ssa.Store:     handle store, check for races
    case *ssa.Load:      handle load, check for races
    case *ssa.BinOp:     handle arithmetic, check for div-by-zero
    case *ssa.Call:      handle function call
    case *ssa.Go:        spawn new goroutine
    case *ssa.Send:      handle channel send
    ...
    }
```

The interpreter handles over 40 SSA instruction types. For most arithmetic and
control flow, it executes the real operation on the concrete value. For memory
operations, it also updates the shadow memory.

---

## Shadow memory: tracking every pointer's story

The shadow memory (`pkg/shadow/memory.go`) is a parallel data structure that
lives alongside the interpreter. Its job is to remember, for every allocated
object:

- Where it was allocated (heap, stack, arena, global)
- How big it is (in bytes)
- Which goroutine allocated it
- Whether each byte has been initialized
- Which arena (if any) it belongs to
- Whether it has been freed

Every `*shadow.Pointer` value carries an `AllocID` (a unique integer
identifying the allocation) and a byte `Offset` into that allocation. When
the interpreter executes a store or load, it asks the shadow:

```go
err := mem.CheckAccess(ptr, size, AccessWrite, site)
```

The shadow looks up the allocation, checks the bounds, verifies the
initialization state, and passes the pointer to each registered **Detector**.

### Pointer provenance

A key design decision in Giri is **transitive provenance tracking**. When
you cast an `unsafe.Pointer` to a `uintptr` and back, or box a concrete value
into an interface, Giri preserves the provenance metadata through the cast:

```go
// Giri tracks that p2 derives from the same allocation as p:
p := new(int32)
u := uintptr(unsafe.Pointer(p))     // u.Provenance = p's AllocID
p2 := (*int32)(unsafe.Pointer(u))  // p2.Provenance = same AllocID
```

This lets the bounds checker correctly report out-of-bounds accesses even
after pointer arithmetic, and lets the arena detector correctly detect
use-after-free even after interface boxing.

---

## The detector framework

Detectors (`pkg/detector/detector.go`) are composable checkers that receive
every memory access and finalization event. The interface is simple:

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

`CheckAccess` is called on every load, store, and bounds check. `CheckFinalize`
is called after the program terminates to report leaks and pending violations.

Giri ships with four detectors:

**BoundsDetector** checks that every access falls within its allocation:

```go
if int(ptr.Offset)+size > int(alloc.Size) {
    return &shadow.OutOfBoundsError{...}
}
```

**RaceDetector** maintains a vector clock per goroutine and per allocation.
A data race is reported when two concurrent accesses from unrelated
goroutines touch the same memory and at least one is a write:

```go
if !happensBefore(lastWrite.clock, thisClock) {
    return &shadow.DataRaceError{...}
}
```

**UnsafeDetector** enforces Go's six `unsafe.Pointer` conversion rules.
The most important: a `uintptr` value must not survive a GC safepoint, because
the GC can move the object it was pointing to. The detector tracks pending
`uintptr` conversions and reports violations when a GC-safe point is reached
(a function call, a channel operation, or any synchronization).

**ArenaDetector** tracks arena lifetimes. When an arena is freed, all
allocations made from it are marked. Any subsequent access to those addresses
is a use-after-free.

---

## The scheduler

Concurrency bugs only manifest at specific goroutine interleavings. Most test
runs use Go's default scheduler, which might never produce the racing
interleaving. Giri's scheduler (`pkg/scheduler/`) is adversarial by design.

Three strategies are available:

**RoundRobin** — deterministic, fast, good for finding bugs that manifest
at any interleaving. Runs goroutines in a fixed round-robin order.

**Random** — shuffles the goroutine order at each step using a configurable
seed. Good for sampling the space of interleavings.

**PCT (Probabilistic Concurrency Testing)** — a principled algorithm from
the research literature that is provably more likely to find bugs of a given
*depth* (number of preemptions from the bug path) than random testing. Giri
implements PCT with configurable `BugDepth` and can run multiple independent
iterations with `--runs N`.

For most programs, start with RoundRobin (`-strategy roundrobin`) and switch
to PCT (`-strategy pct -runs 100`) if you want to explore the space of
concurrent interleavings.

---

## Stdlib interception

One challenge with interpreting real Go programs: the standard library is
compiled native code, not SSA. Giri can't interpret `strings.Contains` by
reading its SSA body — there isn't one available.

Giri solves this with **stdlib intercepts**: a dispatch table that maps
`(pkgPath, funcName)` to a handler function that models the stdlib function's
semantics. For example:

```go
case "Contains":
    if s0ok && s1ok {
        return Value{Raw: strings.Contains(s0, s1)}, true
    }
    return Value{Raw: true}, true // pessimistic: assume it contains
```

When the concrete arguments are available, Giri delegates to the real stdlib
function and returns the real result. When they're not (e.g., the string came
from an unmodeled external call), Giri returns a pessimistic non-zero value
so downstream code takes the non-trivial branch.

Giri ships with intercepts for over 60 stdlib packages, covering strings,
encoding, crypto, networking, filesystem, archival, and more.

---

## Putting it all together

When you run `giri ./mypackage`:

1. `go/packages` loads your package and all its (non-stdlib) dependencies.
2. `golang.org/x/tools/go/ssa` converts the AST to SSA form.
3. `ssautil.ParseSuppressions` extracts `//giri:ignore` directives.
4. `interpreter.Run` initializes shadow memory, detector registry, and
   scheduler, then starts the interpreter loop.
5. The interpreter executes SSA instructions, calling detectors on every
   memory access and querying the stdlib intercept table on every external
   call.
6. After the program terminates (or the step limit is hit), `CheckFinalize`
   is called on each detector to report leaks and pending violations.
7. `report.Build` formats the violations and `rpt.Write` outputs them in
   text, JSON, or SARIF format.

---

## Part 3: extending Giri

The next post covers `Config.Intercepts` — how to model your own libraries
so Giri can analyze code that depends on them.

- **Part 3:** [Extending Giri with Custom Intercepts](03-custom-intercepts.md)
