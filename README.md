# Giri

**Go IR Interpreter — Undefined Behavior Detection for Go**

Giri interprets Go programs via SSA (Static Single Assignment) form and validates every memory operation against a shadow memory system. It catches classes of bugs that the Go compiler, `go vet`, `-race`, and static analyzers miss.

Named after [Miri](https://github.com/rust-lang/miri) (MIR Interpreter for Rust), Giri fills the same role for Go: an interpretive runtime verifier that understands your program's memory semantics.

## What It Detects

| Category | Example | Existing Go Tool |
|---|---|---|
| Arena use-after-free | Access arena memory after `Free()` | None |
| Arena pointer escapes | Return/channel/global arena pointers | `arenacheck` (static, limited) |
| Arena leaks | Forgotten `Free()` | `runtime.SetFinalizer` (delayed) |
| `unsafe.Pointer` rule violations | All 6 rules from Go spec | `go vet` (partial) |
| Out-of-bounds via unsafe | Pointer arithmetic past allocation | None |
| Data races | Including channel ordering bugs | `-race` (partial) |
| Uninitialized reads | Reading memory before first write | None |
| Double-free | Freeing memory twice | None |

## Architecture

```
┌──────────┐     ┌──────────────┐     ┌───────────────┐
│ Go Source │────►│  SSA Loader  │────►│  Interpreter   │
└──────────┘     └──────────────┘     │                │
                                      │  For each SSA  │
                                      │  instruction:  │
                                      │  1. Execute    │
                                      │  2. Validate   │
                                      └───────┬───────┘
                                              │
                        ┌─────────────────────┼─────────────────────┐
                        │                     │                     │
                  ┌─────▼─────┐      ┌───────▼───────┐    ┌───────▼───────┐
                  │  Shadow    │      │  Detectors    │    │  Scheduler    │
                  │  Memory    │◄─────│               │    │               │
                  │            │      │ • Arena       │    │ • RoundRobin  │
                  │ • Allocs   │      │ • Bounds      │    │ • Random      │
                  │ • Arenas   │      │ • Unsafe      │    │ • PCT         │
                  │ • Pointers │      │ • Races       │    │               │
                  │ • Init     │      │ • Custom...   │    │               │
                  └────────────┘      └───────────────┘    └───────────────┘
                                              │
                                      ┌───────▼───────┐
                                      │    Report     │
                                      │  • Text/JSON  │
                                      │  • SARIF      │
                                      │  • CI exit    │
                                      └───────────────┘
```

### Key Design Decisions

**Shadow Memory** (`pkg/shadow/`): Every allocation in the interpreted program gets an `AllocID` with metadata — provenance, bounds, lifecycle state, and initialization bits. Every pointer carries a `Pointer` struct linking it to its source allocation. This is directly inspired by Miri's allocation tracking.

**Provenance Tracking**: Pointer provenance is transitive. If you take an arena pointer, cast it through `unsafe.Pointer`, store it in an interface, and extract it later — it still knows it came from the arena. This is what makes Giri more powerful than static analysis.

**Composable Detectors** (`pkg/detector/`): Each class of UB is checked by an independent module. You can enable/disable detectors individually, and adding new checks doesn't touch the interpreter core.

**Pluggable Scheduling** (`pkg/scheduler/`): Concurrency bugs depend on execution order. Giri supports multiple scheduling strategies — from deterministic (round-robin) to probabilistic (PCT) — with seeds for reproducible bug reports.

## Usage

```bash
# Install
go install github.com/scttfrdmn/giri/cmd/giri@latest

# Check a package
giri ./...

# Arena safety only
giri -arena ./pkg/allocator

# With PCT scheduling for concurrency bugs
giri -strategy pct -seed 42 -depth 3 ./...

# JSON output for CI
giri -format json ./... > giri-report.json

# Verbose mode (print every SSA instruction)
giri -v ./cmd/server

# Dump SSA for debugging
giri -dump-ssa ./pkg/mypackage
```

## Example Output

```
╔══════════════════════════════════════════════╗
║  Giri - Go IR Interpreter                    ║
║  Undefined Behavior Detection Report          ║
╚══════════════════════════════════════════════╝

Found 3 violation(s):

── [1] ERROR: use-after-free ──
use-after-free: access to *Record (alloc 7) at main.go:25
  allocated at: main.go:22
  freed at:     main.go:24 (arena 1 freed)

  hint: Arena-allocated pointer was used after arena.Free().
        Use safearena.Clone() to copy to heap.

── [2] ERROR: arena-escape ──
arena pointer escape: return (alloc 12, arena 2) escapes via return at server.go:48
  allocated at: server.go:45

  hint: Copy to heap with Clone() before the arena is freed.

── [3] ERROR: unsafe-pointer-rule 3: pointer arithmetic must stay within allocation ──
unsafe.Pointer violation (rule 3) at parser.go:112
  unsafe.Add moved pointer to offset 1048576, allocation is 32 bytes

  hint: Review the six rules at https://pkg.go.dev/unsafe#Pointer.

── Summary ──
  ERROR: 3

── Memory ──
  allocations: 0 live / 45 total (0 bytes), arenas: 0 live / 2 total, pointers: 38 tracked
```

## Relationship to SafeArena

Giri grew out of [SafeArena](https://github.com/scttfrdmn/safearena)'s `arenacheck` static analyzer. Where arenacheck performs static SSA analysis to approximate provenance, Giri executes the SSA and tracks provenance concretely. This eliminates false positives and handles dynamic dispatch, closures, and interfaces that static analysis can't follow.

SafeArena provides **runtime wrappers** (add safety to your code). Giri provides **interpretive verification** (find bugs in existing code). They're complementary:

| | SafeArena | Giri |
|---|---|---|
| Approach | Runtime wrappers | SSA interpretation |
| Requires code changes | Yes (`Ptr[T]`, `Scoped`) | No |
| False positives | None (runtime) | Very low (concrete execution) |
| Performance | Runs at near-native speed | 10-100x slower (interpretation) |
| Scope | Arena safety only | Arena + unsafe + races + bounds |
| When to use | Production code | Testing/CI |

## Project Structure

```
giri/
├── cmd/giri/           # CLI entry point
├── pkg/
│   ├── interpreter/    # SSA interpreter core
│   │   ├── interpreter.go  # Value types, frame management, instruction handlers
│   │   └── exec.go         # SSA instruction walker, call dispatch
│   ├── shadow/         # Shadow memory system
│   │   ├── memory.go       # Allocation tracking, provenance, validation
│   │   └── errors.go       # UB error types with structured diagnostics
│   ├── detector/       # Composable safety checkers
│   │   └── detector.go     # Arena, bounds, unsafe, race detectors
│   ├── scheduler/      # Goroutine scheduling strategies
│   │   └── scheduler.go    # RoundRobin, Random, PCT
│   └── report/         # Output formatting
│       └── report.go       # Text, JSON, SARIF reports
├── internal/
│   └── ssautil/        # SSA loading utilities
│       └── loader.go       # Package → SSA conversion
└── testdata/           # Programs with known UB
    └── ub_patterns.go      # Arena, unsafe, race test cases
```

## Implementation Status

### Phase 1: Core Interpreter + Arena Safety ← **Current**
- [x] Shadow memory with allocation tracking
- [x] Pointer provenance (transitive through casts)
- [x] Arena lifecycle management
- [x] Use-after-free, double-free, escape detection
- [x] SSA instruction walker
- [x] Deferred call handling (critical for `defer arena.Free()`)
- [x] Report generation (text + JSON)
- [ ] Full SSA instruction coverage
- [ ] Integration test suite

### Phase 2: unsafe.Pointer Rules
- [x] Rule 3: Pointer arithmetic bounds checking
- [x] Error types for all 6 rules
- [ ] Rule 1: Alignment verification
- [ ] Rule 2: uintptr liveness tracking across GC points
- [ ] Rule 5: reflect.Value.Pointer validation
- [ ] Rule 6: SliceHeader/StringHeader manipulation

### Phase 3: Concurrency Verification
- [x] Vector clock infrastructure
- [x] Scheduler strategies (RoundRobin, Random, PCT)
- [x] Data race detector framework
- [ ] Channel happens-before tracking
- [ ] Mutex happens-before tracking
- [ ] sync.Pool lifetime verification
- [ ] Systematic interleaving exploration

### Phase 4: Advanced
- [ ] Interprocedural analysis (follow calls across packages)
- [ ] cgo boundary checking
- [ ] reflect package safety verification
- [ ] `go:linkname` tracking
- [ ] GC safepoint simulation
- [ ] SARIF output for IDE integration
- [ ] `go test -giri` integration

## Requirements

- Go 1.23+
- `GOEXPERIMENT=arenas` for arena-related checks

## License

Apache 2.0 — see [LICENSE](LICENSE)

## Credits

Inspired by [Miri](https://github.com/rust-lang/miri) for Rust and built on the foundation of [SafeArena](https://github.com/scttfrdmn/safearena)'s arenacheck analyzer.

Built by [@scttfrdmn](https://github.com/scttfrdmn)
