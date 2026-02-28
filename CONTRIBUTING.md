# Contributing to Giri

Thank you for your interest in contributing to Giri. This document covers everything
you need to get started: setting up your development environment, running the test
suite, and submitting changes.

---

## Table of Contents

1. [Development Setup](#development-setup)
2. [Running Tests](#running-tests)
3. [Code Style and Lint](#code-style-and-lint)
4. [Commit Conventions](#commit-conventions)
5. [Pull Request Workflow](#pull-request-workflow)
6. [Adding a Stdlib Intercept](#adding-a-stdlib-intercept)
7. [Adding an Integration Test](#adding-an-integration-test)
8. [Project Structure](#project-structure)

---

## Development Setup

### Prerequisites

- Go 1.23 or later
- `GOEXPERIMENT=arenas` enabled for all builds and tests (the arena packages are
  experimental and must be unlocked explicitly)

### Clone and build

```bash
git clone https://github.com/scttfrdmn/giri.git
cd giri
GOEXPERIMENT=arenas go build ./...
```

### Install the CLI locally

```bash
GOEXPERIMENT=arenas go install ./cmd/giri
```

---

## Running Tests

Always use `GOEXPERIMENT=arenas` when running tests; without it, arena-related
tests will fail to compile.

```bash
# Run the full test suite
GOEXPERIMENT=arenas go test ./...

# Run with race detection enabled
GOEXPERIMENT=arenas go test -race ./...

# Run integration tests only (verbose)
GOEXPERIMENT=arenas go test -v -run TestIntegration ./pkg/interpreter/

# Run showcase tests only
GOEXPERIMENT=arenas go test -v -run TestShowcase ./pkg/interpreter/

# Run unit tests for a specific package
GOEXPERIMENT=arenas go test ./pkg/shadow/
GOEXPERIMENT=arenas go test ./pkg/detector/
```

### Running fuzz tests

```bash
# Fuzz the shadow memory allocator
GOEXPERIMENT=arenas go test -fuzz=FuzzShadowMemory -fuzztime=30s ./pkg/shadow/

# Fuzz the SSA expression evaluator
GOEXPERIMENT=arenas go test -fuzz=FuzzBinOp -fuzztime=30s ./pkg/interpreter/
```

---

## Code Style and Lint

Giri uses [golangci-lint](https://golangci-lint.run) for static analysis. Install it
and run before submitting a PR:

```bash
# Install (macOS / Linux)
brew install golangci-lint          # macOS
# or
curl -sSfL https://raw.githubusercontent.com/golangci-lint/golangci-lint/HEAD/install.sh \
  | sh -s -- -b $(go env GOPATH)/bin latest

# Run linters
GOEXPERIMENT=arenas golangci-lint run ./...
```

Zero lint warnings are expected. If your change introduces a lint warning that is a
false positive, add a targeted `//nolint:rulename // reason` comment.

### Style notes

- **No new external dependencies.** Giri has a single external dependency
  (`golang.org/x/tools`). PRs that add new module dependencies will not be merged
  without discussion.
- **Follow existing patterns.** Look at neighboring code in the same package before
  introducing a new pattern.
- **Document exported symbols.** Every exported type, function, and field should have
  a doc comment.
- **Keep intercepts pessimistic.** Stdlib intercepts must return non-nil values for
  pointer/interface return types. See the
  [custom intercepts blog post](docs/blog/03-custom-intercepts.md) for guidance.

---

## Commit Conventions

Commits should follow this format:

```
<type>: <short summary>

[optional body]

[optional footer: "Closes #<issue>"]
```

**Types:**

| Type | When to use |
|------|-------------|
| `feat` | New feature or API addition |
| `fix` | Bug fix |
| `test` | Adding or updating tests |
| `docs` | Documentation changes only |
| `chore` | Dependency updates, CI changes, build tooling |
| `refactor` | Code restructuring with no behavior change |
| `perf` | Performance improvements |

**Examples:**

```
feat: add Config.Intercepts API for custom stdlib modeling

fix: nil-interface type-switch dispatch enters first case incorrectly

test: add type_switch_nil regression for issue #41

docs: add CONTRIBUTING.md and SECURITY.md

Closes #115
```

---

## Pull Request Workflow

1. **Open an issue first** for non-trivial changes. This lets maintainers give
   feedback on the approach before you invest time in implementation.

2. **Fork and branch:**

   ```bash
   git checkout -b feat/your-feature
   ```

3. **Make your changes**, following the style notes above.

4. **Add tests.** All new behavior must be covered by a test — either a unit test
   in the package or an integration test (see below).

5. **Verify everything passes:**

   ```bash
   GOEXPERIMENT=arenas go build ./...
   GOEXPERIMENT=arenas go test ./...
   GOEXPERIMENT=arenas go vet ./...
   GOEXPERIMENT=arenas golangci-lint run ./...
   ```

6. **Open the PR** against `main`. Fill in the description with:
   - What the change does
   - Why it is needed
   - How it was tested
   - `Closes #<issue>` if applicable

7. **Respond to review comments.** A single approval from a maintainer is required
   to merge.

---

## Adding a Stdlib Intercept

Stdlib intercepts live in `pkg/interpreter/stdlib.go`. Each intercept models the
behavior of a standard library function so the interpreter can reason about it
without access to the compiled native code.

### Steps

1. Identify the package path and function name, e.g. `"net/url"`, `"Parse"`.

2. Find (or create) the handler function for the package. Pattern:

   ```go
   func (interp *Interpreter) handleURLCall(pkgPath, name string, args []Value) (Value, bool) {
       switch name {
       case "Parse":
           // (url, error) tuple — return opaque non-nil url, nil error
           opaque := Value{Raw: struct{}{}}
           return Value{Raw: []Value{opaque, {}}}, true
       }
       return Value{}, false
   }
   ```

3. Add a `case` in `execStdlibCall`'s switch:

   ```go
   case "net/url":
       return interp.handleURLCall(pkgPath, name, args)
   ```

4. Add an integration test (see next section).

5. Update the package count comment at the top of `stdlib.go`.

### Return value encoding

| Return type | Value encoding |
|-------------|----------------|
| `void` | `Value{}` |
| `bool` | `Value{Raw: true}` (pessimistic) |
| `string` | `Value{Raw: "sentinel"}` |
| `int` (concrete) | `Value{Raw: int64(n)}` |
| `*T` (opaque) | `Value{Raw: struct{}{}}` |
| `(T, error)` | `Value{Raw: []Value{{Raw: v}, {}}}` |
| `(T, bool)` | `Value{Raw: []Value{{Raw: v}, {Raw: false}}}` |
| `[]byte` | `Value{Raw: []byte{}}` |

See `docs/blog/03-custom-intercepts.md` for a detailed guide.

---

## Adding an Integration Test

Integration tests live under `pkg/interpreter/testdata/integration/<name>/main.go`
and are registered in `pkg/interpreter/interpreter_integration_test.go`.

### Create the test program

```bash
mkdir -p pkg/interpreter/testdata/integration/my_feature
```

Write a minimal Go program in `main.go` that exercises the code path:

```go
// my_feature verifies that <description>.
// Expected: 0 violations.
package main

func main() {
    // ... exercise the feature ...
}
```

Add an `// Expected: N violations.` comment so reviewers understand the intent.

### Register the test

In `pkg/interpreter/interpreter_integration_test.go`, find the section for the
current version and add an entry:

```go
{
    name:           "my feature",
    dir:            "my_feature",
    wantViolations: 0,
    wantCategory:   "",
    config:         interpreter.DefaultConfig(),
},
```

If the test expects a specific violation category (e.g. `"data-race"`), set both
`wantViolations` and `wantCategory`.

### Run the new test

```bash
GOEXPERIMENT=arenas go test -v -run "TestIntegration/my_feature" ./pkg/interpreter/
```

---

## Project Structure

```
giri/
├── cmd/giri/            CLI entry point and flag parsing
├── pkg/
│   ├── interpreter/     SSA interpreter core (exec.go, stdlib.go, ...)
│   ├── shadow/          Shadow memory: allocation tracking, provenance
│   ├── detector/        Composable safety checkers (race, bounds, unsafe, arena)
│   ├── scheduler/       Goroutine scheduling (RoundRobin, Random, PCT)
│   └── report/          Output formatting (text, JSON, SARIF)
├── internal/
│   └── ssautil/         SSA loading from Go packages
├── docs/
│   ├── blog/            Blog post series (introduction, architecture, intercepts)
│   └── paper/           Academic paper (ECOOP/ISSTA style)
└── testdata/            Programs with known UB used by integration tests
```
