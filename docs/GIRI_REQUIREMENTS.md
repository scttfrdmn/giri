# What tumble needs from giri

giri is a Go SSA interpreter built for undefined-behavior detection: shadow memory, provenance, composable detectors, pluggable goroutine scheduling, step budgets, and concrete intercepts for most of the standard library. Everything tumble needs is a view onto machinery giri already has. Nothing here changes what `giri ./...` does from the command line.

## The role giri plays

tumble's fitness function must evaluate hundreds of thousands to millions of candidate Go functions per run. Compiling each one with `go build` costs 0.5–2 s and makes that budget impossible. giri interprets SSA directly, so a candidate is evaluated by building one package per generation (one `go/ssa` build for ~256 functions) and interpreting each function on a fixed set of input vectors. Interpretation is 10–100× slower than native, but the inputs are kernel-sized, so a candidate costs microseconds to low milliseconds, and most of each candidate is inherited from parents that were already evaluated — which is what the value cache below exploits.

giri also does three things no compile-and-run evaluator can:

- Its step counter is a deterministic, machine-independent cost. tumble uses executed instruction count as the primary performance fitness for the whole population, and only runs native `go test -bench` on a few elites. This removes benchmark noise from selection entirely.
- Its detectors reject candidates that pass their tests by accident: out-of-bounds reads that land on valid memory, uninitialized reads that happen to be zero. Differential testing cannot see those.
- Its `unsafe.Pointer` rules and shadow memory are what will later allow `unsafe` into the search grammar at all. STOKE searched over raw x86 because an SMT solver verified the result; tumble will search over unsafe Go because giri verifies the memory semantics.

## Requirements, in prose

**1. Library entry point.** A package (say `pkg/exec` or an exported surface on `pkg/interpreter`) that takes an already-built `*ssa.Function`, a slice of argument sets expressed as ordinary Go values, and run options, and returns for each argument set the function's return values as Go values, plus the aggregate instrumentation below. Today giri runs `main()` or `TestXxx`; tumble needs to call an arbitrary exported function with constructed arguments. Marshalling in v0 covers: integers of all widths, bools, floats, strings, slices of those, and nil. Structs, maps, channels, funcs, interfaces are out of scope for v0 and should return a typed error.

**2. Per-value traces.** For every `ssa.Value` in the function that has a result (every instruction except stores, jumps, returns, etc.), the sequence of concrete values it took during execution, per argument set. For values inside loops that is one entry per execution of the defining instruction. Phi nodes included. The trace is the raw material for tumble's semantic fingerprints; the fingerprint is computed on tumble's side, so giri only needs to record. Recording must be switchable off per run (it is the dominant memory cost) and bounded (a per-value cap on sequence length, after which the value is marked truncated — tumble treats truncated values as having no fingerprint).

**3. Value-cache hook.** Before executing an instruction, the interpreter consults an optional caller-supplied hook with `(ssa.Value, current operand values)`; if the hook returns a result, the interpreter installs it as the instruction's value, adds the hook-reported step count to the counters, and skips execution. tumble's side of the hook keys on a structural hash of the value's backward slice (which tumble computes and supplies in a side table keyed by `ssa.Value`) and the hash of the operand values. This is Hashlife applied to the population: identical sub-DAGs on identical inputs are computed once across all individuals and all generations. For correctness the hook must be invoked only for instructions whose value depends on nothing but their operands and the function parameters — pure instructions. Loads, stores, calls into intercepts with side effects, channel ops, and anything touching shadow memory are not cacheable and the hook must not be consulted for them. giri decides purity (it knows which instructions it treats as effectful); tumble never asks for a cache on an impure instruction.

**4. Cost counters by instruction class.** Steps are already counted for `max_steps`. tumble needs the count broken down by class — arithmetic, comparison, load, store, index/slice, call (intercepted), call (interpreted), branch, phi, alloc — so cost weights can later be calibrated against native `ns/op`. Counts returned per run, not printed.

**5. Concreteness query.** A function `Intercepts.IsConcrete(fn *types.Func) bool` (or equivalent) that says whether a given stdlib function is modelled concretely (`math/bits.OnesCount64`, `sort.Ints`) or opaquely (returns a placeholder, as the crypto key-generation intercepts do). tumble's grammar may only admit concrete calls; an opaque call yields a degenerate fingerprint and would poison the index. Expose the set, don't make tumble reverse-engineer it from the intercept tables.

**6. Fast path for single-goroutine, verified-once runs.** For kernels, `max_goroutines == 1` and no scheduling decisions exist; the scheduler should be bypassed. Separately, tumble will evaluate the same genome more than once (once on discovery with detectors on; again whenever it is re-scored or used as a substitution donor). A run option to disable shadow memory and detectors for a function that has already been verified clean would make the second and later evaluations materially cheaper. Any genome that reaches tumble's native stage is re-run with everything on. Measure the speedup before and after; if it is under 2× it isn't worth keeping the option.

**7. Batch build.** A loader that takes a directory containing one package with N functions and returns the `*ssa.Package` with all functions built, in one call, with `ssa.SanityCheckFunctions` off and `ssa.BuildSerially` off. giri's `internal/ssautil` is close to this already; it needs to be reachable from outside and to accept a directory rather than a pattern.

