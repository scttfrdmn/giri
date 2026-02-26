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

## Running the Showcase Tests

The integration test suite validates all showcase programs automatically:

```bash
go test -run TestShowcase ./pkg/interpreter/
```

To run Giri directly on a showcase program:

```bash
go run ./cmd/giri ./testdata/showcase/unsafe_oob
```
