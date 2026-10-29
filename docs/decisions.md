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
