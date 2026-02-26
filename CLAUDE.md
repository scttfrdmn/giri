# CLAUDE.md - Giri Development Context

## Project Overview
Giri (Go IR Interpreter) is an undefined behavior detector for Go programs. It interprets Go SSA and validates memory operations against shadow memory, similar to Miri for Rust.

## Build & Test
```bash
go build ./...
go test ./...
go test -race ./...
GOEXPERIMENT=arenas go test ./...
```

## Architecture
- `pkg/shadow/` - Shadow memory: allocation tracking, provenance, validation
- `pkg/interpreter/` - SSA interpreter: executes instructions, checks safety
- `pkg/detector/` - Composable checkers: arena, bounds, unsafe, race
- `pkg/scheduler/` - Goroutine scheduling: RoundRobin, Random, PCT
- `pkg/report/` - Output formatting: text, JSON, SARIF
- `internal/ssautil/` - SSA loading from Go packages
- `cmd/giri/` - CLI entry point
- `testdata/` - Programs with known UB for testing

## Key Design Decisions
1. Shadow memory tracks every allocation with provenance metadata
2. Pointer provenance is transitive through unsafe casts and interfaces
3. Detectors are composable and independent of interpreter core
4. Scheduling is pluggable with seeds for reproducible bug reports
5. Grew from safearena's arenacheck - generalized from arena-only to all UB

## Dependencies
- `golang.org/x/tools/go/ssa` - SSA form
- `golang.org/x/tools/go/packages` - Package loading

## Current Phase
Phase 1: Core interpreter + arena safety. SSA instruction walker is scaffolded.
Next priority: complete SSA instruction coverage and integration tests.
