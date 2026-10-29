# Session log

## 2026-10-28 — pkg/exec for tumble (GIRI_REQUIREMENTS.md reqs 1–11)

Branch `feat/tumble-exec` (on top of `feat/autostub-225`), 11 commits, not
pushed. Each commit carries `Closes #N`, so the issues close when the branch
merges to `main`; each issue has a comment naming the commit and its tests.

### What exists

- **`pkg/exec`**: `Run`, `LoadDir`, `Options`
  (`MaxSteps`, `TrackInit`, `Trace`, `TraceCap`, `Cache`, `SingleGoroutine`,
  `SkipDetectors`, `Seed`), `Result` (`Outputs`, `Outcomes`, `Steps`,
  `StepsPerSet`, `StepsByClass`, `Violations`, `Exhausted`, `Traces`,
  `CacheHits`, `PureExecuted`), `CacheHook`, `IsPure`, `IsConcrete`,
  `ConcreteFuncs`, `Class`, `Version`, and typed errors `*UnsupportedTypeError`
  and `*ArgError`. Reference: `docs/exec-api.md`.
- **`pkg/interpreter` concrete mode and sessions**:
  - `NewSession`/`Session.Call`, never used by the CLI;
  - value-faithful integer, float and memory semantics (`concrete.go`);
  - a native-backed table of concrete stdlib functions (`concrete_intercepts.go`: `math`, `math/bits`, `strings`, `strconv`, `unicode`, `unicode/utf8`, `sort`);
  - per-class counters, value-cache hook, tracer and step budget (`session.go`);
  - `IsPure` and `ClassOf`.
- **Fix #249**: same-module calls in dotless module paths are interpreted.
- Docs: `docs/GIRI_REQUIREMENTS.md` (copied from tumble), `docs/exec-api.md`,
  `docs/decisions.md` (every departure, with reasons), `docs/perf.md`,
  README section, CHANGELOG [Unreleased].
- Filed **#250**: CLI-path `math/bits` narrow-width and `sort` intercepts return
  wrong values (pkg/exec unaffected).

### What is tested (all in `pkg/exec` unless noted)

| Req | Issue | Tests |
|---|---|---|
| 1 library entry | #238 | `TestRoundTripMatchesNative` (35 kernels vs native via `reflect.DeepEqual`), `TestPanicOutcome`, `TestUnsupportedTypes` |
| 11 value-faithful memory | #248 | covered by the round-trip kernels (slice sums, prefix sums, local arrays, struct fields) |
| 7 batch build | #244 | `TestLoadDirManyFunctions` (300 functions, one load, all called), `TestLoadDirErrors` |
| 4 class counters | #241 | `TestStepsByClass` (hand-counted SSA mixes, exact), `TestClassOf` |
| 2 traces | #239 | `TestTraces`, `TestTraceCapTruncates`, `TestTraceKindsAndOff` |
| 3 value cache | #240 | `TestCacheSecondRunExecutesNoPureInstructions`, `TestCacheNotStoredOnViolation`, `TestIsPure` |
| 5 concreteness | #242 | `TestIsConcrete`, `TestConcreteFuncsExist` |
| 6 fast paths | #243 | `TestFastPathsPreserveResults`, `TestSingleGoroutineRejectsGo`, `TestSkipDetectorsStillPanics`, `BenchmarkIntLoop`, `BenchmarkSliceLoop` |
| 8 determinism | #245 | `TestDeterministicResults` (byte-identical over 6 runs), `TestSeedChangesRandomness`, `TestMapIterationSorted` |
| 9 budget | #246 | `TestBudgetExhaustion` |
| 10 stability | #247 | `TestVersion`; full existing suite unchanged |
| — dotless paths | #249 | `internal/ssautil` `TestDotlessModuleCallInterpreted`, `pkg/interpreter` `TestIsStdlibOrKnownExternalPkg` |

The full suite (`GOEXPERIMENT=arenas go build/vet/test ./...`) passes at HEAD
and after each commit that touched `pkg/interpreter`.

### Perf measured (Apple M4 Pro, arm64, go1.27.3; details in docs/perf.md)

- Straight-line integer loop: **62.6 ns/instr** with every option combination
  (169 ns/instr before the session SSA-name memo).
- Load/store loop: **~320 ns/instr** by default, **75 ns/instr** with
  `SkipDetectors` (4.3×).
- `SingleGoroutine`: no measurable effect.
- `LoadDir`: ~30 ms for 300 functions without imports, ~160 ms for a
  package importing seven stdlib packages.

### Blocked / open

- **amd64 numbers**: not measured, since no amd64 machine was available.
- **x/tools v0.41 cannot read go1.27 export data.** `LoadDir` type-checks
  dependencies from source instead (slower with many imports). giri declares
  Go 1.26 (`go.mod`, CI matrix 1.25/1.26). It builds and passes on go1.27.3,
  but moving to 1.27 (x/tools bump, CI matrix) is a separate change not made
  here.
- **Consumers must build with `GOEXPERIMENT=arenas`**, like all of giri.
- **Sub-DAG (Hashlife) caching is not achievable with the per-instruction
  hook.** It saves only the hit instruction. Skipping whole backward slices
  needs lazy operand evaluation, a design decision for tumble and giri
  together (#240).
- **CLI value-faithfulness** (the CLI half of #248) was deliberately not done;
  it changes CLI results and needs its own decision.
- **#250** (CLI intercept bugs) is filed, not fixed.
- **v0 marshalling limits**: no structs or maps as arguments/results, no
  interpretation of non-stdlib dependencies, `cap()` after `append` growth
  can be smaller than native (`docs/decisions.md`).
- **Next performance step**: slot-indexed frames instead of name-keyed
  `Locals` maps. It touches every instruction handler the CLI shares.
- Not posted: the summary comment on scttfrdmn/tumble#14 that the
  requirements doc's own prompt asks for (this session's brief asked for
  this log instead).
