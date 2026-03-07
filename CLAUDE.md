# CLAUDE.md - Giri Development Context

## Project Overview
Giri (Go IR Interpreter) is an undefined behavior detector for Go programs. It interprets Go SSA and validates memory operations against shadow memory, similar to Miri for Rust.

**Current version: v0.90.0** — mature, with 259 integration tests and intercepts for 170+ stdlib packages.

## Build & Test
```bash
GOEXPERIMENT=arenas go build ./...
GOEXPERIMENT=arenas go test ./...
go vet ./...
```

Target always: zero failing tests, zero vet warnings.

## Architecture
- `pkg/shadow/` - Shadow memory: allocation tracking, provenance, validation
- `pkg/interpreter/` - SSA interpreter: executes instructions, checks safety
  - `interpreter.go` - core run loop, goroutine scheduling, init()
  - `exec.go` - SSA instruction dispatch (TypeAssert, Call, BinOp, …)
  - `stdlib.go` - stdlib intercept handlers (170+ package paths)
- `pkg/detector/` - Composable checkers: arena, bounds, unsafe, race
- `pkg/scheduler/` - Goroutine scheduling: RoundRobin, Random, PCT
- `pkg/report/` - Output formatting: text, JSON, SARIF, HTML
- `internal/ssautil/` - SSA loading from Go packages
- `internal/tools/tools.go` - `//go:build tools` anchor for x/ deps in go.mod
- `cmd/giri/` - CLI entry point
- `testdata/showcase/` - 14 curated UB demonstrations
- `pkg/interpreter/testdata/integration/` - 259 integration test programs

## Dependencies
- `golang.org/x/tools` - SSA form + package loading
- `golang.org/x/text`, `golang.org/x/crypto`, `golang.org/x/net`, `golang.org/x/sys`, `golang.org/x/mod`, `golang.org/x/sync` - extended stdlib intercepts (direct deps via internal/tools/tools.go)

## Key Design Decisions
1. Shadow memory tracks every allocation with provenance metadata
2. Pointer provenance is transitive through unsafe casts and interfaces
3. Detectors are composable and independent of interpreter core
4. Scheduling is pluggable with seeds for reproducible bug reports
5. `stdlibOpaque` sentinel (`Value{Raw: struct{}{}}`) for opaque stub returns
6. Generic intercept: when `callee.Package()==nil`, fall back to `callee.Origin()` for package/name

## Adding a New Stdlib Intercept

1. Find the handler for the package in `stdlib.go` (search for `handleXxxCall`)
2. If no handler exists, add a new `handleXxxCall` function and a routing case in `execStdlibCall`
3. Add the new `case "FuncName":` returning an appropriate stub value:
   - Package-level constructors returning `(T, error)` → `Value{Raw: []Value{opaque, {}}}`
   - Methods returning nothing → `Value{}, true`
   - Methods returning `bool` → `Value{Raw: false}, true` (conservative)
   - Methods returning `string` → `Value{Raw: ""}, true`
4. **Verify the function actually exists** with `go doc <pkg> <FuncName>` before adding
5. Create an integration test in `testdata/integration/<name>/main.go`
6. Add the test table entry in `interpreter_integration_test.go`

## Release Checklist

Before tagging a new version, verify ALL of the following:

### 1. Accurate test counts
```bash
# Integration test count (excludes showcase):
sed -n '14,<showcaseTests_line>p' pkg/interpreter/interpreter_integration_test.go | grep -c 'dir:'

# Showcase count:
ls testdata/showcase/ | grep -v README | wc -l

# Quick combined check:
grep -n '^var showcaseTests' pkg/interpreter/interpreter_integration_test.go
# then: sed -n '14,<that_line-1>p' ... | grep -c 'dir:'
```
**Current baseline**: 259 integration + 14 showcase = 273 total (as of v0.90.0)

### 2. CHANGELOG entry format
```markdown
## [X.Y.Z] - YYYY-MM-DD

### Added
- **`pkg/name` completions** (issue #NNN): brief description. N new intercepts.
- Integration test `test_name`: exercises new intercepts; 0 violations.
- N new integration test(s) (255 integration + 14 showcase = 269 total).
```
Use **actual grep counts** — never estimate or carry forward from memory.

### 3. README accuracy
- GitHub Action tag in README: `uses: scttfrdmn/giri/.github/actions/giri@vX.Y.Z` — update on each release
- stdlib coverage count ("170+ packages") — update when a major milestone is crossed
- Check: `grep -n 'actions/giri@' README.md`

### 4. Build & vet clean
```bash
GOEXPERIMENT=arenas go build ./... && go vet ./...
```

### 5. Tag and release
```bash
git tag vX.Y.Z && git push && git push --tags
gh release create vX.Y.Z --title "vX.Y.Z" --notes "..."
gh issue close NNN --repo scttfrdmn/giri
```

## Integration Test Conventions

- Test programs live in `pkg/interpreter/testdata/integration/<dir>/main.go`
- Package: `package main`; no external test framework
- Expected violations declared in the test table (`wantViolations`, `wantCategory`)
- For 0-violation tests: avoid any operation that legitimately triggers a detector
  - `reflect.Value.Pointer()`/`UnsafeAddr()` return `uintptr` → correctly triggers Rule 5 — omit from 0-violation tests
  - Misaligned unsafe.Pointer access → Rule 1 — only use in tests expecting violations
- `wantCategory` matches either the hyphenated report category (e.g. `"out-of-bounds"`) or a substring of `err.Error()`

## Common Gotchas

- `unsafe.Add`, `len`, `cap`, `append` etc. are `*ssa.Builtin` — `StaticCallee()` returns nil
- SSA globals are pointer-typed: `var x int` → `*ssa.Global` with type `*int`
- `gzip.NewWriter(w)` / `zlib.NewWriter(w)` return a **single** value (no error)
- `net/http/httputil.NewSingleHostReverseProxy` NOT `NewReverseProxy` (doesn't exist)
- `testing/synctest.Test(t,f)` + `Wait()` — NOT `synctest.Run()`
- `syscall.Getenv` returns `(string, bool)` NOT `string` (unlike `os.Getenv`)
- `sync` package methods (Mutex.Lock, WaitGroup.Add, etc.) are handled in `exec.go`, NOT in `stdlib.go`
- `handleReflectCall` uses local `opaque := Value{Raw: struct{}{}}`, NOT `stdlibOpaque`
- Integration test programs must NOT have a local `go.mod` — they're part of the module
- Closure-captured channels don't work for channel-race testing — pass channels as explicit args
- Race tests: use **sibling goroutines** (both spawned from main), NOT parent-child
- Division-by-zero detection requires a **direct literal `0` argument** — zero values from slice range iteration are NOT tracked through SSA
