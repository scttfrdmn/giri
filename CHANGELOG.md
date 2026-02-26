# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.3.0] - 2026-02-25

### Added

- **unsafe.Pointer Rule 1 – alignment check** (`pkg/interpreter/interpreter.go`):
  `handleUnsafePointer` now accepts `targetType types.Type` and `valueID string`.
  For `UnsafeOpFromPointer` (unsafe.Pointer → *T) it verifies that the pointer's
  byte offset is divisible by `types.Sizes.Alignof(T)`, firing `RuleConversion`
  when misaligned.
- **unsafe.Pointer Rule 2 – uintptr GC point tracking** (`pkg/detector/detector.go`,
  `pkg/interpreter/exec.go`):
  - `UnsafeDetector.ClearAllUintptrConversions()` clears all pending uintptr entries.
  - `Registry` gains `unsafeDetector *UnsafeDetector` field extracted in `NewRegistry`;
    four delegation methods added: `RecordUintptrConversion`, `ClearUintptrConversion`,
    `ClearAllUintptrConversions`, `CheckGCPoint`.
  - `execCall` calls `registry.CheckGCPoint(site)` before every non-builtin function
    call; any live uintptr at that point fires `RuleUintptr`.
  - `UnsafeOpToUintptr` in `handleUnsafePointer` now calls `registry.RecordUintptrConversion`.
  - `UnsafeOpArithmetic` (uintptr → unsafe.Pointer) now calls `registry.ClearAllUintptrConversions`.
- **Channel happens-before tracking** (`pkg/interpreter/interpreter.go`, `exec.go`):
  Added `ChanID` type, `chanEntry` struct (stores sender GID + clock snapshot),
  `channels map[ChanID]*chanEntry`, `nextChanID atomic.Uint64`, and `createChannel()`.
  `MakeChan` now allocates a real `ChanID`; `Send` extracts the `ChanID` and records
  the sender's vector clock via `handleChannelSend`; channel receive (`<-ch`) extracts
  the `ChanID` and merges the sender's clock via `handleChannelRecv`.
- **Integration tests** (3 new programs in `testdata/integration/`):
  - `uintptr_gc` — uintptr survives to a function call (GC point); expects 1 Rule 2
    violation.
  - `safe_uintptr` — uintptr immediately converted back, no intervening call; expects
    0 violations.
  - `misaligned_ptr` — unsafe.Add offset 1, convert to `*uint32`; expects 1 Rule 1
    violation.

### Changed

- `handleUnsafePointer` signature extended with `targetType types.Type, valueID string`.
- `handleChannelSend` signature changed from `(gid, val, site)` to `(gid, chanID, val, site)`.
- `handleChannelRecv` signature changed from `(gid, senderGID, site)` to `(gid, chanID, site)`.
- `Registry.NewRegistry` now scans the detector list to extract the `*UnsafeDetector`
  for direct delegation (no public API change).

## [0.2.0] - 2026-02-25

### Added

- **Constant value representation** (`pkg/interpreter/exec.go`): `constToValue` now uses
  `go/constant` to extract typed values (`int64`, `float64`, `bool`, `string`) instead of
  returning raw strings, enabling correct arithmetic.
- **Accurate type sizing** (`pkg/interpreter/interpreter.go`): Replaced `estimateTypeSize`
  string-matching heuristic with `go/types.Sizes` for the target platform (`gc`/`GOARCH`).
  Field offsets in `FieldAddr` and element sizes in `IndexAddr` now use `types.Sizes.Offsetsof`.
- **Phi node predecessor tracking**: Added `PrevBlock *ssa.BasicBlock` to `Frame`; `execFunction`
  tracks the previous block so Phi edges are resolved correctly rather than taking the first
  available value.
- **New SSA instruction coverage** (15 new cases): `BinOp`, `Extract`, `Field`, `Index`,
  `Lookup`, `MapUpdate`, `MakeSlice`, `Slice`, `MakeMap`, `MakeChan`, `Range`, `Next`,
  `TypeAssert`, `ChangeInterface`, `MultiConvert`, `SliceToArrayPointer`, `Select`.
- **Builtin interception** (`execBuiltin`): Handles `unsafe.Add`, `len`, `cap`, `append`,
  `copy`, `delete`, `close`, `recover`, `unsafe.Slice` by dispatching on `*ssa.Builtin`
  before the normal call path.
