# Giri

**Go IR Interpreter — Undefined Behavior Detection for Go**

Giri interprets Go programs via SSA (Static Single Assignment) form and validates every memory operation against a shadow memory system. It catches classes of bugs that the Go compiler, `go vet`, `-race`, and static analyzers miss.

Named after [Miri](https://github.com/rust-lang/miri) (MIR Interpreter for Rust), Giri fills the same role for Go: an interpretive runtime verifier that understands your program's memory semantics.

## What It Detects

| Category | Example | go vet | -race |
|---|---|---|---|
| Arena use-after-free | Access arena memory after `Free()` | — | — |
| Arena pointer escapes | Return/channel/global arena pointers | — | — |
| Arena leaks | Forgotten `Free()` | — | — |
| `unsafe.Pointer` rule 1 | Misaligned pointer conversion | — | — |
| `unsafe.Pointer` rule 2 | `uintptr` held across GC safepoint | — | — |
| `unsafe.Pointer` rule 3 | Pointer arithmetic past allocation end | — | — |
| Out-of-bounds slice reslice | `s[0:100]` when `cap(s)==4` | — | — |
| Data races | Including channel ordering bugs | — | partial |
| Uninitialized reads | Reading memory before first write | — | — |
| Nil pointer dereference | On uncovered code paths | — | partial |
| Send/close on closed channel | `ch <- v` after `close(ch)` | — | — |
| Double-free | Freeing memory twice | — | — |

## Showcase

The programs in [`testdata/showcase/`](testdata/showcase/) are minimal, self-contained examples that compile cleanly, pass `go vet`, and pass `go test -race` — but Giri detects a real bug in each one. See the [showcase README](testdata/showcase/README.md) for a walkthrough.

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

# Analyze existing TestXxx functions (no standalone main needed)
giri -test ./...

# Arena safety only
giri -arena ./pkg/allocator

# PCT multi-run concurrency sweep (tags violations with replay seed)
giri -strategy pct -runs 100 ./...

# Reproduce a specific PCT run
giri -strategy pct -seed 42 ./...

# JSON output for CI
giri -format json ./... > giri-report.json

# SARIF output for GitHub code scanning
giri -format sarif ./... > giri-results.sarif

# HTML report for human review
giri -format html ./... > giri-report.html

# Detect uninitialized reads (off by default — slower)
giri -init ./...

# Verbose mode (print every SSA instruction)
giri -v ./cmd/server

# Dump SSA for debugging
giri -dump-ssa ./pkg/mypackage
```

### Project Configuration File

Commit your preferred Giri settings to the repository by adding a `.giri.json`
file in the project root. CLI flags always override file values.

```json
{
  "format":   "sarif",
  "strategy": "pct",
  "runs":     100,
  "seed":     42,
  "race":     true,
  "unsafe":   true,
  "arena":    true
}
```

Supported fields mirror CLI flags:

| Field | Type | CLI equivalent |
|-------|------|----------------|
| `format` | string | `-format` |
| `strategy` | string | `-strategy` |
| `seed` | int64 | `-seed` |
| `runs` | int | `-runs` |
| `depth` | int | `-depth` |
| `race` | bool | `-race` |
| `unsafe` | bool | `-unsafe` |
| `arena` | bool | `-arena` |
| `init` | bool | `-init` |
| `verbose` | bool | `-v` |
| `max_steps` | uint64 | `-max-steps` |
| `max_goroutines` | int | `-max-goroutines` |

### Arena Programs

If a package imports `"arena"` but `GOEXPERIMENT=arenas` is not set, those packages cannot be compiled. Giri prints a warning, skips the arena packages, and continues analyzing everything else. Arena-specific checks will produce no findings for the skipped packages.

```
warning: some packages import "arena" but GOEXPERIMENT=arenas is not set.
  Arena analysis is disabled. To enable it, re-run with:
  GOEXPERIMENT=arenas giri ./...
```

To enable full arena analysis, set `GOEXPERIMENT=arenas` — the same flag needed to build the code.

### CI Integration

Giri exits with code 0 (no violations), 1 (violations found), or 2 (load/internal error), making it suitable as a CI gate.

**One-step GitHub Action (#59):**

```yaml
# .github/workflows/giri.yml
name: Giri scan
on: [push, pull_request]
jobs:
  giri:
    runs-on: ubuntu-latest
    permissions:
      contents: read
      security-events: write   # required for SARIF upload
    steps:
      - uses: actions/checkout@v4
      - uses: scttfrdmn/giri/.github/actions/giri@v0.17.0
        with:
          packages: './...'
          upload-sarif: 'true'
```

The action builds and runs Giri, then uploads the SARIF report to GitHub Code Scanning so findings appear as security alerts on your repository.

**Available inputs:**

| Input | Default | Description |
|---|---|---|
| `packages` | `./...` | Go package patterns to analyse |
| `go-version` | `1.23` | Go version used to build Giri |
| `format` | `sarif` | Output format: `text`, `json`, `sarif`, or `html` |
| `output-file` | `giri-results.sarif` | Path for the output file |
| `upload-sarif` | `true` | Upload SARIF to GitHub Code Scanning |
| `fail-on-findings` | `false` | Exit 1 when violations found |
| `extra-flags` | | Additional flags passed to `giri` |

**Manual steps (without the composite action):**

```yaml
- name: Run giri
  run: giri -format sarif ./... > giri-results.sarif || true

- name: Upload SARIF
  uses: github/codeql-action/upload-sarif@v3
  with:
    sarif_file: giri-results.sarif
    category: giri
```

See [`.github/workflows/sarif.yml`](.github/workflows/sarif.yml) for the full workflow used on this repository.

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

── [2] ERROR: unsafe-pointer-rule 2 ──
unsafe-pointer-violation: rule 2: uintptr held across GC safepoint at main.go:40
  uintptr converted from unsafe.Pointer at main.go:36
  GC safepoint (function call) at main.go:39

  hint: Review the six rules at https://pkg.go.dev/unsafe#Pointer.

── [3] ERROR: nil-pointer-deref ──
nil pointer dereference (goroutine 1) at main.go:55

  hint: Check for nil before dereferencing. Map lookups return zero
        values for absent keys.

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
└── testdata/
    ├── showcase/        # Bugs Giri catches that go vet and -race miss
    └── integration/     # (via pkg/interpreter/testdata/integration/)
```

## Implementation Status

### Phase 1: Core Interpreter + Arena Safety ✓
- [x] Shadow memory with allocation tracking and provenance
- [x] Arena lifecycle management (new, free, double-free, escape)
- [x] Use-after-free, arena pointer escape, arena leak detection
- [x] SSA instruction walker (20+ instruction types)
- [x] Deferred call handling (`defer arena.Free()`)
- [x] Goroutine spawning, closures, multi-return
- [x] Report generation (text, JSON, SARIF)
- [x] Integration test suite (178+ tests)

### Phase 2: unsafe.Pointer Rules ✓
- [x] Rule 1: Alignment verification at conversion sites
- [x] Rule 2: `uintptr` liveness tracking across GC safepoints
- [x] Rule 3: Pointer arithmetic bounds checking
- [x] Rule 5: `reflect.Value.Pointer` / `UnsafeAddr` validation
- [x] Rule 6: `SliceHeader` / `StringHeader` manipulation

### Phase 3: Concurrency Verification ✓ (core)
- [x] Vector clock infrastructure (Lamport clocks per goroutine)
- [x] Scheduling strategies (RoundRobin, Random, PCT)
- [x] Data race detection (write/write and write/read conflicts)
- [x] Channel happens-before tracking
- [x] Goroutine spawn happens-before
- [x] `sync.Mutex` / `sync.WaitGroup` / `sync.Once` happens-before
- [x] Send/close on closed channel detection
- [x] PCT replay seeds: `RunN` tags each violation with the seed that triggered it; text report prints `replay: giri -strategy pct -seed N ./...`

### Phase 4: Advanced
- [x] Interprocedural analysis (follow calls across packages) — v0.17.0
- [x] Standard library intercepts (60+ packages: `strings`, `strconv`, `fmt`, `time`, `os`, `math/rand`, `sync`, `bytes`, `errors`, `sort`, `regexp`, `encoding/json`, `net/http`, `database/sql`, `crypto/tls`, and more) — v0.11–0.30
- [ ] `cgo` boundary checking
- [ ] `reflect` package safety verification
- [ ] `go:linkname` tracking
- [x] `giri -test ./...`: discovers and runs `TestXxx(*testing.T)` functions from `_test.go` files — v0.33.0
- [x] Context cancel leak detection: `context.WithCancel/WithTimeout/WithDeadline` cancel functions tracked; uncalled ones reported as `context-cancel-leak` — v0.34.0
- [x] HTML report format: `-format html` produces self-contained HTML with color-coded findings — v0.34.0
- [x] Nil channel operation detection: `close(nil)`, send on nil, receive from nil — reported as `nil-channel` — v0.35.0
- [x] `make()` negative argument detection: negative len/cap → reported as `make-invalid` — v0.35.0
- [x] String index out-of-bounds detection: `s[i]` where `i >= len(s)` — reported as `out-of-bounds` — v0.36.0
- [x] Negative shift count detection: `x << n` where `n < 0` — reported as `negative-shift` — v0.36.0
- [x] Nil slice element access detection: `s[i]` on a nil slice — reported as `out-of-bounds` — v0.37.0
- [x] Unlock of unlocked mutex: `mu.Unlock()` / `mu.RUnlock()` without prior lock — reported as `mutex-unlock` — v0.37.0
- [x] `unsafe.Slice` negative length detection: `unsafe.Slice(ptr, n)` where `n < 0` — reported as `unsafe-slice` — v0.38.0
- [x] `unsafe.Slice` nil pointer detection: `unsafe.Slice(nil, n)` where `n != 0` — reported as `unsafe-slice` — v0.38.0
- [x] `FieldAddr` nil struct pointer detection: `var p *T; _ = p.Field` — reported as `nil-pointer-deref` — v0.39.0
- [x] `unsafe.String` argument validation: negative length or nil pointer with non-zero length — reported as `unsafe-slice` — v0.39.0
- [x] Array pointer bounds detection: `p[i]` where `p` is `*[N]T` and `i >= N` — reported as `out-of-bounds` — v0.40.0
- [x] Slice element OOB beyond declared length: `s[i]` where `i >= len(s)` even if `i < cap(s)` — reported as `out-of-bounds` — v0.41.0
- [x] `make([]T, len, cap)` with len > cap detection — reported as `make-invalid` — v0.41.0
- [x] `make(map[K]V, n)` negative size hint detection: `n < 0` → reported as `make-invalid` — v0.42.0
- [x] Range-over-array: `for i, v := range [N]T{}` now executes the loop body (fixes silent skip that hid violations) — v0.42.0
- [x] `len(map)` / `len(chan)` / `cap(chan)` now return correct values (fixes false-positive violations in non-empty guards) — v0.43.0
- [x] Integer truncation in `ssa.Convert`: `int8(300)=44`, `int8(256)=0` etc. now apply correct bit-width semantics — v0.43.0
- [x] `token.AND_NOT` (`&^`) bit-clear operator now evaluated correctly in `evalBinOp` — v0.44.0
- [x] Complex number support: `real()`, `imag()`, `complex()` builtins + `complex128` arithmetic (`+`, `-`, `*`, `/`, `==`, `!=`) — v0.44.0
- [x] `string → []rune` and `[]rune → string` conversions in `convertValue` (Unicode text-processing) — v0.45.0
- [x] `for x := range ch {}` range-over-channel: `CommaOk` receive now returns `ok=false` when channel is closed and drained (was: always `ok=true`, causing infinite loop) — v0.45.0
- [x] Complex128 unary negation: `-c` where `c` is `complex128` now correctly returns `-c` (was: returned `c` unchanged) — v0.46.0
- [x] `complex64 ↔ complex128` type conversions in `convertValue`; defensive `int/float → complex` — v0.46.0
- [x] `ssa.Select` receive readiness: buffered-channel `pendingCount` and closed-channel state now checked; `recvOk` formula aligned with `token.ARROW CommaOk` fix from v0.45.0 — v0.46.0
- [x] `handleLoad` uninitialized global: false out-of-bounds replaced by correct nil-pointer-deref; returning `Value{}` for offset-0 miss in `valueStore` — v0.47.0
- [x] `golang.org/x/tools/go/packages` intercept: `Load` returns empty list; prevents false violations in programs that import go/packages — v0.47.0
- [x] Package `init()` called before `main()`: synthesized init now runs before main(); dependency package inits suppressed; `flag.*` intercepts preserve default values; `handleLoad` dereferences native Go primitive pointers — v0.48.0
- [x] `LoadAllPrograms`: unguarded `initial[0].Fset` before `len` check eliminated; Giri now reports 0 violations on its own source tree (`giri ./...`) — v0.49.0
- [x] `slices`, `maps`, `cmp`, `log/slog` intercepts (Go 1.21+); generic instantiation fix: `callee.Origin()` used when `callee.Package()==nil` so all generic stdlib calls are intercepted — v0.50.0

## Contributing

Contributions are welcome. See [CONTRIBUTING.md](CONTRIBUTING.md) for:

- Development setup (`GOEXPERIMENT=arenas go test ./...`)
- Commit conventions and PR workflow
- How to add a stdlib intercept
- How to add an integration test

Please report security issues via the [GitHub private security advisory](https://github.com/scttfrdmn/giri/security/advisories)
rather than as public issues. See [SECURITY.md](SECURITY.md) for details.

## Requirements

- Go 1.23+

## License

Apache 2.0 — see [LICENSE](LICENSE)

## Credits

Inspired by [Miri](https://github.com/rust-lang/miri) for Rust and built on the foundation of [SafeArena](https://github.com/scttfrdmn/safearena)'s arenacheck analyzer.

Built by [@scttfrdmn](https://github.com/scttfrdmn)