**8. Determinism.** Given the same SSA, the same inputs, the same options, and `max_goroutines == 1`, the outputs, traces, and counters are identical across runs and across machines. Float results follow IEEE semantics for the host; tumble's float fingerprints will bucket, but the interpreter must not introduce nondeterminism of its own (map iteration order in intercepts, `time`, `rand` without a seed option).

**9. Budget behaviour.** When `max_steps` is exhausted, return what was recorded so far with an `Exhausted` flag rather than an error; tumble discards the genome but wants the partial counters for its operator-yield readings.

**10. Stability.** Everything above is behind a documented Go API with a version. giri's CLI behaviour, output formats, exit codes, and detector semantics are unchanged. The API is additive.

Performance target to report back (not a gate): nanoseconds per interpreted instruction on a straight-line integer loop, with shadow memory on and off, on arm64 and amd64.

---

## Prompt for giri's Claude Code

Paste the following into Claude Code in the giri repository.

---

A sibling project, `tumble` (`github.com/scttfrdmn/tumble`), is a strict genetic algorithm over Go kernels. It needs giri as its fitness evaluator: it will build one package of ~256 candidate functions per generation and interpret each on ~64 input vectors, hundreds of thousands of times per run. giri's step counter becomes the performance fitness; its detectors reject candidates that pass tests by accident; its per-value execution traces become semantic fingerprints that drive tumble's crossover. The requirements are in `GIRI_REQUIREMENTS.md` (copy it into `docs/` here). All of them are additive: the CLI, output formats, exit codes, and detector semantics do not change.

Implement, in this order, with tests, committing after each:

1. **`pkg/exec`** — library entry point. `exec.Run(fn *ssa.Function, args [][]any, opts exec.Options) (exec.Result, error)`. Marshal Go values of integer, bool, float, string, and slice-of-those types into interpreter values and back (`pkg/interpreter` already has the value representation; reuse it). Return `Result{Outputs [][]any; Steps uint64; StepsByClass map[Class]uint64; Violations []...; Exhausted bool; Traces ...}`. Unsupported argument types return a typed error. Tests: call a handful of small functions with each supported type and assert round-trip equality against native execution of the same functions.

2. **Batch loader** — `exec.LoadDir(dir string) (*ssa.Package, error)` building all functions in one call; expose what `internal/ssautil` already does, taking a directory. Test: a package with 300 trivial functions loads once and every function is callable.

3. **Step counters by class** — extend the existing `max_steps` counting with a per-class breakdown (arithmetic, compare, load, store, index/slice, call-intercepted, call-interpreted, branch, phi, alloc, other). Returned in `Result`; never printed by the CLI unless `-v`. Test: a function with known instruction mix produces the expected counts.

4. **Per-value traces** — `Options.Trace bool`, `Options.TraceCap int`. When on, record for every value-producing instruction the sequence of concrete values per argument set, capped, with a truncated flag. Keyed by `ssa.Value` in the result. Memory is the risk: store compactly (per-type slices, not `[]any`). Test: a loop with a known accumulator produces the known sequence; the cap truncates and flags.

5. **Value-cache hook** — `Options.Cache exec.CacheHook` with `Lookup(v ssa.Value, operands []Value) (result Value, steps uint64, ok bool)` and `Store(v ssa.Value, operands []Value, result Value, steps uint64)`. Consulted only for instructions the interpreter classifies as pure (no memory access, no effectful intercept, no channel/goroutine op); add that classification as an explicit function with its own test. Test: run a function twice with a recording hook; the second run executes zero pure instructions and produces identical outputs and counters.

6. **Concreteness query** — `exec.IsConcrete(fn *types.Func) bool` backed by the intercept tables. Test: `math/bits.OnesCount64` is concrete; a crypto key-generation intercept is not.

7. **Fast path options** — `Options.SingleGoroutine` bypasses the scheduler when `max_goroutines == 1`; `Options.SkipDetectors` disables shadow memory and detectors for a function the caller asserts has already been verified. Benchmark both on a straight-line integer loop and record ns/instruction with and without, on this machine, in `docs/perf.md`. If `SkipDetectors` gives under 2× it stays implemented but documented as not worth using.

8. **Determinism and budget behaviour** — audit intercepts reachable from kernel code for nondeterminism (map iteration, time, rand) and add `Options.Seed`; make `max_steps` exhaustion return partial results with `Exhausted = true` rather than an error. Test: two runs of the same function with the same seed produce byte-identical `Result`.

9. **API doc** — `docs/exec-api.md` describing the surface, its stability promise (additive, versioned alongside giri), and the purity classification. Update `README.md` with one paragraph and a link.

Requirements 1–10 are tracked as issues on this repo (tumble req 1 … tumble req 10); close each in the commit that lands it.

Decide names, internal structure, and test shape yourself; record anything that departs from `GIRI_REQUIREMENTS.md` as a comment on the giri issue for that requirement, with the reason. Do not change CLI behaviour. End the session with a comment on scttfrdmn/tumble#14 stating what exists, what is tested, the perf numbers measured, and what is blocked.
