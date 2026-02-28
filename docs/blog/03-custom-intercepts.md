# Extending Giri with Custom Intercepts

*Giri ships with intercepts for 60+ stdlib packages. But what about your own
libraries, or third-party dependencies? This post shows how to add intercepts
for any package using the `Config.Intercepts` API introduced in v0.31.0.*

---

## The opaque value problem

When Giri encounters a call to a function it can't execute — either because
the function is compiled native code or because interpreting it would be too
expensive — it returns `Value{}`, an *opaque* zero value. The interpreter then
continues with this value as if the function had returned the zero value for
its result type.

This is usually fine for `bool`-returning functions (the false branch is
taken) and for `error`-returning functions (nil is returned). But it breaks
down in two common patterns:

**Pattern 1: opaque values used as input to later operations.**
If `mylib.Parse(input)` returns an opaque `*mylib.Config`, and your code then
calls `config.GetTimeout()` on the result, the interpreter sees a method call
on a nil-raw value. Depending on how SSA lowers the call, this may produce a
false-positive nil-dereference violation or cause the interpreter to silently
skip the call.

**Pattern 2: functions that produce side effects the interpreter must know
about.**
If `mylib.MakeChannel(cap)` creates a channel with specific capacity
semantics, the interpreter's built-in channel tracking won't know about it.
Subsequent send/receive operations may produce spurious violations or miss
real ones.

Custom intercepts solve both problems.

---

## The Config.Intercepts API

Since v0.31.0, `interpreter.Config` has an `Intercepts` field:

```go
type InterceptFunc func(args []Value) (Value, bool)

type CustomIntercepts map[string]map[string]InterceptFunc

type Config struct {
    // ... existing fields ...

    // Intercepts are checked before built-in stdlib handlers.
    Intercepts CustomIntercepts
}
```

The outer map key is the Go package import path. The inner map key is the
function or method name (without the receiver). The callback receives the
call arguments and returns `(result, true)` when handled, or
`(Value{}, false)` to fall through to built-in handling.

---

## Encoding return values

The `Value` type wraps any Go value:

| Scenario | How to return it |
|---|---|
| No meaningful return (`func f()`) | `Value{}` |
| Single scalar (`func f() int`) | `Value{Raw: int64(42)}` |
| Boolean | `Value{Raw: true}` |
| String | `Value{Raw: "sentinel"}` |
| Opaque non-nil object | `Value{Raw: struct{}{}}` |
| Tuple `(T, error)` | `Value{Raw: []Value{{Raw: v}, {}}}` |
| Tuple `(T, bool)` | `Value{Raw: []Value{{Raw: v}, {Raw: false}}}` |
| Byte slice | `Value{Raw: []byte{0}}` |
| Slice of values | `Value{Raw: []Value{}}` |

The `{}` in the tuple is a zero-value `Value` — it represents a nil error or
the zero value for the second return.

---

## Example 1: modeling a custom allocator

Imagine you're using a high-performance slab allocator:

```go
// github.com/myco/slab
package slab

// New allocates a T from the slab and returns a pointer.
// Must be paired with Free.
func New[T any](s *Slab) *T { ... }

// Free returns a pointer to the slab.
func Free[T any](s *Slab, p *T) { ... }
```

Without an intercept, `slab.New` returns `Value{}` and downstream code
dereferences the result — triggering a spurious nil-dereference violation.

With an intercept:

```go
cfg := interpreter.DefaultConfig()
cfg.Intercepts = interpreter.CustomIntercepts{
    "github.com/myco/slab": {
        // New[T] returns an opaque non-nil pointer.
        "New": func(args []interpreter.Value) (interpreter.Value, bool) {
            return interpreter.Value{Raw: struct{}{}}, true
        },
        // Free is a noop from the interpreter's perspective.
        "Free": func(args []interpreter.Value) (interpreter.Value, bool) {
            return interpreter.Value{}, true
        },
    },
}
```

Now `slab.New` returns a non-nil opaque value, method calls on the result
dispatch correctly to the `slab` case, and `slab.Free` is silently ignored.

---

## Example 2: overriding a stdlib function

Custom intercepts are checked *before* the built-in stdlib intercepts. This
lets you override stdlib behavior for testing purposes:

```go
cfg := interpreter.DefaultConfig()
cfg.Intercepts = interpreter.CustomIntercepts{
    "strings": {
        // Override strings.Contains to always return true for test isolation.
        "Contains": func(args []interpreter.Value) (interpreter.Value, bool) {
            return interpreter.Value{Raw: true}, true
        },
    },
}
```

This is useful when you're testing code paths that depend on specific string
matching behavior.

---

## Example 3: modeling a configuration library

A common pattern in Go microservices: a configuration library that reads from
environment variables or a file at startup:

