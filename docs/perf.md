# giri performance

Numbers for v0.100.0, the result of the performance pass in milestone
v0.100.0 (#270–#273), against v0.99.1. Benchmarks live in
`pkg/exec/fastpath_test.go`, `pkg/exec/bench_test.go` and
`pkg/interpreter/cli_bench_test.go`:

```bash
GOEXPERIMENT=arenas go test ./pkg/exec/ -run '^$' -bench 'KernelBatch|IntLoop|SliceLoop|LoadDir|Loader' -benchmem
GOEXPERIMENT=arenas go test ./pkg/interpreter/ -run '^$' -bench CLISynth -benchtime 3x -benchmem
```

`ns/instr` is wall time divided by `Result.Steps`, the instructions the call
executed. Each iteration of a `pkg/exec` benchmark is one `exec.Run`, which
includes session setup.

## Method

v0.99.1 predates the benchmark files, so they were copied into a v0.99.1 tree
and both trees were built into test binaries. The binaries were then run
alternately, three rounds each, and the tables give medians. Per-op allocations
were identical across rounds on each machine.

- **arm64:** Apple M4 Pro (12 cores), go1.27.3 darwin/arm64. This is a shared
  workstation; load average was 6–10 during the run. The prototype work found
  that load above ~20 inflates ns/instr by 1.5–2.5×, so check `uptime` before
  comparing numbers.
- **amd64:** Intel i9-9960X @ 3.10 GHz (32 threads), Rocky Linux 9, go1.27.0
  linux/amd64. The run went through janus.local's `measured` pueue queue, one
  job at a time, with load average about 0.2.

## tumble-shaped workloads (`BenchmarkKernelBatch`)

A small kernel on 64 argument sets per `Run`, the shape of tumble's
evaluations, plus a single-set `Run`. Per-`Run` and per-call costs dominate
these.

| Benchmark | arm64 v0.99.1 | arm64 v0.100.0 | amd64 v0.99.1 | amd64 v0.100.0 |
|---|---:|---:|---:|---:|
| MixU64 ×64 (7 instr/call) | 206 µs | **64.1 µs** (3.2×) | 402 µs | **137 µs** (2.9×) |
| SumU32 ×64 (16-element slice) | 3.62 ms | **0.512 ms** (7.1×) | 6.63 ms | **1.08 ms** (6.2×) |
| MixU64 ×1 | 16.2 µs | **6.38 µs** (2.5×) | 23.6 µs | **13.0 µs** (1.8×) |
| SumU32 ×64, `SkipDetectors` | 0.909 ms | 0.465 ms (2.0×) | 1.60 ms | 0.952 ms (1.7×) |

Allocations for SumU32 ×64 drop from 5.68 MB/op to 0.43 MB/op.

## Instruction loops

| Benchmark | arm64 v0.99.1 | arm64 v0.100.0 | amd64 v0.99.1 | amd64 v0.100.0 |
|---|---:|---:|---:|---:|
| `IntLoop` (straight-line integer loop) | 61.5 ns/instr | **36.8** (1.7×) | 119.5 | **78.6** (1.5×) |
| `SliceLoop`, detectors on (load + store per iteration) | 330 ns/instr | **53.6** (6.2×) | 634 | **120** (5.3×) |
| `SliceLoop`, `SkipDetectors` | 77.2 ns/instr | 47.8 (1.6×) | 141 | 103 (1.4×) |

`SingleGoroutine` makes no measurable difference on any benchmark. The
scheduler is only consulted on `go` statements and goroutine exit. The option
is kept because it rejects `go` statements, which kernels shouldn't contain.

**`SkipDetectors` now matters little for single-goroutine kernels.** Detectors
on cost 53.6 vs 47.8 ns/instr on `SliceLoop` (arm64), because the race
detector stays dormant until a second goroutine exists. Before this pass the
gap was 4.3×.

## CLI (`BenchmarkCLISynth`)

A whole-program CLI run with the default detectors on a mixed workload: four
goroutines doing sort, map, struct, append and `strings.Builder` work, joined
with a `WaitGroup` and a `Mutex`.

| | arm64 v0.99.1 | arm64 v0.100.0 | amd64 v0.99.1 | amd64 v0.100.0 |
|---|---:|---:|---:|---:|
| time per run | 258 ms | **163 ms** (1.6×) | 465 ms | **325 ms** (1.4×) |
| allocated | 337 MB | 170 MB | 342 MB | 174 MB |

Findings are unchanged: 0 violations both before and after. The CLI findings
diff over all integration and showcase programs showed no changes for any PR
in the pass.

## Package loading (`BenchmarkLoadDir`, `BenchmarkLoader`)

A generated package of 300 trivial functions.

| | arm64 `LoadDir` | arm64 `Loader.Load` | amd64 `LoadDir` | amd64 `Loader.Load` |
|---|---:|---:|---:|---:|
| no imports | 27.2 ms | **1.34 ms** | 14.2 ms | **2.73 ms** |
| 7 stdlib imports | 153 ms | **6.45 ms** | 241 ms | **9.59 ms** |

`LoadDir` runs `go list` and type-checks every dependency from source on each
call. `Loader` (#273) type-checks dependencies once per `Loader`; the figures
are steady-state, after one warm-up load.

## What changed

| PR | Change | Mainly helps |
|---|---|---|
| #270 | Session calls no longer merge every earlier call's clock (quadratic in argument sets); lazy `math/rand` seeding; memoized function names | many-set `Run`s, single-set setup |
| #271 | Slot-indexed locals in sessions (no `map[string]Value` per frame); goroutine lookup cache | every `pkg/exec` instruction |
| #272 | Race detector dormant until a second goroutine exists; one shared clock snapshot per access instead of a copy per byte | loads and stores, the CLI |
| #273 | `exec.Loader`: in-memory loading with shared dependency types | per-generation package builds |

## Where the time goes now

Profiled on `IntLoop` after the pass, these are the next opportunities, each
filed:

- **Per-instruction map lookups:** the value→slot map, the site memo and
  `classify`. Precompiling functions into instruction tables (#274) is
  estimated, not measured, at a further 25–35%.
- **Boxing integers in `Value.Raw interface{}`:** `runtime.convT64` plus the GC
  work it causes (#275). Unboxing touches ~3000 construction sites shared with
  the CLI, so it's high risk.
- **Pointer allocation on every `IndexAddr`** in value-faithful memory, and the
  `elems` map growing with every store (#276).
- **Per-byte race entries** (#277) and **`posString` on the CLI path** (#278).
