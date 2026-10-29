# pkg/exec: departures from GIRI_REQUIREMENTS.md

Decisions taken while implementing [`GIRI_REQUIREMENTS.md`](GIRI_REQUIREMENTS.md)
(tumble's requirements for giri), with the reason for each. The API they
describe is documented in [`exec-api.md`](exec-api.md).

## Req 1 — library entry point

**A concrete (value-faithful) interpreter mode was needed, not just marshalling.**
The requirements assumed `pkg/interpreter`'s value representation could be
reused as-is. The representation is reused, but its semantics are approximate
by design (giri is a UB detector):

- integer ops evaluate in plain `int64` with no width wrap and signed semantics for unsigned types;
- memory remembers only the value at offset 0 of an allocation;
- `append`/`copy` carry no element values;
- `uint64` constants above `MaxInt64` are undefined.

A kernel over a slice could not match native execution. `NewSession` turns on
concrete mode (`pkg/interpreter/concrete.go`): typed integer and float
semantics, a per-(allocation, offset) element store behind every load and
store, element-carrying `append`/`copy`/`clear`/string slicing/conversions,
exact constants, and integer divide-by-zero as a halting panic. CLI entry
points (`Run`, `RunN`, `RunTests`) never set it, so CLI output, exit codes and
detector semantics are unchanged; the full integration suite is the regression
check. This also delivers tumble req 11 (#248), scoped to `pkg/exec` only. The
CLI path is not made value-faithful, because that would change which branches
existing programs take, and so which findings they report. (Discussed on #238
and #248.)

**Result is per-argument-set aware.** Beyond the fields in the requirements,
`Result` has `Outcomes []Outcome` (`Returned`, `Panicked`, `Exhausted`,
`Opaque`) and `StepsPerSet`. A plain `panic`, or a run-time panic giri raises,
makes that set's output meaningless, and tumble needs to tell that apart from
a returned value. `Opaque` marks a set that returned a value no concrete model
produced.

**Argument kinds must match exactly.** An `int` argument for an `int64`
parameter is an `*ArgError`, not a conversion. Silently converting would hide
generator bugs on tumble's side. Named basic types are accepted and returned
by their underlying Go type (`type Celsius float64` comes back as `float64`),
because pkg/exec cannot construct the caller's named type.

**Floating point is never fused.** giri rounds after every operation. Go on
arm64 fuses `x*y + z` into FMA unless an explicit conversion prevents it,
which the spec permits, so native arm64 results can differ in the last bit
from giri's. giri matches amd64 (no fusion) on every host, which is what
requirement 8 asks for. The round-trip tests use explicit conversions where
native code would otherwise fuse.

**Float→integer conversion out of range saturates.** Go leaves it
implementation-defined, and amd64 and arm64 disagree. giri saturates and maps
NaN to 0, so results are identical on every host.

**`cap` after `append` growth follows the runtime's growth formula without
allocator size-class rounding**, so `cap()` of a grown slice can be smaller
than native. `len` and contents are exact.

**Not modelled concretely in v0:** whole-value loads and stores of arrays and
structs (element and field access works), maps as arguments or results,
interface method calls into the standard library, and calls into non-stdlib
dependencies of the loaded package (their bodies are not built).

## Req 4 — cost counters by instruction class

**One class beyond the requested list: `builtin`.** Calls to builtins (`len`,
`append`, `copy`, `min`, …) are neither intercepted stdlib calls nor
interpreted calls, and their native cost differs from both by orders of
magnitude, so lumping them into either call class would skew calibration.
(`other` was already in the list; it collects conversions, `Extract`,
`Return`, `Panic` and the remaining bookkeeping instructions.) The set is
append-only.

**Callee instructions count toward the caller's run.** `Steps` is every
instruction executed during a call, including interpreted callees, which is
what a cost model compares against native `ns/op`.

**The CLI prints nothing new, even with `-v`.** Class counters exist only in
library sessions; the CLI never creates one.

## Req 2 — per-value traces

**Only the traced function's own instructions are recorded**, not those of
interpreted callees; recursive activations of the function append to the
same sequence. Traces are keyed by `ssa.Value`, and a callee's values belong
to a different function.

**Non-scalar values record only an execution count** (`TraceOther`).
Pointers, slices and tuples have no machine-independent concrete value to
fingerprint; their element loads are traced as scalars.

**A value that cannot be represented stops its trace with `Opaque` set**,
alongside the requested `Truncated` flag. Both mean "no fingerprint".

## Req 3 — value-cache hook

**Granularity is one instruction.** The interpreter executes instructions in
order, so by the time it reaches a pure instruction its operands have
already been computed (by execution or by earlier cache hits). A hit
therefore saves that one instruction, not its backward slice. The saving
is real only where the instruction is expensive to interpret: concrete
pure intercepts such as `math.Sqrt` or `bits.OnesCount64`, and arithmetic
whose interpretation costs more than a lookup. Skipping whole sub-DAGs
would need demand-driven (lazy) evaluation of operands. That was not
built; it would change the evaluation order the step counters measure.

**Phis are not pure.** A phi's value depends on which predecessor block ran,
not only on its operands.

**Interpreted calls are not pure in v0**, even when the callee only does
arithmetic. Proving that needs a transitive purity analysis of the callee.

**Pure ⊆ scalar.** Operands and results must be booleans, numbers or strings,
the values tumble can hash. Conversions into slices, loads and every
memory-touching instruction are excluded.

**Panicking executions are never stored.** Integer division and shifts are
pure but can panic. The interpreter stores an entry only for an execution
that recorded no violation and did not halt, so a cache hit can never mask a
detector finding.

`Result.CacheHits` and `Result.PureExecuted` are additions, so a caller can
verify cache effectiveness.

## Req 5 — concreteness query

**The concrete set is a new, explicit table, not a reading of the existing
intercepts.** giri's CLI intercepts (`stdlib.go`) can't be trusted as
"concrete" without auditing each one. Some are no-ops (`sort.Ints` does not
sort), and some are wrong for narrow widths (`bits.LeadingZeros32`,
`RotateLeft32`, `Reverse32`, `TrailingZeros8(0)` all call the 64-bit
function). `pkg/interpreter/concrete_intercepts.go` lists every concretely
modelled function, and each entry wraps the real stdlib function, so a
listed function is correct by construction. In concrete mode the table
takes precedence over the CLI intercepts; under the CLI nothing changes.
v0 covers `math`, `math/bits`, `strings`, `strconv`, `unicode`,
`unicode/utf8` and `sort` (the scalar-signature functions, plus the typed
`sort` slice helpers). The CLI-side width bugs are tracked separately.

`exec.ConcreteFuncs()` exposes the set, as the requirement asks. `IsConcrete`
is false for methods and for functions of the loaded package, which are
interpreted from source rather than intercepted.

## Req 6 — fast paths

**`SkipDetectors` is kept and recommended for memory-touching kernels.** The
requirement says to measure on a straight-line integer loop and drop the
option below 2×. On that loop it is 1.0×, because the loop performs no
memory operations and there is nothing to skip. On a load/store loop it is
4.3× (`docs/perf.md`). It stays implemented, and the docs say exactly where
it helps.

**`SkipDetectors` keeps allocation identity.** Concrete mode keys element
values by allocation, so allocations still get IDs. Bounds metadata, access
checks, initialization bits, pointer tracking, stack poisoning and all
detectors are skipped. Run-time panics (index out of range, nil dereference,
divide by zero) are still raised, so outputs and outcomes are identical to a
detectors-on run. `TestFastPathsPreserveResults` checks this.

**`SingleGoroutine` gives no speedup.** The scheduler is consulted only on
`go` statements and goroutine exit. It is kept because it rejects `go`
statements, which kernels should not contain.

## Req 7 — batch build

**Dependencies are type-checked from source.** The fast path would read
dependency types from export data. x/tools v0.41's reader cannot read the
export data written by go1.27, so every import came back without types.
`LoadDir` uses `NeedDeps`, like `internal/ssautil`, but builds SSA bodies only
for the target package. Upgrading x/tools would allow the export-data path
and cut load time for packages with many imports.

## Req 8 — determinism

**Audit of interpreter-introduced nondeterminism** (intercepts reachable from
kernel code, and the interpreter itself):

| Source | Status |
|---|---|
| `range` over a map | Go randomizes the order. Concrete mode iterates keys in sorted order (bool, int, float, string, then other by text). The CLI path is unchanged. |
| `math/rand`, `math/rand/v2` | Drawn from the interpreter's generator, seeded from `Options.Seed`. |
| Random/PCT schedulers | Seeded from `Options.Seed`; with `SingleGoroutine` there is no scheduler. |
| `time.Now`, `Since`, `Until`, timers | Intercepts return fixed values; already deterministic. |
| `crypto/rand.Read` | Returns the length without filling the buffer; deterministic. Not concrete. |
| `os.Getpid/Getuid/Getgid`, `os.Getenv`, `syscall.Getenv`, `os.Getwd`, `os.Hostname` | Host-dependent, deterministic within a host. None is concrete, so tumble's grammar never admits them. |
| Interpreter bookkeeping maps (goroutines, channels, mutexes) | Never iterated in a way that reaches a Result: sessions do not run the CLI's end-of-program `Finish` checks, and violations are kept in detection order. |
| Goroutine IDs in violation messages | Per-session counters; deterministic. |

`TestDeterministicResults` serializes every field of `Result`, traces
included, and requires byte-identical output across six runs.

**Across machines, two things legitimately differ.** First, violation
messages contain absolute source paths. Second, results of `math` functions
with architecture-specific assembly (e.g. `math.Exp` on amd64 versus arm64)
follow the host, as requirement 8 allows. Basic float arithmetic is never
fused (see req 1), and `int`/`uint` are 64-bit on every supported host.

**Sessions skip end-of-program checks.** Goroutine leaks, deadlocks, arena
leaks and unreleased context cancels are reported by the CLI's `Finish` once
`main` returns. A session calls one function repeatedly and has no "end of
program", so those checks do not run. Findings raised during a call are all
reported.

## Req 9 — budget behaviour

**`MaxSteps` is one budget for the whole Run**, shared across argument sets
in order, not a per-set limit. That bounds the cost of evaluating one
candidate. When it runs out, the Run stops: `Outputs` and `Outcomes` are
shorter than `args`, the last outcome is `Exhausted`, `Result.Exhausted` is
set, and steps, class counters and traces cover everything executed. No
violation is recorded for exhaustion, unlike the CLI's step-limit error.
`MaxSteps == 0` means `DefaultMaxSteps` (10M, giri's CLI default), not
unlimited, because evolved candidates routinely loop forever.
