# Giri Showcase: Bugs the Go Toolchain Misses

Each program in this directory demonstrates a class of bug that:

- compiles without errors
- passes `go vet`
- passes `go test -race`

…but is caught by Giri.

## Comparison Table

| Program | `go vet` | `go test -race` | `giri` |
|---|---|---|---|
| [`unsafe_oob`](#unsafe_oob) | ✅ pass | ✅ pass | ❌ unsafe-pointer OOB |
| [`unsafe_alignment`](#unsafe_alignment) | ✅ pass | ✅ pass | ❌ unsafe-pointer rule 1 |
| [`uintptr_gc_hazard`](#uintptr_gc_hazard) | ✅ pass | ✅ pass | ❌ unsafe-pointer rule 2 |
| [`uninit_read`](#uninit_read) | ✅ pass | ✅ pass | ❌ uninitialized read\* |
| [`nil_deref`](#nil_deref) | ✅ pass | ✅ pass† | ❌ nil pointer dereference |
| [`type_assert`](#type_assert) | ✅ pass | ✅ pass | ❌ type-assertion failure |
| [`reflect_unsafe`](#reflect_unsafe) | ✅ pass | ✅ pass | ❌ unsafe-pointer rule 5 |
| [`goroutine_leak`](#goroutine_leak) | ✅ pass | ✅ pass | ❌ goroutine leak |
| [`deadlock`](#deadlock) | ✅ pass | ✅ pass | ❌ global deadlock |
| [`wg_negative`](#wg_negative) | ✅ pass | ✅ pass | ❌ waitgroup negative counter |
| [`pct_race`](#pct_race) | ✅ pass | ✅ pass | ❌ waitgroup (PCT only)\* |

\*\* Requires `--runs N` flag (multi-run PCT scheduling).

\* Requires `--track-init` flag.
† Passes if the failing code path is not covered by tests.

---

## unsafe_oob

**File:** `unsafe_oob/main.go`

A frame parser reads a protocol header using `unsafe.Add`, but the offset
calculation is wrong — it reads one byte past the end of the 4-byte allocation.

```go
func parseFrameType(hdr *[4]byte) byte {
    return *(*byte)(unsafe.Add(unsafe.Pointer(hdr), 4)) // offset 4 > len 4
}
```

**Why `-race` misses it:** single goroutine, no concurrent access.
**Why it matters:** silently reads adjacent stack or heap memory. No panic, no
error — just wrong data that may surface as a subtle protocol bug in production.

```
$ giri ./testdata/showcase/unsafe_oob
VIOLATION unsafe-pointer-violation: pointer arithmetic exceeds allocation bounds
  rule:  Rule 3 — arithmetic must stay within the allocation
  site:  unsafe_oob/main.go:30
```

---

## unsafe_alignment

**File:** `unsafe_alignment/main.go`

A binary decoder reads a `uint32` from a byte slice at a non-4-byte-aligned
offset, violating `unsafe.Pointer` Rule 1.

```go
func readU32LE(b []byte, offset int) uint32 {
    return *(*uint32)(unsafe.Pointer(&b[offset])) // offset 1 mod 4 != 0
}
```

**Why `-race` misses it:** single goroutine, no concurrent access.
**Why it matters:**
- **ARM / RISC-V / MIPS:** `SIGBUS` crash at runtime (bus error).
- **x86:** may silently return torn or rotated data on some microarchitectures.
- Violates the Go specification regardless of hardware behaviour.

```
$ giri ./testdata/showcase/unsafe_alignment
VIOLATION unsafe-pointer-violation: invalid pointer conversion
  rule:  Rule 1 — pointer must be aligned to target type's requirements
  site:  unsafe_alignment/main.go:28
```

---

## uintptr_gc_hazard

**File:** `uintptr_gc_hazard/main.go`

A pointer is stored as a `uintptr` integer, a function is called (creating a
GC safepoint), and then the integer is converted back to a pointer.

```go
cache = uintptr(unsafe.Pointer(x)) // save pointer as integer
doWork()                            // GC safepoint — x may be collected
_ = *(*int)(unsafe.Pointer(cache)) // stale pointer: undefined behaviour
```

**Why `-race` misses it:** single goroutine, no concurrent access.
**Why it matters:** The current Go GC does not move objects, so this code
appears to work today. But `unsafe.Pointer` Rule 2 explicitly forbids this
pattern, and it **will silently break** with any future moving or compacting GC.
This is a time-bomb in the codebase.

```
$ giri ./testdata/showcase/uintptr_gc_hazard
VIOLATION unsafe-pointer-violation: uintptr held across GC point
  rule:  Rule 2 — uintptr must not be held across a potential GC safepoint
  site:  uintptr_gc_hazard/main.go:42
```

---

## uninit_read

**File:** `uninit_read/main.go`

A struct is allocated with `new()` and a field is read before any explicit
write. Go's spec guarantees zero-initialization, so this is not a language bug.
With `--track-init`, Giri flags reads on bytes the program never explicitly
wrote — useful for auditing security-sensitive code.

```go
func isNullToken(t *AuthToken) bool {
    return t.value[0] == 0 // read before any explicit write
}
```

**Use cases:**
- Verifying that sensitive buffers are explicitly zeroed before use.
- Catching logic bugs where initialization was accidentally skipped.
- Auditing code that must not rely on allocator zero-fill behaviour.

```
$ giri --track-init ./testdata/showcase/uninit_read
VIOLATION uninitialized read: byte at offset 0 was never written
  alloc: heap (AuthToken), size 32
  site:  uninit_read/main.go:41
```

---

## nil_deref

**File:** `nil_deref/main.go`

A map lookup returns `nil` for an absent key; the caller dereferences without
a nil check. Tests that only exercise the "found" path will not catch this.

```go
func getPort(scheme string) int {
    return *defaultPorts[scheme] // nil dereference if scheme absent
}

_ = getPort("http")  // fine
_ = getPort("ftp")   // nil dereference — but maybe no test covers this
```

**Why `-race` misses it:** no concurrent access; and if the `"ftp"` code path
is not exercised during the test run, `-race` never sees it.
**Why Giri catches it:** Giri traces all reachable SSA paths, including
`getPort("ftp")`, and reports the nil dereference statically without requiring
the failing execution path to be covered by a test.

```
$ giri ./testdata/showcase/nil_deref
VIOLATION nil pointer dereference (goroutine 1) at nil_deref/main.go:37
```

---

---

## type_assert

**File:** `type_assert/main.go`

`makeAnimal("cat")` returns a `*Cat`, but the caller asserts without the comma-ok
form that it holds a `*Dog`. This panics at runtime.

```go
a := makeAnimal("cat")   // returns *Cat
d := a.(*Dog)            // panics: interface holds *Cat, not *Dog
_ = d.Sound()
```

**Why `go vet` misses it:** `go vet` does not track the return type of
`makeAnimal` through branches — it only flags assertions where the concrete
type is provably wrong from the static type.
**Why `-race` misses it:** single goroutine, no concurrent access.
**Why Giri catches it:** Giri interprets `makeAnimal("cat")` along the `"cat"`
branch, sees the `MakeInterface` wrapping a `*Cat`, then detects at the
`TypeAssert(*Dog)` that the dynamic type does not match.

```
$ giri ./testdata/showcase/type_assert
VIOLATION type-assertion failed: interface holds *main.Cat, not *main.Dog (goroutine 1) at ...
```

---

## reflect_unsafe

**File:** `reflect_unsafe/main.go`

`processValue()` calls `reflect.Value.Pointer()` (which returns a `uintptr`),
then calls `doWork()` (a GC safepoint), then converts the uintptr back to a pointer.

```go
func processValue(v reflect.Value) *int {
    uptr := v.Pointer() // Rule 5: returns uintptr, not a tracked pointer
    doWork()            // GC safepoint — uptr may now be stale!
    return (*int)(unsafe.Pointer(uptr))
}
```

**Why `go vet` misses it:** the types are correct — `uintptr` is a valid
intermediate form for reflect-obtained pointers. `go vet` does not track
liveness of uintptr values across function boundaries.
**Why `-race` misses it:** single goroutine, no concurrent access.
**Why Giri catches it:** Giri intercepts `reflect.Value.Pointer()`, records
the resulting uintptr as a pending Rule 5 conversion, and fires a violation
when `doWork()` constitutes a GC safepoint before the uintptr is converted back.

```
$ giri ./testdata/showcase/reflect_unsafe
VIOLATION unsafe-pointer-violation (rule 5: reflect pointer conversion) at ...
  uintptr value t4 (converted at ...) survived to GC point without conversion back to unsafe.Pointer
  hint: Review the six rules at https://pkg.go.dev/unsafe#Pointer.
```

---

## goroutine_leak

**File:** `goroutine_leak/main.go`

A `worker` goroutine reads from a `results` channel that `main` never writes to.
This is a common pattern in production code when the caller forgets to send a
result after an early return or error.

```go
func worker(results chan int) {
    val := <-results // blocks forever — main exits without sending
    _ = val
}

func main() {
    results := make(chan int)
    go worker(results)
    // BUG: main returns without ever sending on results
}
```

**Why `go vet` misses it:** the channel operations are type-correct.
**Why `-race` misses it:** there is no concurrent *data* access — just a
permanently blocked goroutine.
**Why Giri catches it:** Giri tracks which channels have ever had a sender.
At finalization, any goroutine blocked on a channel with no recorded sender
is reported as a goroutine leak.

```
$ giri ./testdata/showcase/goroutine_leak
VIOLATION goroutine-leak: goroutine 2 blocked on channel receive
  spawned at: goroutine_leak/main.go:22
  blocked at: goroutine_leak/main.go:14
  hint: Ensure every goroutine that reads from a channel has a corresponding
        sender, or use select with a default clause to avoid permanent blocking.
```

---

## deadlock

**File:** `deadlock/main.go`

Both `main` and a spawned goroutine wait forever on the same channel.
Nobody ever sends — all goroutines are simultaneously blocked.
Go's runtime would print "all goroutines are asleep — deadlock!" and abort.

```go
func recv(ch chan int) { <-ch }

func main() {
    ch := make(chan int)
    go recv(ch) // blocks forever
    recv(ch)    // main also blocks — deadlock
}
```

**Why `go vet` misses it:** channel operations are type-correct.
**Why `-race` misses it:** no concurrent *data* access.
**Why Giri catches it:** at finalization, every goroutine is in `GoroutineBlocked`
state and none has finished. Giri recognizes the all-blocked condition as a global
deadlock, distinct from a goroutine leak (where main exits normally).

```
$ giri ./testdata/showcase/deadlock
VIOLATION deadlock: all goroutines are asleep — 2 goroutine(s) blocked
  hint: Check for circular channel dependencies or missing sends.
```

---

## wg_negative

**File:** `wg_negative/main.go`

A worker pool where the caller's cleanup code calls `Done()` one extra time.
When the goroutine finishes and also calls `Done()`, the counter goes below zero
and panics: `"sync: negative WaitGroup counter"`.

```go
func process(wg *sync.WaitGroup, id int) {
    defer wg.Done()   // one Done for the goroutine
    _ = id * 2
}

func main() {
    var wg sync.WaitGroup
    wg.Add(1)
    go process(&wg, 42)
    wg.Done()         // BUG: extra Done — total Dones = 2 for one Add
    wg.Wait()
}
```

**Why `go vet` misses it:** `Done()` is a valid method call.
**Why `-race` misses it:** no concurrent unsynchronized data access.
**Why Giri catches it:** Giri tracks the WaitGroup counter through every `Add`,
`Done` call and reports when the counter goes negative.

```
$ giri ./testdata/showcase/wg_negative
VIOLATION waitgroup: negative WaitGroup counter (-1) (goroutine 2) at ...
  hint: Each Done() call must be matched by an Add(1).
```

---

## pct_race

**File:** `pct_race/main.go`

A WaitGroup misuse that is invisible under the default round-robin schedule
but discovered by PCT multi-run scheduling.

`work()` (GID 2) calls `Done()` and `setup()` (GID 3) calls `Add(1)`.
Round-robin always runs the higher-GID goroutine first: `setup()` runs before
`work()`, the counter goes 0→1→0 — no violation. PCT randomizes priorities,
so in some runs `work()` executes before `setup()`: the counter goes 0→-1 —
WaitGroup negative counter violation.

```go
var wg sync.WaitGroup
func setup() { wg.Add(1) }      // GID 3 — runs first with round-robin
func work()  { wg.Done() }      // GID 2 — runs second with round-robin
                                 //         BUT sometimes first with PCT → violation
```

**Why single-run Giri misses it:** round-robin always picks `setup()` first.
**Why `RunN` finds it:** PCT tries different orderings; in the `work()`-first
ordering, `Done()` decrements from 0 to -1.

```
$ giri --runs 20 ./testdata/showcase/pct_race
VIOLATION waitgroup: negative WaitGroup counter (-1) (goroutine 2) at ...
  hint: Each Done() call must be matched by an Add(1).
```

---

## Running the Showcase Tests

The integration test suite validates all showcase programs automatically:

```bash
go test -run TestShowcase ./pkg/interpreter/
```

To run Giri directly on a showcase program:

```bash
go run ./cmd/giri ./testdata/showcase/unsafe_oob
```