```go
// github.com/myco/config
func Load(path string) (*Config, error) { ... }
func (c *Config) GetString(key string) string { ... }
func (c *Config) GetInt(key string) int { ... }
```

Intercept:

```go
cfg.Intercepts = interpreter.CustomIntercepts{
    "github.com/myco/config": {
        // Load returns (opaque *Config, nil error).
        "Load": func(args []interpreter.Value) (interpreter.Value, bool) {
            opaque := interpreter.Value{Raw: struct{}{}}
            return interpreter.Value{Raw: []interpreter.Value{opaque, {}}}, true
        },
        // GetString returns a non-empty sentinel so string comparisons
        // take the non-trivial branch.
        "GetString": func(args []interpreter.Value) (interpreter.Value, bool) {
            return interpreter.Value{Raw: "sentinel"}, true
        },
        // GetInt returns a non-zero sentinel.
        "GetInt": func(args []interpreter.Value) (interpreter.Value, bool) {
            return interpreter.Value{Raw: int64(1)}, true
        },
    },
}
```

---

## Example 4: using concrete argument values

Intercepts receive the actual argument values, so you can make the return
value depend on the inputs:

```go
"Clamp": func(args []interpreter.Value) (interpreter.Value, bool) {
    if len(args) < 3 {
        return interpreter.Value{Raw: int64(0)}, true
    }
    // args[0] = value, args[1] = min, args[2] = max
    v, vok := args[0].Raw.(int64)
    lo, lok := args[1].Raw.(int64)
    hi, hok := args[2].Raw.(int64)
    if vok && lok && hok {
        if v < lo {
            return interpreter.Value{Raw: lo}, true
        }
        if v > hi {
            return interpreter.Value{Raw: hi}, true
        }
        return interpreter.Value{Raw: v}, true
    }
    // Non-concrete args: return pessimistic mid-range value.
    return interpreter.Value{Raw: int64(1)}, true
},
```

---

## Wiring intercepts into your analysis harness

If you're using Giri as a library (rather than the CLI), intercepts are just
another Config field:

```go
package myanalysis

import (
    "golang.org/x/tools/go/packages"
    "github.com/scttfrdmn/giri/internal/ssautil"
    "github.com/scttfrdmn/giri/pkg/interpreter"
    "github.com/scttfrdmn/giri/pkg/report"
)

func Analyze(dir string) error {
    cfg := interpreter.DefaultConfig()
    cfg.Intercepts = buildIntercepts()

    prog, err := ssautil.LoadPackages(nil, dir)
    if err != nil {
        return err
    }

    result := interpreter.Run(prog, cfg)
    rpt := report.Build(result.Violations, nil)
    return rpt.Write(os.Stdout, report.FormatText)
}

func buildIntercepts() interpreter.CustomIntercepts {
    return interpreter.CustomIntercepts{
        "github.com/myco/db": {
            "Open": func(args []interpreter.Value) (interpreter.Value, bool) {
                opaque := interpreter.Value{Raw: struct{}{}}
                return interpreter.Value{Raw: []interpreter.Value{opaque, {}}}, true
            },
            "Query": func(args []interpreter.Value) (interpreter.Value, bool) {
                opaque := interpreter.Value{Raw: struct{}{}}
                return interpreter.Value{Raw: []interpreter.Value{opaque, {}}}, true
            },
        },
    }
}
```

---

## Intercept design guidelines

**Return non-nil opaque values for pointer/interface return types.** If your
function returns `*Foo` and you return `Value{}`, the interpreter sees a nil
pointer. Downstream code that calls methods on it will either crash or produce
false positive nil-dereference violations. Use `Value{Raw: struct{}{}}` as a
safe opaque non-nil.

**Return `(val, nil error)` for `(T, error)` returns.** The tuple encoding is
`Value{Raw: []Value{{Raw: val}, {}}}` where `{}` is a nil Value representing
nil error.

**Be pessimistic for boolean predicates.** If your function returns a boolean
and you don't have concrete inputs to evaluate it, return `true` rather than
`false`. This ensures the interpreter takes the non-trivial branch and
exercises more code.

**Return empty-but-non-nil for slice/map results.** Use `Value{Raw: []Value{}}`
for slices and `Value{Raw: map[string]interface{}{}}` for maps. This avoids
nil-slice panics in downstream range loops.

**Check `len(args)` before accessing arguments.** The interpreter calls your
intercept with however many arguments the call site provides. Method calls
include the receiver as `args[0]`, so `args[1]` is the first explicit
parameter.

---

## What's next

That covers the core Giri workflow. If you have questions or want to
contribute intercepts for popular third-party packages, the project is at
[github.com/scttfrdmn/giri](https://github.com/scttfrdmn/giri).

- **Part 1:** [Introducing Giri](01-introduction.md)
- **Part 2:** [How Giri Works](02-architecture.md)
