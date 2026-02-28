# Giri: Catching Undefined Behavior in Go Programs

*Go is memory-safe — until it isn't. This post introduces Giri, a dynamic
undefined behavior detector that finds the bugs your tests miss.*

---

## The problem with "memory-safe" languages

When you write Go code, you get a lot of safety guarantees for free: no buffer
overflows from plain slice indexing, no use-after-free from the garbage
collector, no null-pointer arithmetic. The language specification and runtime
collaborate to make an entire class of C/C++ bugs simply impossible in
idiomatic Go code.

But "memory-safe" doesn't mean "bug-free." Go programs can still exhibit
undefined behavior — behavior that is technically permitted by the runtime but
produces results the programmer didn't intend, or that violates invariants in
ways that are silent until they explode in production. The three most
dangerous categories are:

**Data races.** When two goroutines access the same memory concurrently and
at least one is writing, the Go memory model makes no guarantees about what
either goroutine observes. `go test -race` can detect many races, but only
if the racing interleaving actually occurs during the test run — which is
a matter of luck with the default scheduler.

**Unsafe pointer misuse.** The `unsafe.Pointer` type lets you escape Go's type
system entirely. The rules governing safe use of `unsafe.Pointer` (documented
in the `unsafe` package) are subtle and easy to violate. The compiler can
silently move objects in memory; code that violates the rules may work today
and break after a compiler upgrade.

**Arena misuse.** `github.com/golang/x/exp/arenas` (and similar region
allocators) give Go programs explicit lifetime control for performance. But
an arena freed too early leaves dangling pointers that the GC won't catch —
the backing memory is marked free but the addresses are still in use.

Go's built-in tools help: `go test -race` finds some races, `go vet` catches
some unsafe patterns, and `staticcheck` catches more. But none of them
*execute* your program and observe its memory behavior. That's where Giri
comes in.

---

## Giri in 60 seconds

Giri is an open-source static/dynamic analysis tool for Go. Instead of
compiling and running your program normally, Giri *interprets* its SSA
(Static Single Assignment) intermediate representation — the same IR that
the Go compiler uses internally. While interpreting, it maintains a **shadow
memory** alongside the real heap: every allocation is tracked, every pointer
is stamped with provenance metadata, and every memory access is validated
against the shadow.

The result: Giri finds bugs that are deterministically present in your code
but that conventional testing misses because the triggering interleaving
never happened.

### Quick demo

Here's a simple program with two goroutines that race on a shared variable:

```go
package main

import "sync"

func main() {
    var x int
    var wg sync.WaitGroup
    wg.Add(2)
    go func() {
        defer wg.Done()
        x = 1 // write
    }()
    go func() {
        defer wg.Done()
        _ = x // read
    }()
    wg.Wait()
}
```

`go test -race` may or may not catch this — it depends on whether the two
goroutines actually overlap during the test. Giri always catches it because
it controls the scheduler:

```
$ giri ./...

VIOLATION: data race
  goroutine 2 wrote x at main.go:9
  goroutine 3 read x at main.go:13
  These accesses are not ordered by any happens-before relationship.

Exit code: 1
```

### Installing Giri

```bash
go install github.com/scttfrdmn/giri/cmd/giri@latest
```

Run it on any Go package:

```bash
giri ./...               # check all packages
giri -race ./...         # race detection only
giri -unsafe ./...       # unsafe.Pointer rules only
giri -format json ./...  # machine-readable output
giri -format sarif ./... # GitHub Code Scanning compatible
```

---

## What Giri detects

Giri ships with four composable detectors:

| Detector | What it finds |
|---|---|
| **Race** | Data races via vector-clock happens-before tracking |
| **Bounds** | Out-of-bounds pointer arithmetic and slice accesses |
| **Unsafe** | All six `unsafe.Pointer` conversion rules from the Go spec |
| **Arena** | Use-after-free, double-free, and escape from arena lifetimes |

Beyond these, Giri detects a range of runtime panics-in-disguise:

- Nil pointer dereference through interface values
- Send on closed channel
- Division by zero
- Negative `sync.WaitGroup` counter
- Deadlock (all goroutines blocked)
- Goroutine leaks (blocked with no sender)
- Double-close on a channel
- Write to nil map

---

## Suppressing false positives

Sometimes you know a piece of code is safe even if it looks dangerous to a
static analysis tool. Add a `//giri:ignore` directive on the line before the
suspicious operation:

```go
//giri:ignore
_ = *(*uint32)(unsafe.Pointer(&b[0]))
```

Giri will suppress violations on the annotated line and the following line.

---

## What's next

This post introduced Giri and showed what it detects. The next post in this
series digs into how it works: the SSA interpreter, shadow memory, and the
vector-clock race detector. The third post covers how to extend Giri with
custom intercepts for your own libraries.

- **Part 2:** [How Giri Works: SSA Interpretation and Shadow Memory](02-architecture.md)
- **Part 3:** [Extending Giri with Custom Intercepts](03-custom-intercepts.md)

---

*Giri is open source under the MIT license.
Source: [github.com/scttfrdmn/giri](https://github.com/scttfrdmn/giri)*