- **Detector registry wiring**: `New()` builds a `*detector.Registry` from config flags and
  `handleLoad`/`handleStore` now call `registry.CheckAccess` after the base `CheckAccess`.
  `Finish()` calls `registry.Finalize` (replaces manual arena leak loop).
- **Scheduler wiring**: `New()` instantiates the configured `scheduler.Scheduler`;
  `ssa.Go` now enqueues tasks in `runQueue`; `Run()` drains the queue via the scheduler
  after `main` completes.
- **Verbose access logging fix** (`pkg/shadow/memory.go`): `AccessLog` is initialized in
  `Allocate` when verbose mode is enabled; `CheckAccess` records each access using a
  per-allocation `logMu` mutex (no global write-lock upgrade needed).
- **Unit tests**: `pkg/shadow/memory_test.go`, `pkg/shadow/errors_test.go`,
  `pkg/detector/detector_test.go`, `pkg/scheduler/scheduler_test.go`.
- **Integration tests**: `pkg/interpreter/interpreter_integration_test.go` with four
  programs in `pkg/interpreter/testdata/integration/`: `safe_alloc` (0 violations),
  `unsafe_oob` (1 violation, unsafe.Pointer OOB), `binop` (0 violations), `multi_return`
  (0 violations).

### Changed

- `Frame` struct gains `PrevBlock *ssa.BasicBlock` field.
- `Interpreter` struct gains `sizes`, `registry`, `sched`, `runQueue` fields.
- `Return` instruction stores multi-value results as `[]Value` tuple (was single value only).
- `UnOp` handler uses `token.XXX` constants and handles `token.ARROW` (channel receive).
- `handleAlloc` uses a conservative 8-byte default (callers with concrete types use
  `Memory.Allocate` + `typeSizeOf` directly).

### Fixed

- `unsafe.Add` was silently ignored (treated as `<dynamic>` call); now correctly intercepted
  via `*ssa.Builtin` dispatch and triggers `UnsafePointerViolation` on out-of-bounds arithmetic.

## [0.1.0] - 2026-02-25

### Added

- Shadow memory system (`pkg/shadow`) with allocation tracking, pointer provenance,
  arena lifecycle management, and optional per-byte initialization tracking.
- Structured error types for all violation categories: use-after-free, double-free,
  arena double-free, out-of-bounds, uninitialized read, unsafe.Pointer violations (all
  six rules defined), arena pointer escape, and data race.
- SSA interpreter (`pkg/interpreter`) with support for core instruction types: `Alloc`,
  `Store`, `UnOp`, `FieldAddr`, `IndexAddr`, `Jump`, `If`, `Return`, `Panic`, `Call`,
  `Defer`, `Go`, `Send`, `Convert`, `ChangeType`, `MakeInterface`, `Phi`, `DebugRef`.
- Deferred call handling with correct LIFO execution order, including `arena.Free()`
  interception.
- Vector clock infrastructure for happens-before tracking in the interpreter.
- Composable detector framework (`pkg/detector`) with `ArenaDetector`, `BoundsDetector`,
  `UnsafeDetector`, and `RaceDetector`.
- Goroutine scheduling strategies (`pkg/scheduler`): `RoundRobin`, `Random` (seeded),
  and `PCT` (Probabilistic Concurrency Testing, Burckhardt et al. 2010).
- Report package (`pkg/report`) with structured `Finding` types, text and JSON output
  formats, severity levels, and CI-friendly exit codes.
- SSA loader utility (`internal/ssautil`) for loading Go packages into SSA form,
  including test package support.
- CLI entry point (`cmd/giri`) with flags for detector selection, scheduling strategy,
  output format, and execution limits.
- `BugDepth` configuration field wired from `-depth` CLI flag to PCT scheduler.
- `testdata/ub_patterns.go` documenting the UB patterns Giri is designed to detect.
- `CHANGELOG.md` in keepachangelog format.

### Fixed

- Removed seven root-level `.go` files that declared conflicting package names,
  causing `go build` to fail with "found packages X and Y" errors.
- Moved `report.go` from the root directory to `pkg/report/report.go`, resolving
  the missing import for `github.com/scttfrdmn/giri/pkg/report`.
- Generated `go.sum` via `go mod tidy`.

[Unreleased]: https://github.com/scttfrdmn/giri/compare/v0.3.0...HEAD
[0.3.0]: https://github.com/scttfrdmn/giri/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/scttfrdmn/giri/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/scttfrdmn/giri/releases/tag/v0.1.0
