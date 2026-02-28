# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.25.0] - 2026-02-27

### Added

- **`net/url` intercepts** (issue #89): New `handleNetURLCall` in `stdlib.go` covers
  `url.Parse`, `ParseQuery`, `ParseRequestURI`, `QueryEscape/Unescape`,
  `PathEscape/Unescape`, `User/UserPassword`, `JoinPath`; and URL/Values method
  dispatches (`Hostname`, `Port`, `RequestURI`, `IsAbs`, `String`, `Query`,
  `Get/Set/Add/Del/Has/Encode`, etc.). Direct struct field accesses on `*url.URL`
  are opaque; tests use method calls only. Two integration tests: `url_parse`,
  `url_values`.

- **`os/exec` intercepts** (issue #90): New `handleExecCall` covers
  `exec.Command/CommandContext`, `LookPath`, and `*Cmd` method calls
  (`Run`, `Output`, `CombinedOutput`, `Start`, `Wait`, `StdoutPipe`,
  `StderrPipe`, `StdinPipe`, `String`, `Environ`). Integration test:
  `exec_command`, `exec_lookpath`.

- **`compress/gzip` and `compress/zlib` intercepts** (issue #91): New
  `handleGzipCall` and `handleZlibCall` cover `NewReader`, `NewWriter`,
  `NewWriterLevel` and all `*Reader/*Writer` method calls (`Read`, `Write`,
  `Flush`, `Close`, `Reset`, `Multistream`). Note: `gzip.NewWriter` returns a
  single `*Writer`; `zlib.NewWriter` returns a single `*Writer` (no error).
  Integration tests: `gzip_readwrite`, `zlib_compress`.

- **`sync.Pool`, `sync.Cond`, `sync.RWMutex.TryLock/TryRLock`, `sync.Map.Range`
  intercepts** (issue #92):
  - `sync.Pool.Get` returns `Value{}` (nil, triggering the allocation fallback
    branch); `Put` is a noop.
  - `sync.Cond.Signal/Broadcast` ticks the vector clock and snapshots it;
    `Cond.Wait` merges the last snapshot (same HB model as Mutex Lock/Unlock).
    `sync.NewCond` returns an opaque value.
  - `sync.RWMutex.TryLock/TryRLock` return `true` (optimistic); lock state is
    recorded so the corresponding `Unlock/RUnlock` finds the expected state.
  - `sync.Map.Range` probes the callback function with sentinel args, executing
    its body for analysis. Integration tests: `sync_pool`, `sync_cond`.

## [0.24.0] - 2026-02-27

### Fixed

- **3-index slice expressions** (issue #85): `s[low:high:max]` was previously
  silently discarding `inst.Max`, causing the resulting slice to inherit the
  source capacity instead of `max - low`. The `*ssa.Slice` handler in `exec.go`
  now reads `inst.Max` and sets `Cap = maxVal - lowVal`. The bounds check is
  also extended to validate `0 ≤ low ≤ high ≤ max ≤ cap(s)`; a violation outside
  this range fires `OutOfBoundsError`. Integration test: `slice_3index`.

### Added

- **`reflect` package intercepts** (issue #86): New `handleReflectCall` in
  `stdlib.go` covers the full `reflect` API surface:
  - `TypeOf` (opaque non-nil Type), `ValueOf` (identity), `DeepEqual` (concrete
    args call real `reflect.DeepEqual`; opaque → `true` pessimistic)
  - Constructor functions: `New`, `Zero`, `MakeSlice`, `MakeMap`, `MakeChan`,
    `MakeFunc`, `Append`, `AppendSlice`, `Copy`, `Indirect`, `PtrTo`,
    `SliceOf`, `ArrayOf`, `MapOf`, `ChanOf`, `FuncOf`, `StructOf`
  - `reflect.Value` methods: `Kind`, `Type`, `Interface`, `Elem`, `Field`,
    `Index`, `MapIndex`, `MapKeys`, `NumField`, `NumMethod`, `Method`,
    `MethodByName`, `Len`, `Cap`, `IsNil`, `IsValid`, `IsZero`, `CanAddr`,
    `CanSet`, `Set*`, `Int`, `Uint`, `Float`, `Bool`, `String`, `Bytes`,
    `Addr`, `Call`, `Convert`, `Recv`, `Send`, `Close`, `TrySend`, `TryRecv`
  - `reflect.Type` methods: `Name`, `PkgPath`, `Size`, `Implements`,
    `AssignableTo`, `ConvertibleTo`, `Comparable`, `In`, `Out`, `NumIn`,
    `NumOut`, `Key`, `ChanDir`, `IsVariadic`, `Bits`, `FieldByName`,
    `FieldByIndex`, `FieldByNameFunc`, `Align`, `FieldAlign`
  Two integration tests: `reflect_type_of`, `reflect_deep_equal`.

- **`encoding/xml` and `encoding/csv` intercepts** (issue #87): New
  `handleXMLCall` and `handleCSVCall` in `stdlib.go`:
  - `encoding/xml`: `Marshal` (calls real `xml.Marshal` for concrete values),
    `MarshalIndent`, `Unmarshal`, `NewDecoder`, `NewEncoder`, `Decode`,
    `DecodeElement`, `Token`, `Encode`, `EncodeElement`, `EncodeToken`,
    `Flush`, `EscapeText`, `Escape`, `CopyToken`
  - `encoding/csv`: `NewReader`, `NewWriter`, `Read` (sentinel record),
    `ReadAll` (single-row sentinel), `Write`, `WriteAll`, `Flush`, `Error`
  Two integration tests: `xml_marshal`, `csv_readall`.

- **`flag` and `runtime` package intercepts** (issue #88): New `handleFlagCall`
  and `handleRuntimeCall` in `stdlib.go`:
  - `flag`: `String`/`Int`/`Int64`/`Uint`/`Uint64`/`Bool`/`Float64`/`Duration`
    return non-nil pointers to zero values; `*Var` variants return nil (set in
    place); `Parse` noop; `Parsed` → `true`; `Arg`/`Args`/`NArg`/`NFlag`
    sentinels; `Lookup` → nil; `Set` → nil; `PrintDefaults`/`Visit`/`Usage`
    noops; `CommandLine`/`NewFlagSet` → opaque
  - `runtime`: `NumCPU` (real value), `GOMAXPROCS` (real query/set),
    `NumGoroutine` → 1, `Caller`/`Callers` conservative, `GC`/`Gosched`/
    `LockOSThread`/`UnlockOSThread` noops, `Version`/`GOARCH`/`GOOS`/`GOROOT`
    return real values, `Stack` → 0, `SetFinalizer`/`KeepAlive`/`ReadMemStats`
    noops
  Two integration tests: `flag_parse`, `runtime_numcpu`.

Closes #85, #86, #87, #88. Integration test count: 88 total.

## [0.23.0] - 2026-02-27

### Added

- **`encoding/hex` and `encoding/base64` intercepts** (issue #81): New
  `handleHexCall` and `handleBase64Call` in `stdlib.go`:
  - `encoding/hex`: `EncodeToString`, `DecodeString`, `Encode`, `Decode`,
    `EncodedLen`, `DecodedLen`, `NewEncoder`, `NewDecoder`, `Dump`
  - `encoding/base64`: all `*Encoding` methods (`EncodeToString`,
    `DecodeString`, `Encode`, `Decode`, `EncodedLen`, `DecodedLen`),
    `NewEncoding`, `NewEncoder`, `NewDecoder`
  Concrete arguments call the real stdlib; opaque arguments return sentinels
  (`"deadbeef"`, `"aGVsbG8="`, etc.) to keep downstream branches reachable.
  Two integration tests: `hex_encode`, `base64_encode`.

- **`crypto/rand` and hash package intercepts** (issue #82): New
  `handleCryptoRandCall` and `handleHashCall` in `stdlib.go`:
  - `crypto/rand`: `Read` (returns filled-length), `Int`, `Prime`
  - `crypto/md5`, `crypto/sha1`, `crypto/sha256`, `crypto/sha512`:
    `New`/`New224`/`New384`, package-level `Sum`/`Sum256`/`Sum512`,
    `Write`, `Sum` (digest), `Reset`, `Size`, `BlockSize`, `Sum32`/`Sum64`
  All four hash packages share one handler, keyed by `pkgPath` for correct
  `digestLen` (16/20/32/64).  Imports `encoding/base64`, `encoding/hex`,
  `net`, `path/filepath` added to stdlib.go.
  Two integration tests: `crypto_rand_read`, `hash_sha256`.

- **`path/filepath` and `path` intercepts** (issue #83): New
  `handleFilepathCall` and `handlePathCall` in `stdlib.go`:
  - `path/filepath`: `Join`, `Dir`, `Base`, `Ext`, `Abs`, `Clean`,
    `IsAbs`, `Split`, `Rel`, `Match`, `Glob`, `Walk`/`WalkDir` (noop),
    `FromSlash`, `ToSlash`, `VolumeName`, `EvalSymlinks`, `SplitList`
  - `path` (slash-only): `Join`, `Dir`, `Base`, `Ext`, `Clean`, `IsAbs`,
    `Split`, `Match`
  Concrete string arguments call the real `filepath.*` functions; opaque
  arguments return sensible path sentinels.
  Two integration tests: `filepath_join`, `path_basic`.

- **`net` utility and `text/template`/`html/template` intercepts** (issue #84):
  New `handleNetCall` and `handleTemplateCall` in `stdlib.go`:
  - `net`: `SplitHostPort`, `JoinHostPort`, `ParseIP`, `ParseCIDR`,
    `LookupHost`, `LookupPort`, `LookupIP`/`TXT`/`MX`/`NS`/`CNAME`,
    `ResolveTCPAddr`/`UDPAddr`/`IPAddr`/`UnixAddr`, `Dial`/`DialTimeout`,
    `Listen`/`ListenPacket`, `Pipe`, `IPv4`, `IPv4Mask`, `CIDRMask`
  - `text/template` and `html/template`: `New`, `Must`, `ParseFiles`,
    `ParseGlob`, `Parse`, `ParseFS`, `Execute`, `ExecuteTemplate`,
    `Funcs`, `Delims`, `Lookup`, `Name`, `Clone`, `Templates`, `Option`,
    `HTMLEscape`/`JSEscape` variants
  Concrete network arguments delegate to the real `net.*` pure functions
  (no I/O); opaque arguments return sentinel host/port values.
  Two integration tests: `net_parse`, `template_execute`.

Closes #81, #82, #83, #84. Integration test count: 81 total.

## [0.22.0] - 2026-02-27

### Added

- **`sync/atomic` intercepts** (issue #77): New `handleAtomicCall` in `stdlib.go`
  models all atomic operations (Load, Store, Add, CompareAndSwap, Swap, And, Or)
  on all integer widths (int32, int64, uint32, uint64, uintptr) and pointer.
  Reads and writes use `interp.valueStore` keyed by the pointer's `AllocID`,
  giving correct sequential semantics without calling `handleLoad`/`handleStore`
  (avoiding false-positive race reports on atomic accesses).
  Two integration tests: `atomic_counter`, `atomic_cas`.

- **`io` and `bufio` package intercepts** (issue #78): New `handleIOCall` and
  `handleBufioCall` in `stdlib.go` model the most common operations:
  - `io`: `ReadAll`, `Copy`/`CopyBuffer`/`CopyN`, `WriteString`, `Pipe`,
    `NopCloser`, `LimitReader`, `MultiReader`, `MultiWriter`, `TeeReader`,
    `NewSectionReader`, `ReadAtLeast`, `ReadFull`, `Discard`
  - `bufio`: `NewReader`/`NewReaderSize`, `NewWriter`/`NewWriterSize`,
    `NewScanner`, scanner methods (`Scan`, `Text`, `Bytes`, `Err`, `Split`, `Buffer`),
    reader methods (`ReadString`, `ReadLine`, `ReadByte`, `ReadRune`, `Peek`,
    `ReadSlice`, `UnreadByte`, `UnreadRune`), writer methods (`Write`, `WriteString`,
    `WriteByte`, `WriteRune`, `Flush`, `Available`, `Buffered`, `Size`, `Reset`, `Discard`)
  Two integration tests: `io_readall`, `bufio_scanner`.

- **`strings.Builder` and `bytes.Buffer` method intercepts** (issue #79):
  Extended `handleStringsCall` and `handleBytesCall` with method cases that fire
  when the SSA callee package path is `"strings"` or `"bytes"`:
  - `strings.Builder`: `WriteString`, `WriteByte`, `WriteRune`, `Write`, `String`,
    `Len`, `Cap`, `Reset`, `Grow`
  - `bytes.Buffer`: `Write`, `WriteString`, `WriteByte`, `WriteRune`, `String`,
    `Bytes`, `Len`, `Cap`, `Reset`, `Truncate`, `Grow`, `ReadFrom`, `WriteTo`,
    `ReadByte`, `ReadRune`, `ReadString`, `ReadBytes`, `Next`, `UnreadByte`,
    `UnreadRune`
  Two integration tests: `strings_builder`, `bytes_buffer`.

- **`log` package intercepts** (issue #80): New `handleLogCall(gid, name, args)`
  in `stdlib.go` models the standard `log` package:
  - `Print`/`Println`/`Printf` → noop
  - `Fatal`/`Fatalln`/`Fatalf` → marks all goroutines `Panicked` (simulates
    `os.Exit(1)`)
  - `Panic`/`Panicln`/`Panicf` → marks current goroutine `Panicked`
  - `New` → returns opaque `*log.Logger`; `SetOutput`, `SetFlags`, `SetPrefix`,
    `Flags`, `Prefix`, `Writer`, `Default` also intercepted
  Two integration tests: `log_print`, `log_fatal`.

Closes #77, #78, #79, #80. Integration test count: 73 total.

## [0.21.0] - 2026-02-27

### Fixed

- **`ssa.Index` string byte semantics** (issue #73): `s[i]` now returns the
  **byte** at byte position `i` (Go's actual semantics). Previously the string
  was converted to `[]rune` and indexed by rune position, which produced wrong
  results for multibyte UTF-8 characters.

- **`rangeIter.advance()` string byte offsets** (issue #73): `for i, r := range s`
  now yields **byte offsets** as `i`. Fixed by replacing `[]rune` iteration with
  `utf8.DecodeRuneInString` so each step advances by the byte width of the decoded
  rune. Import `"unicode/utf8"` added to `exec.go`.

- **`ssa.Convert` type conversions** (issue #74): New `convertValue` helper in
  `exec.go` implements three previously-missing conversion patterns:
  - `int`/`rune` → `string`: `string(65)` now produces `"A"` (not the integer)
  - `string` → `[]byte`: `[]byte("hi")` now produces `{0x68,0x69}`
  - `[]byte` → `string`: `string([]byte{…})` now produces the correct string
  - `float64` → `int64` truncation and `int64` → `float64` promotion also corrected.
  Called from the non-unsafe branch of `*ssa.Convert`.

### Added

- **`unicode/utf8` intercepts** (issue #75): New `handleUTF8Call` in `stdlib.go`.
  `RuneCountInString`, `ValidString`, `ValidRune`, `RuneLen`, `EncodeRune`,
  `DecodeRuneInString`, `DecodeLastRuneInString`, `FullRune*` all use the real
  `unicode/utf8` functions for concrete args; return conservative values for opaque.

- **`unicode` intercepts** (issue #75): New `handleUnicodeCall` in `stdlib.go`.
  `IsLetter`, `IsDigit`, `IsSpace`, `IsUpper`, `IsLower`, `IsPunct`, `IsNumber`,
  `IsGraphic`, `IsPrint` use real `unicode` functions for concrete rune args.
  `ToLower`, `ToUpper`, `ToTitle`, `SimpleFold` convert concretely.
  `"unicode/utf8"` and `"unicode"` registered in `execStdlibCall`.

- **`context` package intercepts** (issue #76): New `handleContextCall` in
  `stdlib.go`. `context.Background` and `context.TODO` return an opaque non-nil
  value; `WithCancel`, `WithTimeout`, `WithDeadline`, `WithCancelCause` return
  `(ctx, cancelFn)` tuples; `WithValue` returns an opaque context; `Err`, `Done`,
  `Value`, `Deadline`, `Cause` return conservative nil/false values.
  `"context"` registered in `execStdlibCall`.

- **6 new integration tests** (issues #73–#76): `string_byte_index`,
  `string_range_utf8`, `convert_string_rune`, `convert_bytes_string`,
  `utf8_rune_count`, `context_basic`. Total: **71 tests**.

## [0.20.0] - 2026-02-27

### Added

- **Go 1.21+ `min()` and `max()` builtins** (issue #69): `execBuiltin` now handles
  `"min"` and `"max"` (both variadic). Supports concrete `int64`, `float64`, and
  `string` raw values; returns conservative `Value{}` for opaque arguments.

- **Go 1.21+ `clear()` builtin** (issue #69): `execBuiltin` handles `"clear"` for
  both maps (race-checks via `handleStore`, then empties the interpreter map) and
  slices (race-checked no-op). Nil map/slice triggers `NilMapWriteError`, consistent
  with `delete`.

- **`encoding/json` intercepts** (issue #70): New `handleJSONCall` in `stdlib.go`.
  `json.Marshal` / `json.MarshalIndent` return `([]byte, nil)`; `json.Unmarshal`
  returns nil error; `json.NewDecoder` / `json.NewEncoder` return opaque values so
  downstream method calls are intercepted; `json.Valid` returns true; multi-return
  codec methods (`Decode`, `Encode`, `Token`) return conservative values.
  `"encoding/json"` registered in `execStdlibCall`.

- **`regexp` intercepts** (issue #71): New `handleRegexpCall` in `stdlib.go`.
  `regexp.Compile` returns `(*Regexp, nil)`; `regexp.MustCompile` returns an opaque
  Regexp; package-level `Match*` functions return `(true, nil)`; `*Regexp` method
  calls (`MatchString`, `FindString`, `FindAllString`, `ReplaceAllString`,
  `ReplaceAllStringFunc`, `Split`) return conservative values. `ReplaceAllStringFunc`
  probes its callback to surface any violations inside it. `"regexp"` registered in
  `execStdlibCall`.

- **`math` package intercepts** (issue #72): New `handleMathCall` in `stdlib.go`.
  Covers `Abs`, `Floor`, `Ceil`, `Round`, `Trunc`, `Sqrt`, `Cbrt`, `Pow`, `Pow10`,
  `Log`, `Log2`, `Log10`, `Exp`, `Exp2`, `Sin`, `Cos`, `Tan`, `Asin`, `Acos`,
  `Atan`, `Atan2`, `Min`, `Max`, `Mod`, `Hypot`, `Dim`, `Inf`, `IsInf`, `IsNaN`,
  `NaN`, `Signbit`, `Copysign`, `Frexp`, `Modf`, `Lgamma`, `Gamma`, and more.
  Concrete `float64` arguments call the real `math.*` function; opaque arguments
  return a safe non-NaN sentinel. `"math"` registered in `execStdlibCall`.

- **5 new integration tests**: `min_max_builtins`, `clear_map`, `json_marshal`,
  `regexp_match`, `math_ops`. Total: 65 tests.

## [0.19.0] - 2026-02-27

### Added

- **`bytes.*` intercepts** (issue #66): New `handleBytesCall` in `stdlib.go` mirrors
  `handleStringsCall`. Covers 30+ functions: predicates (`Contains`, `HasPrefix`,
  `HasSuffix`, `Equal`, `EqualFold`), index functions (`Index`, `Count`, `LastIndex`),
  transformers (`ToLower`, `ToUpper`, `TrimSpace`, `TrimPrefix`, `TrimSuffix`,
  `Replace`, `ReplaceAll`, `Repeat`), splitters (`Split`, `Fields`), and combiners
  (`Join`, `Cut`, `CutPrefix`, `CutSuffix`). `"bytes"` registered in `execStdlibCall`.

- **`errors.*` intercepts** (issue #67): New `handleErrorsCall` in `stdlib.go`.
  `errors.New(msg)` returns a real Go `error`; `errors.Is` compares error strings;
  `errors.As` returns `false` (conservative); `errors.Unwrap` returns nil;
  `errors.Join` returns the first non-nil error. `"errors"` registered in
  `execStdlibCall`.

- **`sort.*` intercepts** (issue #68): New `handleSortCall(gid, name, args, site)` in
  `stdlib.go`. `sort.Slice` and `sort.SliceStable` probe the comparator with `(0, 1)`
  to surface violations in user callbacks. `sort.Search` probes `f(n/2)`. `sort.Strings`,
  `sort.Ints`, `sort.Float64s` are noops. `sort.Find` probes `cmp(0)`.
  `execStdlibCall` signature extended with `gid int64` and `site string` to enable
  callbacks; the one additional call site in `executeDeferred` updated accordingly.

### Fixed

- **`fmt` output function return values** (issue #65): `fmt.Printf`, `fmt.Println`,
  `fmt.Print`, `fmt.Fprintf`, `fmt.Fprintln`, and `fmt.Fprint` now return
  `(n=1, err=nil)` instead of an empty `Value{}`. Callers that check `err != nil`
  or `n == 0` now take the correct non-error path.

- **`sort` callback free-variable ordering**: `probeCallback` in `handleSortCall` was
  prepending free variables before explicit params (wrong order). Fixed to append free
  vars after params, matching `execFunction`'s binding convention.

- **5 new integration tests**: `fmt_print_return`, `bytes_ops`, `errors_new`,
  `sort_slice`, `sort_strings`. Total: 60 tests.

## [0.18.0] - 2026-02-27

### Added

- **`sync.Once` support** (issue #61): `once.Do(f)` calls `f()` exactly once per
  `Once` instance and is a noop for all subsequent calls. `Interpreter` gains
  `onceState map[shadow.AllocID]bool`; `handleSyncCall` handles `"Do"`. To enable
  calling function-value arguments, `resolveValue` for `*ssa.Function` now returns
  `Value{Raw: f}` (the actual SSA function) instead of the function's string
  representation.

- **`os` package intercepts** (issue #62): `os.Exit(n)` is intercepted in
  `execCall` — all goroutines are marked `Panicked` to halt interpretation cleanly,
  preventing spurious violations from code that runs after an exit. `execStdlibCall`
  now also intercepts `os.Getenv`, `os.LookupEnv`, `os.Setenv`, `os.Getwd`, and
  common filesystem operations via a new `handleOSCall` in `stdlib.go`.

- **`delete()` builtin** (issue #63): The `delete(map, key)` builtin is now fully
  implemented. It performs a nil-map check (records `NilMapWriteError`), calls
  `handleStore` for race detection (deletion is a write), and removes the key from
  the interpreter's map representation. Previously it was a no-op.

- **`math/rand` intercepts** (issue #64): `math/rand` functions (`Intn`, `Int63n`,
  `Int63`, `Int`, `Float64`, `Perm`, `Shuffle`, `Read`, `Seed`, `New`,
  `NewSource`, etc.) are intercepted in a new `handleMathRandCall` in `stdlib.go`.
  `Interpreter` gains a `rng *rand.Rand` field seeded from `config.RandomSeed` for
  deterministic values. Without intercepts, programs using `math/rand` would try to
  interpret the stdlib global-locked source, which cannot be correctly modelled.

- **Six new integration tests**: `sync_once` (0 violations), `os_exit` (0
  violations), `os_getenv` (0 violations), `delete_race` (1 violation — data race),
  `safe_delete` (0 violations), `rand_intn` (0 violations). Total: 55 tests.

## [0.17.0] - 2026-02-27

### Added

- **`//giri:ignore` suppression directive** (issue #58): Source lines annotated with
  `//giri:ignore` (inline or on the preceding line) are silenced. `ParseSuppressions`
  in `internal/ssautil/loader.go` scans all loaded package syntax trees and builds a
  `map[string]bool` keyed by `"file:line"`. The interpreter checks
  `interp.suppressions[interp.currentSite]` inside `recordViolation` before appending
  a finding. Both inline (`code //giri:ignore rule 1`) and preceding-line
  (`//giri:ignore rule 1\ncode`) forms are supported.

- **Multi-package support** (issue #53): `LoadAllPrograms(patterns []string)` in
  `internal/ssautil/loader.go` loads all Go packages matching the given patterns,
  builds a shared SSA `*ssa.Program`, and returns one `*interpreter.Program` per
  `main` package found. `cmd/giri/main.go` now uses `LoadAllPrograms` and iterates
  over all returned programs, aggregating violations into a single report. Because
  `LoadProgram` has always used `NeedDeps` + `prog.Build()`, cross-package function
  bodies (e.g. a library called from `main`) are already available to `execFunction`;
  no interpreter changes were required.

- **GitHub Action** (issue #59): New composite action at
  `.github/actions/giri/action.yml`. Users can add a one-step Giri scan to any
  repository workflow. The action installs Giri, runs it against the specified
  packages, and optionally uploads the SARIF report to GitHub Code Scanning.
  Inputs: `packages`, `go-version`, `format`, `output-file`, `upload-sarif`,
  `fail-on-findings`, `extra-flags`. README CI Integration section updated with
  usage examples and input reference table.

- **Two new integration tests**: `suppress_ignore` (0 violations — misaligned load
  silenced by `//giri:ignore`), `multi_pkg` (1 violation — Rule 1 from an imported
  library package).

### Fixed

- `LoadProgram` now populates `Program.Suppressions` so single-package invocations
  also benefit from `//giri:ignore` directives.

## [0.16.0] - 2026-02-27

### Added

- **Stack alloc poisoning** (issue #51): `Frame` gains a `StackAllocs []shadow.AllocID`
  field. When `ssa.Alloc(Heap=false)` is executed, the alloc ID is appended to
  `frame.StackAllocs`. In `popFrame`, after deferred calls and `recomputeNamedReturns`
  have run, every stack alloc is poisoned via `Memory.Poison` (sets `Freed=true`).
  Any subsequent `CheckAccess` on a poisoned alloc returns the existing
  `UseAfterFreeError`. Go's SSA escape analysis guarantees that `Heap=false` allocs
  never have surviving external references, so this is always safe and never produces
  false positives in well-formed programs.

- **Bounded `valueStore`** (issue #60): `popFrame` now calls
  `delete(interp.valueStore, id)` for each alloc ID in `frame.StackAllocs`
  immediately after poisoning. This evicts stack alloc entries from `valueStore`
  precisely when the owning frame exits, keeping the map bounded to live heap and
  global allocs. For programs with many short-lived function calls (e.g.
  tight loops with named returns), this substantially reduces peak `valueStore` size.
  The cleanup is a subset of Option A from the issue: heap alloc entries are retained
  since their lifetimes may span multiple frames.

- `Memory.Poison(id AllocID, site string)`: new method on `*Memory` that sets an
  allocation's `Freed=true` unconditionally. Unlike `Free`, it does not check for
  double-free and does not return an error — stack allocs are always live when their
  frame exits.

- Two new integration tests: `safe_stack_alloc` (named-return struct with
  `Heap=false` alloc, 0 violations), `bounded_value_store` (100-iteration loop
  over named-return struct calls, verifies eviction does not corrupt return values,
  0 violations). Total: 47 integration tests.

- Closes issues #51, #60.

## [0.15.0] - 2026-02-27

### Added

- **Deadlock detection** (issue #56): `checkGoroutineLeaks` now distinguishes between
  a goroutine leak (main exits normally, spawned goroutines blocked) and a global
  deadlock (all goroutines blocked, none finished). A `shadow.DeadlockError` is emitted
  when `allBlocked && blocked > 0 && !anyFinished`. The `GoroutineCount` field reports
  how many goroutines are stuck. Mirrors Go runtime's "all goroutines are asleep —
  deadlock!" message. New integration test: `deadlock` (1 violation, "deadlock").
  New showcase: `testdata/showcase/deadlock`.

- **WaitGroup negative counter detection** (issue #57): `mutexState` gains a `wgCounter int`
  field. The `handleSyncCall` handler now tracks `Add(delta)` (increments) and `Done()`
  (decrements, equivalent to `Add(-1)`). When the counter goes below zero a
  `shadow.WaitGroupNegativeError` is recorded with the goroutine ID and site. Catches
  both `wg.Done()` called without a prior `Add(1)` and extra `Done()` calls in deferred
  cleanup. New integration test: `wg_negative` (1 violation, "waitgroup").
  New showcase: `testdata/showcase/wg_negative`.

- **PCT MultiRun — `RunN`** (issue #50): new exported function
  `RunN(prog *Program, config Config, n int, seed int64) *RunResult` in
  `pkg/interpreter/exec.go`. Runs the program `n` times, each with a fresh PCT
  schedule derived from successive seeds. Violations are deduplicated by error string;
  the first unique occurrence per class is returned. The `--runs N` CLI flag
  (N > 1) activates `RunN` and implies PCT scheduling. `TestShowcase` is extended
  to support `runs`/`seed` fields, using `RunN` when `runs > 0`.
  New showcase: `testdata/showcase/pct_race` — a WaitGroup ordering bug that
  round-robin scheduling always misses but PCT finds in ~41% of individual runs.

- Two new error types in `pkg/shadow/errors.go`: `DeadlockError`, `WaitGroupNegativeError`.
  Two new `classifyError` cases in `pkg/report/report.go`.

- Closes issues #50, #56, #57.

## [0.14.0] - 2026-02-27

### Added

- **Double-close channel detection** (issue #52): `handleChannelClose` now emits a
  typed `shadow.DoubleCloseError` when a channel that is already closed is closed
  again. Previously the check existed but used an untyped `fmt.Errorf`; the typed
  error enables structured reporting and `classifyError` classification.
  New integration test: `double_close` (1 violation, "closed channel").

- **Nil map write detection** (issue #54): the `ssa.MapUpdate` handler now checks
  whether the map value has a nil backing (`m.Raw == nil`) before performing any
  race check or update. A `shadow.NilMapWriteError` is recorded and execution of
  the current block stops. Catches `var m map[K]V; m[k] = v` patterns that would
  panic at runtime with "assignment to entry in nil map".
  New integration test: `nil_map_write` (1 violation, "nil map").

- **Division by zero detection** (issue #55): the `ssa.BinOp` handler now checks
  `token.QUO` and `token.REM` operations for a statically-known zero `int64`
  divisor before delegating to `evalBinOp`. A `shadow.DivisionByZeroError` is
  recorded. Float division by zero is intentionally excluded (produces ±Inf, not a
  panic). Catches patterns like `ratio(10, 0)` traceable through SSA.
  New integration test: `div_zero` (1 violation, "division by zero").

- Three new error types in `pkg/shadow/errors.go`:
  `DoubleCloseError`, `NilMapWriteError`, `DivisionByZeroError`.
  Three new `classifyError` cases in `pkg/report/report.go` with structured
  categories and remediation hints.

- Closes issues #52, #54, #55.

## [0.13.0] - 2026-02-26

### Fixed

- **`executeDeferred` silently dropped non-arena deferred calls** (issue #47):
  `DeferredCall` now carries a full callable reference (`Callee *ssa.Function`,
  `IsClosure bool`, `ClosureVal *ClosureValue`, `PkgPath`, `FuncName`).
  `executeDeferred` dispatches to: closure via `execFunction` with free-var
  bindings, sync package via `handleSyncCall`, stdlib via `execStdlibCall`,
  arena.Free, or general SSA function body. Previously all non-arena defers
  were silently ignored.

- **`ssa.MakeClosure` in `ssa.Defer` / `ssa.Go`** (part of issue #47): The SSA
  library's `StaticCallee()` looks through `*ssa.MakeClosure` and returns the
  underlying function, making `inst.Call.Args` appear empty. Both `ssa.Defer`
  and `ssa.Go` handlers now detect `inst.Call.Value.(*ssa.MakeClosure)` and
  extract the free-variable bindings from `mc.Bindings` directly, fixing
  nil-pointer dereferences in deferred/goroutine closures that capture variables.

- **Multi-level panic/recover broken** (issue #48): `ssa.Panic` previously
  cleared the call stack eagerly before any deferred `recover()` could fire.
  The interpreter now uses a lazy unwind model: `ssa.Panic` sets `g.Panicking`
  and returns; the existing Go-level `defer interp.popFrame(gid)` in
  `execFunction` handles per-frame unwinding. `popFrame` temporarily clears
  `Panicking` before each deferred call so `recover()` can intercept it.
  `recover()` sets `g.Recovered = true` to signal `popFrame` to stop unwinding.

- **Named return values not updated by deferred functions** (issue #49): Added
  `valueStore map[shadow.AllocID]Value` to the interpreter tracking the last
  value written through each allocation. `handleStore` records every offset-0
  store; `handleLoad` returns the stored value when available. After all defers
  run, `recomputeNamedReturns` re-reads named-return allocs from `valueStore` so
  the actual return value reflects deferred mutations.

### Added

- **Four new integration tests** (v0.13.0 regression suite):
  - `defer_unlock` — deferred `sync.Mutex.Unlock()` dispatched via sync handler
    (0 violations)
  - `defer_user_func` — deferred named user function with pointer arg executed
    via `execFunction` (0 violations)
  - `multi_recover` — inner panic recovered by deferred `recover()` in outer
    function; main continues normally (0 violations)
  - `named_return_defer` — deferred closure multiplies named return value; result
    reflects the deferred write (0 violations)

### Closes
- Issue #47 (executeDeferred silently drops non-arena deferred calls)
- Issue #48 (multi-level panic/recover broken)
- Issue #49 (named return values not updated by deferred functions)

## [0.12.0] - 2026-02-26

### Added

- **Buffered channel capacity modeling** (issue #44): `chanEntry` now tracks
  `capacity` and `pendingCount` fields. `createChannel(capacity int)` accepts a
  capacity argument, populated from the `ssa.MakeChan` operand. Send to a full
  buffered channel marks the sending goroutine `GoroutineBlocked` with
  `BlockOnSend = true`; `handleChannelRecv` decrements `pendingCount` when a
  buffered value is consumed. `checkGoroutineLeaks` now reports both recv-blocked
  and send-blocked goroutines, using `channelReceivers` (new, symmetric to
  `channelSenders`) to suppress false positives.

- **Blocking select goroutine detection** (issue #45): `ssa.Select` with
  `inst.Blocking = true` and no ready case now marks the current goroutine
  `GoroutineBlocked`, enabling goroutine-leak detection for select-blocked
  goroutines.

- **`time.After` / `time.Sleep` intercepts** (issue #45): `time.After` creates a
  buffered channel (capacity 1) pre-populated with a pending value so select
  timeout arms fire immediately without blocking. `time.Sleep`, `time.Now`,
  `time.Since`, `time.Unix`, `time.NewTimer`, and `time.NewTicker` are modeled
  as noops.

- **Map race detection via shadow memory** (issue #46): `ssa.MakeMap` now
  allocates a shadow `*Pointer` stored as the `Value.Provenance` of the map.
  `ssa.Lookup` and `ssa.MapUpdate` call `handleLoad`/`handleStore` through the
  race detector when the map has a shadow provenance, enabling vector-clock race
  detection for unsynchronized concurrent map access.

- **`sync.Map` noop intercepts** (issue #46): `handleSyncCall` now intercepts
  `sync.Map` methods (`Store`, `Load`, `LoadOrStore`, `Delete`, `LoadAndDelete`,
  `Range`, `Swap`, `CompareAndSwap`, `CompareAndDelete`) as noops, preventing
  false-positive races on `sync.Map`-backed storage.

- **Five new integration tests**:
  - `buffered_chan_overflow` — goroutine blocked sending to a full buffered
    channel with no receiver (1 violation, "goroutine leak")
  - `select_default` — non-blocking select with default; no ready case takes
    default without blocking (0 violations)
  - `select_timeout` — blocking select with `time.After` arm; timeout fires
    immediately via intercept (0 violations)
  - `map_race` — two sibling goroutines writing to the same map without sync
    (1 violation, "data race")
  - `sync_map_no_race` — concurrent `sync.Map` access (0 violations)

### Closes
- Issue #44 (buffered channel capacity modeling)
- Issue #45 (blocking select / time.After)
- Issue #46 (map race detection / sync.Map)

## [0.11.0] - 2026-02-26

### Added

- **Standard library intercepts** (`pkg/interpreter/stdlib.go`): a new
  `execStdlibCall` dispatcher intercepts calls to `strings.*`, `strconv.*`,
  and `fmt.*` before `execCall` falls back to either interpreting the body or
  returning an opaque `Value{}`. Previously, all three packages returned opaque
  zero values for every function, causing downstream `ssa.If` branches to
  permanently take the false/zero path and miss violations that are only
  reachable from the true branch.

  - **`strings` package** — `Contains`, `HasPrefix`, `HasSuffix`, `EqualFold`,
    `Index`, `LastIndex`, `Count`, `TrimSpace`, `Trim`, `TrimLeft`, `TrimRight`,
    `TrimPrefix`, `TrimSuffix`, `ToUpper`, `ToLower`, `ToTitle`, `Replace`,
    `ReplaceAll`, `Repeat`, `Split`, `SplitN`, `SplitAfter`, `Fields`, `Join`,
    `Map`, `Compare`, `Cut`, `ContainsAny`, `ContainsRune`, `IndexByte`,
    `IndexRune`, `IndexAny`. For concrete string arguments the real Go stdlib
    function is called; for non-concrete inputs a pessimistic non-zero result is
    returned (bool predicates → `true`, string results → `"x"`, integer results
    → `0`).

  - **`strconv` package** — `Itoa`, `Atoi`, `FormatInt`, `FormatUint`,
    `FormatBool`, `FormatFloat`, `ParseInt`, `ParseUint`, `ParseFloat`,
    `ParseBool`, `Quote`, `Unquote`, `Append*`. Parse functions return real
    parsed values for concrete inputs and a non-zero sentinel `(1, nil)` for
    non-concrete inputs, ensuring the success path is explored.

  - **`fmt` package** — `Sprintf`, `Errorf`, `Sprint`, `Sprintln`,
    `Println`/`Printf`/`Print`/`Fprintln`/`Fprintf`/`Fprint` (side-effect
    noop), `Sscanf`/`Sscan`/`Sscanln`. `Sprintf` and `Sprint` call the real
    Go formatter with concrete args; non-concrete args are rendered as `"?"` to
    avoid nil-format panics. `Errorf` returns a real non-nil `error` value.

  The intercept check intentionally precedes the `execFunction` fallthrough so
  it fires even when the package source is loaded (packages like `fmt` have
  interpretable SSA bodies but internally use reflection and runtime primitives
  that cannot be modeled). Closes issues #42 and #43.

- **Integration tests** (3 new programs in `pkg/interpreter/testdata/integration/`):
  - `strings_intercept` — `strings.Contains("hello world", "world")` returns
    `true`, entering a branch with a misaligned Rule 1 access. Without
    intercepts the branch was never entered. Expects 1 `"rule 1"` violation.
  - `strconv_atoi` — `strconv.Atoi("42")` returns `(42, nil)`; `n > 0` enters
    a branch with a Rule 1 access. Without intercepts `n` was opaque and the
    branch was never entered. Expects 1 `"rule 1"` violation.
  - `fmt_sprintf` — `fmt.Sprintf("hello=%s", "world")` returns `"hello=world"`;
    `len(s) > 5` enters a Rule 1 branch. Without intercepts `len(nil) == 0` and
    the branch was skipped. Expects 1 `"rule 1"` violation.

## [0.10.0] - 2026-02-26

### Fixed

- **`ssa.TypeAssert` nil-safe guard** (`pkg/interpreter/exec.go`): when the concrete type
  of an interface value is unknown (i.e. the value was not boxed via `MakeInterface` in the
  current trace, such as a nil interface variable), the old code returned `ok = true` for
  every case in a type-switch chain. This caused the first typed case to always be taken,
  regardless of the actual dynamic type. The fix distinguishes two contexts:
  - **CommaOk = true** (type-switch chain): unknown concrete type → `ok = false`, allowing
    execution to fall through to the next case or the default branch.
  - **CommaOk = false** (direct type assertion): unknown concrete type → `ok = true`
    (conservative), avoiding false-positive `TypeAssertionError` violations.
  Closes issue #41.

### Added

- **Integration tests** (2 new programs in `pkg/interpreter/testdata/integration/`):
  - `type_switch_dispatch` — 3-case type switch (`*Dog`, `*Cat`, `*Bird`) with all cases
    exercised via known concrete types. Regression guard for correct multi-case dispatch.
    Expects 0 violations.
  - `type_switch_nil` — nil interface dispatched through a type switch whose first case
    body contains an intentional misaligned `unsafe.Pointer` access (Rule 1). Without the
    fix, nil would incorrectly enter the `*Dog` case and trigger a violation. With the fix,
    nil correctly reaches the `default` branch. Expects 0 violations.

## [0.9.0] - 2026-02-26

### Added

- **Call stack traces in violations**: every violation recorded via `recordViolation` is now
  wrapped in a `ViolationWithStack` struct that captures the goroutine ID, spawn site, and
  full call stack (innermost frame first) at the moment of detection. The stack is exposed
  via a `StackTrace() string` method. `pkg/report` extracts the stack via the `stackTracer`
  interface (defined in report to avoid import cycles) and renders it in both text and JSON
  output. `ViolationWithStack.Unwrap()` ensures existing `errors.As` chains and
  `classifyError` type switches continue to work on the underlying error.

- **Goroutine leak detection**: the interpreter now detects goroutines that are permanently
  blocked on a channel receive with no corresponding sender.
  - In the `token.ARROW` (channel receive) case, if the channel has no pending value and is
    not closed, the goroutine is marked `GoroutineBlocked` and execution stops (analogous to
    the existing `Panicked` halt path).
  - `handleChannelSend` now records every send in a new `channelSenders map[ChanID]bool` on
    the `Interpreter` and always sets `ch.hasPending = true` regardless of `TrackRaces`.
  - `checkGoroutineLeaks()` (called from `Finish()`) reports any goroutine with
    `Status == GoroutineBlocked` where no goroutine ever sent on `BlockChanID`. A sender
    existing on the channel — regardless of scheduling order — suppresses the report to
    prevent false positives.
  - New `GoroutineLeakError` type in `pkg/shadow/errors.go` with `GID`, `SpawnSite`,
    `BlockSite`, and `BlockKind` fields. Classified as `"goroutine-leak"` in
    `pkg/report/report.go`.

- **`SpawnSite` on spawned goroutines** (`Goroutine.SpawnSite`): the `ssa.Go` handler now
  records the source location of the spawn on the new goroutine struct, so goroutine leak
  reports include "spawned at" context.

- **`captureStack` helper** (`pkg/interpreter/interpreter.go`): captures the current call
  stack for a goroutine as `[]StackFrame` (innermost first).

- **`Finding.StackTrace` and `Finding.GoroutineID`** (`pkg/report/report.go`): new optional
  fields on `Finding` populated from `ViolationWithStack` when present.

- **Integration tests** (3 new programs in `pkg/interpreter/testdata/integration/`):
  - `callstack_depth` — 4-level deep misalignment violation; validates stack traces are
    captured through multi-level calls. Expects 1 `"rule 1"` violation.
  - `goroutine_leak` — goroutine blocks on receive with no sender; expects 1
    `"goroutine leak"` violation.
  - `no_goroutine_leak` — sender/receiver pair with different scheduling order; validates
    no false positive. Expects 0 violations.

- **Showcase program** (`testdata/showcase/goroutine_leak/main.go`): `worker()` blocks on
  a `results` channel that `main` never sends on. Compiles clean, passes `go vet` and
  `go test -race`; Giri catches the goroutine leak.

### Changed

- `recordViolation(err error)` → `recordViolation(gid int64, err error)`: all 21 call sites
  updated. The `gid` parameter is used to capture the calling goroutine's stack trace.

### Fixed

- Goroutines marked `GoroutineBlocked` now correctly stop execution in both `execBlock` and
  `execFunction` (checked alongside `g.Panicked`).

## [0.8.0] - 2026-02-26

### Added

- **unsafe.Pointer Rule 5 — reflect.Value.Pointer / UnsafeAddr** (closes #12 partially):
  Calls to `reflect.Value.Pointer()` and `reflect.Value.UnsafeAddr()` are now intercepted in
  `execCall`. Both methods return a `uintptr` that must be converted back to `unsafe.Pointer`
  before any GC safepoint. Giri records the pending conversion (tagged with `RuleReflect`)
  so that `CheckGCPoint` fires a violation if any subsequent non-builtin function call is made
  before the conversion. The violation is classified as `"unsafe-pointer-rule 5: reflect pointer
  conversion"` in the report.

- **unsafe.Pointer Rule 6 — reflect.SliceHeader / StringHeader** (closes #12):
  `handleUnsafePointer` now detects when `unsafe.Pointer` is cast to `*reflect.SliceHeader` or
  `*reflect.StringHeader` (the `UnsafeOpFromPointer` case). These deprecated types should be
  replaced with `unsafe.SliceData` / `unsafe.StringData` (Go 1.17+). A `RuleSliceHeader`
  violation is recorded. The check uses `go/types` to inspect the target pointer type — it is
  robust against type aliases and works regardless of how the type name is spelled.

- **`pendingConversion.rule` field** (`pkg/detector/detector.go`): each pending uintptr
  conversion now carries the `UnsafeRule` that governs it (`RuleUintptr` for Rule 2,
  `RuleReflect` for Rule 5). `CheckGCPoint` uses `pending.rule` instead of the previous
  hardcoded `RuleUintptr`, so violations from reflect calls report as Rule 5.

- **`RecordReflectConversion`** method on `UnsafeDetector` and `Registry`: records a pending
  reflect-derived uintptr conversion (same mechanics as `RecordUintptrConversion` but with
  `RuleReflect`).

- **`isReflectHeaderType` helper** (`pkg/interpreter/interpreter.go`): uses `go/types` to
  check whether a `types.Type` is `*reflect.SliceHeader` or `*reflect.StringHeader` without
  string matching.

- **Integration tests** (2 new programs in `pkg/interpreter/testdata/integration/`):
  - `reflect_uintptr` — `v.Pointer()` result held across `noop()` GC safepoint; expects 1
    `"rule 5"` violation.
  - `slice_header` — `unsafe.Pointer(&s)` cast to `*reflect.SliceHeader`; expects 1
    `"rule 6"` violation.

- **Showcase program** (`testdata/showcase/reflect_unsafe/main.go`): `processValue()` calls
  `v.Pointer()`, then `doWork()` (GC safepoint), then converts back to `*int`. Compiles
  clean, passes `go vet` and `go test -race`; Giri catches the Rule 5 violation.

### Fixed

- Closed GitHub issue #12: unsafe.Pointer Rules 5 & 6 were stubs with no detection logic.

## [0.7.1] - 2026-02-26

### Added

- **safearena v0.5.2 dependency**: `GOEXPERIMENT=arenas` is now required to build Giri.
  CI updated to set `GOEXPERIMENT=arenas` in the build matrix environment.

### Performance

- **Arena-backed hot-path allocations** (PR #38): `Run()` now wraps interpretation in
  `safearena.Scoped`, creating a per-run arena freed automatically on return. All
  short-lived structs on the hot execution path are arena-allocated for the lifetime of
  the run, eliminating per-object GC overhead for the millions of allocations a typical
  interpretation produces:

  | Struct | Site | Frequency |
  |---|---|---|
  | `shadow.Pointer` (via `DerivePointer`) | every field/index/slice op | ~1M+/run |
  | `shadow.Pointer` (direct) | `ssa.Alloc`, `ssa.MakeSlice`, `append`, globals init | per alloc op |
  | `SliceValue` | `ssa.MakeSlice`, `ssa.Slice`, `append`, `unsafe.Slice` | per slice op |
  | `Frame` | `pushFrame` | per function call |
  | `Goroutine` | `spawnGoroutine` | per goroutine spawn |

  `newWithArena(fset, config, a)` mirrors `New()` but wires the arena into `shadow.Memory`
  via `WithPointerArena`. The `arenaNew[T]` generic helper returns `*T` directly, leaving
  all external APIs unchanged. `New()` (used in unit tests) retains normal heap allocation
  via the nil-arena fallback in `arenaNew`.

## [0.7.0] - 2026-02-26

### Added

- **Interface values first-class in the interpreter** (#28): `MakeInterface` now wraps the
  concrete value and its type into an `InterfaceValue` struct (was a pass-through that
  discarded type metadata). `ChangeInterface` unwraps and re-wraps while preserving the
  original concrete type. This is the foundation for all interface-aware bug detection.
- **`TypeAssert` proper type checking** (fixes #28): the always-succeeds stub is replaced with
  real dynamic-type checking using `go/types`. For non-comma-ok assertions, a failing assertion
  records a `*shadow.TypeAssertionError` and halts the goroutine (mirroring the runtime panic).
  For comma-ok assertions, the `ok` value now correctly reflects whether the assertion succeeds.
  If the concrete type is unknown (value not boxed via `MakeInterface`), the check is
  conservative and assumes success.
- **Interface method dispatch (invoke calls)**: `execCall` now handles `call.Call.IsInvoke()`
  before the GC-safepoint check. When the receiver is an `InterfaceValue` with a known concrete
  type and `interp.prog != nil`, `ssa.Program.LookupMethod` resolves the concrete method and
  executes it directly. Unknown concrete types fall through as external calls (safe no-op).
- **`TypeAssertionError`** (`pkg/shadow/errors.go`): new structured error type with fields
  `Site`, `ConcreteType`, `AssertedType`, and `GID`. Error message contains `"type-assertion"`
  for consistent substring matching in tests.
- **`TypeAssertionError` report classification** (`pkg/report/report.go`): classified as
  `SeverityError` / category `"type-assertion-failure"` with a hint recommending the comma-ok form.
- **`interp.prog *ssa.Program`** field on `Interpreter`, set in `Run()` after construction.
  Used by invoke dispatch; no public API change.
- **`typeAssertSucceeds` helper** (`pkg/interpreter/exec.go`): checks `types.Identical`,
  `types.Implements` (both T and *T for interface assertions), and `types.AssignableTo`.
- **Integration tests** (3 new programs in `pkg/interpreter/testdata/integration/`):
  - `type_assert_ok` — assertion to correct concrete type; expects 0 violations.
  - `type_assert_fail` — assertion to wrong concrete type (non-comma-ok); expects ≥ 1
    `type-assertion` violation.
  - `iface_dispatch` — `g.Greet()` on a `*English` concrete type via `Greeter` interface;
    expects 0 violations (validates invoke dispatch end-to-end).
- **Showcase program** (`testdata/showcase/type_assert/main.go`): `makeAnimal("cat")` returns
  `*Cat`; `a.(*Dog)` panics. Compiles clean, passes `go vet` and `go test -race`; Giri catches it.

### Fixed

- Closed GitHub issue #28: TypeAssert always returned ok=true.

## [0.6.2] - 2026-02-26

### Fixed

- **Arena packages are now a soft warning, not a hard error.** When a package
  imports `"arena"` but `GOEXPERIMENT=arenas` is not set, `ssautil.LoadProgram`
  and `ssautil.LoadTest` print a warning to stderr, skip the arena packages,
  and continue analyzing everything else. Arena checks produce no findings for
  skipped packages (they noop). Non-arena packages in the same `./...` run are
  unaffected. Giri does not silently enable `GOEXPERIMENT=arenas`; the warning
  includes the exact command to re-run with full arena analysis.

## [0.6.1] - 2026-02-26

### Changed

- Updated README: accurate implementation status, SARIF CI example, arena
  noop-with-warning behavior, expanded detection table (nil deref, closed
  channel, slice OOB), removed stale `GOEXPERIMENT=arenas` prerequisite.

## [0.6.0] - 2026-02-26

### Added

- **SARIF output format** (#15): `giri -format sarif ./... > results.sarif` emits
  SARIF 2.1.0 for GitHub code scanning and IDE integration. Each finding is mapped
  to a `ruleId` (e.g. `giri/out-of-bounds`), a level (`error`/`warning`/`note`),
  a human-readable message, and a source location with file path and line number.
  Rules are deduplicated across findings and sorted for stable output.
- **`NilPointerDerefError` report classification**: nil-deref violations are now
  correctly classified as category `nil-pointer-deref` instead of falling through
  to the generic `other` category.
- **GitHub Actions CI workflow** (`.github/workflows/ci.yml`): builds and runs
  `go test -race ./...` on Go 1.23 and 1.24 on every push and PR.
- **GitHub Actions SARIF upload** (`.github/workflows/sarif.yml`): runs giri on
  its own codebase on every push to `main` and uploads results to GitHub code
  scanning via `github/codeql-action/upload-sarif`.
- **Report package tests** (`pkg/report/report_test.go`): five tests covering
  exit codes, nil-deref classification, JSON/text/SARIF writers, and location
  parsing.

### Fixed

- Exit codes were already wired (`os.Exit(rpt.ExitCode())` in `cmd/giri/main.go`);
  this release confirms the behaviour: exit 0 = clean, exit 1 = violations found,
  exit 2 = load/internal error.

## [0.5.1] - 2026-02-25

## [0.5.0] - 2026-02-25

### Added

- **Nil pointer dereference detection** (#36): `handleLoad`/`handleStore` now
  fire `*shadow.NilPointerDerefError` when the address has both `Raw == nil`
  and `Provenance == nil`, indicating a true nil dereference. The goroutine is
  halted via `Panicked = true`.
- **Send-on-closed-channel detection** (#31): `chanEntry` gains `closed bool`,
  `hasPending bool`, and `pendingVal Value` fields. `handleChannelSend` fires
  a violation when the channel is already closed; `handleChannelClose` detects
  double-close. A new `close` builtin case dispatches to `handleChannelClose`.
- **Slice re-slice bounds validation** (#32): the `*ssa.Slice` handler now
  validates `0 ≤ low ≤ high ≤ cap(s)` and records `*shadow.OutOfBoundsError`
  on violation. Also handles the `*[N]T → []T` lowering that Go SSA applies to
  `make([]T, n)` with constant `n` (Alloc + Slice rather than MakeSlice).
- **Goroutine spawn happens-before** (#29): the `ssa.Go` handler ticks the
  parent goroutine's vector clock before spawning and copies the resulting
  snapshot into the child's initial clock. This correctly models the Go memory
  model's spawn edge, eliminating false race reports between parent writes and
  child reads.
- **`ssa.Select` minimal implementation** (#30): selects the first ready
  channel (non-closed sender or pending/closed receiver); falls through to
  default case if non-blocking and no channel is ready.
- **`append` proper implementation** (#26): in-place growth returns same
  backing with updated length; reallocation allocates new heap backing with 2×
  cap and copies provenance.
- **`copy` proper implementation** (#27): computes `n = min(dst.Len, src.Len)`,
  triggers a `handleStore` for bounds/race checking, and returns `n`.
- **`recover()` semantics** (#34): if the goroutine has `Panicked=true` and a
  non-nil `PanicValue`, recover returns the panic value and clears the panic
  state so execution can continue.
- **sync.Mutex / sync.WaitGroup modeling** (#33): calls to `sync` package
  methods are intercepted in `execCall` and dispatched to `handleSyncCall`.
  `Lock`/`RLock`/`Wait` merge the last-unlock snapshot into the current clock
  (establishing HB); `Unlock`/`RUnlock`/`Done` tick the clock and store a
  snapshot.
- **Arena pointer global escape detection** (#35): `handleStore` checks when
  a value with `AllocArena` provenance is stored into an `AllocGlobal`
  destination and records `*shadow.EscapedPointerError{EscapeKind: "global"}`.
- **New integration tests** (#29, #31, #32, #36):
  - `spawn_hb` — parent writes `*x`, spawns goroutine that reads `*x`; expects
    0 violations (spawn establishes HB).
  - `nil_deref` — reads from a nil `*int`; expects ≥ 1 `nil pointer` violation.
  - `close_panic` — closes then sends on a channel; expects ≥ 1 `closed
    channel` violation.
  - `slice_oob` — reslices `make([]int,4)` to `[0:100]`; expects ≥ 1
    `out-of-bounds` violation.

### Fixed

- **`data_race` test updated**: the previous test used a parent→child access
  pattern which is NOT a race per the Go memory model (spawn establishes HB).
  Updated to use two sibling goroutines that both write `*x` without any
  synchronisation — a genuine data race.
- **`ssa.Slice` handles Alloc-lowered make**: `make([]T, n)` with constant `n`
  is lowered by Go SSA to `Alloc(*[n]T) + Slice`, not `MakeSlice`. The Slice
  handler now recognises a `*shadow.Pointer` base (from Alloc) and derives the
  initial `SliceValue{Len:n, Cap:n}` from the array type before applying bounds
  checks.

## [0.4.0] - 2026-02-26

### Added

- **Vector clock race detection** (#23): `Detector.CheckAccess` now accepts a
  `clock map[int64]uint64` parameter. `RaceDetector.CheckAccess` stores a
  snapshot of the goroutine's vector clock with every access and uses
  `happensBefore(a, b map[int64]uint64) bool` to determine whether two
  conflicting accesses are causally ordered. Races are reported only when
  neither clock precedes the other — eliminating false positives from
  channel-synchronized programs.
- **Global variable tracking** (#21): `Run()` now iterates
  `prog.SSA.AllPackages()` and allocates `AllocGlobal` shadow memory for every
  `*ssa.Global` member before `main` executes. `resolveValue` looks globals up
  in an `Interpreter.globals map[*ssa.Global]Value` table instead of returning
  a raw string, enabling proper load/store tracking for package-level variables.
- **Map and array support** (#22): `ssa.Lookup` performs real map key lookup
  on `map[interface{}]Value` (with `CommaOk` support); `ssa.MapUpdate` mutates
  the map in place. `ssa.Index` handles slice, string, and map-as-array forms.
  `rangeIter` gains a `mapKeys []interface{}` field and a map case in
  `advance()`. New helpers `toMapKey` and `valueFromMapKey` convert between
  interpreter `Value` and comparable map keys.
- **Integration tests** (#24): three new programs and test table entries:
  - `data_race` — main writes to `*x`; goroutine writes to same `*x` without
    sync; expects ≥ 1 `data race` violation.
  - `no_race_chan` — writer goroutine writes then signals on channel; reader
    goroutine receives then reads; expects 0 violations (channel HB).
  - `uninit_read` — reads from `new(int)` before any write with
    `Config.TrackInit=true`; expects ≥ 1 `uninitialized` violation.

### Changed

- `Detector.CheckAccess` interface signature extended with
  `clock map[int64]uint64`; all four implementations (Arena, Bounds, Unsafe,
  Race) and `Registry.CheckAccess` updated accordingly.
- `handleLoad` and `handleStore` now pass `g.VClock.Clocks` to
  `registry.CheckAccess` so the race detector receives live clock data.

## [0.3.1] - 2026-02-26

### Fixed

- **MaxSteps enforcement** (#17): `Config.MaxSteps` is now checked in
  `execInstruction`. A per-interpreter `steps uint64` counter increments on
  every instruction; exceeding the cap records an `"execution limit exceeded"`
  violation and sets `Goroutine.Panicked = true` to halt further execution.
  Previously the field was wired from the CLI but never read.
- **Phi node fallback picks wrong value** (#18): The fallback path (taken when
  no predecessor block matches `frame.PrevBlock`) previously skipped any edge
  whose resolved value had `Raw == nil`, meaning loop variables initialised to
  `0`, `false`, or a nil pointer were silently discarded. The fallback now
  unconditionally takes `inst.Edges[0]`, which is correct by SSA edge ordering.
- **Closure FreeVars not bound** (#19): `execFunction` now binds `fn.FreeVars`
  from args appended after the regular parameter slice. `ssa.MakeClosure` is
  handled to capture binding values into a new `ClosureValue` struct. Both
  `execCall` and `ssa.Go` detect `ClosureValue` callee values and append the
  captured free vars when dispatching.
- **ssa.Panic stub** (#20): `ssa.Panic` now runs deferred calls across the
  entire goroutine stack in LIFO order (innermost frame first), clears the
  stack, and sets `Goroutine.Panicked = true`. This prevents false arena-leak
  reports when a `defer a.Free()` is registered above the panic site. The
  `Goroutine.Panicked` flag is also checked at the top of `execBlock` and in
  the `execFunction` block loop so that execution stops cleanly after a halt.

### Added

- `Goroutine.Panicked bool` field — shared halt signal for panic and step-limit.
- `ClosureValue` struct (`Fn *ssa.Function`, `FreeVars []Value`) in interpreter.go.
- `steps uint64` field on `Interpreter`.
- Integration tests for each fix: `loop` (Phi zero), `closure` (FreeVars),
  `maxsteps` (step limit), `panic_defers` (panic stack unwind).

## [0.3.0] - 2026-02-25

### Added

- **unsafe.Pointer Rule 1 – alignment check** (`pkg/interpreter/interpreter.go`):
  `handleUnsafePointer` now accepts `targetType types.Type` and `valueID string`.
  For `UnsafeOpFromPointer` (unsafe.Pointer → *T) it verifies that the pointer's
  byte offset is divisible by `types.Sizes.Alignof(T)`, firing `RuleConversion`
  when misaligned.
- **unsafe.Pointer Rule 2 – uintptr GC point tracking** (`pkg/detector/detector.go`,
  `pkg/interpreter/exec.go`):
  - `UnsafeDetector.ClearAllUintptrConversions()` clears all pending uintptr entries.
  - `Registry` gains `unsafeDetector *UnsafeDetector` field extracted in `NewRegistry`;
    four delegation methods added: `RecordUintptrConversion`, `ClearUintptrConversion`,
    `ClearAllUintptrConversions`, `CheckGCPoint`.
  - `execCall` calls `registry.CheckGCPoint(site)` before every non-builtin function
    call; any live uintptr at that point fires `RuleUintptr`.
  - `UnsafeOpToUintptr` in `handleUnsafePointer` now calls `registry.RecordUintptrConversion`.
  - `UnsafeOpArithmetic` (uintptr → unsafe.Pointer) now calls `registry.ClearAllUintptrConversions`.
- **Channel happens-before tracking** (`pkg/interpreter/interpreter.go`, `exec.go`):
  Added `ChanID` type, `chanEntry` struct (stores sender GID + clock snapshot),
  `channels map[ChanID]*chanEntry`, `nextChanID atomic.Uint64`, and `createChannel()`.
  `MakeChan` now allocates a real `ChanID`; `Send` extracts the `ChanID` and records
  the sender's vector clock via `handleChannelSend`; channel receive (`<-ch`) extracts
  the `ChanID` and merges the sender's clock via `handleChannelRecv`.
- **Integration tests** (3 new programs in `testdata/integration/`):
  - `uintptr_gc` — uintptr survives to a function call (GC point); expects 1 Rule 2
    violation.
  - `safe_uintptr` — uintptr immediately converted back, no intervening call; expects
    0 violations.
  - `misaligned_ptr` — unsafe.Add offset 1, convert to `*uint32`; expects 1 Rule 1
    violation.

### Changed

- `handleUnsafePointer` signature extended with `targetType types.Type, valueID string`.
- `handleChannelSend` signature changed from `(gid, val, site)` to `(gid, chanID, val, site)`.
- `handleChannelRecv` signature changed from `(gid, senderGID, site)` to `(gid, chanID, site)`.
- `Registry.NewRegistry` now scans the detector list to extract the `*UnsafeDetector`
  for direct delegation (no public API change).

## [0.2.0] - 2026-02-25

### Added

- **Constant value representation** (`pkg/interpreter/exec.go`): `constToValue` now uses
  `go/constant` to extract typed values (`int64`, `float64`, `bool`, `string`) instead of
  returning raw strings, enabling correct arithmetic.
- **Accurate type sizing** (`pkg/interpreter/interpreter.go`): Replaced `estimateTypeSize`
  string-matching heuristic with `go/types.Sizes` for the target platform (`gc`/`GOARCH`).
  Field offsets in `FieldAddr` and element sizes in `IndexAddr` now use `types.Sizes.Offsetsof`.
- **Phi node predecessor tracking**: Added `PrevBlock *ssa.BasicBlock` to `Frame`; `execFunction`
  tracks the previous block so Phi edges are resolved correctly rather than taking the first
  available value.
- **New SSA instruction coverage** (15 new cases): `BinOp`, `Extract`, `Field`, `Index`,
  `Lookup`, `MapUpdate`, `MakeSlice`, `Slice`, `MakeMap`, `MakeChan`, `Range`, `Next`,
  `TypeAssert`, `ChangeInterface`, `MultiConvert`, `SliceToArrayPointer`, `Select`.
- **Builtin interception** (`execBuiltin`): Handles `unsafe.Add`, `len`, `cap`, `append`,
  `copy`, `delete`, `close`, `recover`, `unsafe.Slice` by dispatching on `*ssa.Builtin`
  before the normal call path.
- **Detector registry wiring**: `New()` builds a `*detector.Registry` from config flags and
  `handleLoad`/`handleStore` now call `registry.CheckAccess` after the base `CheckAccess`.
  `Finish()` calls `registry.Finalize` (replaces manual arena leak loop).
- **Scheduler wiring**: `New()` instantiates the configured `scheduler.Scheduler`;
  `ssa.Go` now enqueues tasks in `runQueue`; `Run()` drains the queue via the scheduler
  after `main` completes.
- **Verbose access logging fix** (`pkg/shadow/memory.go`): `AccessLog` is initialized in
  `Allocate` when verbose mode is enabled; `CheckAccess` records each access using a
  per-allocation `logMu` mutex (no global write-lock upgrade needed).
- **Unit tests**: `pkg/shadow/memory_test.go`, `pkg/shadow/errors_test.go`,
  `pkg/detector/detector_test.go`, `pkg/scheduler/scheduler_test.go`.
- **Integration tests**: `pkg/interpreter/interpreter_integration_test.go` with four
  programs in `pkg/interpreter/testdata/integration/`: `safe_alloc` (0 violations),
  `unsafe_oob` (1 violation, unsafe.Pointer OOB), `binop` (0 violations), `multi_return`
  (0 violations).

### Changed

- `Frame` struct gains `PrevBlock *ssa.BasicBlock` field.
- `Interpreter` struct gains `sizes`, `registry`, `sched`, `runQueue` fields.
- `Return` instruction stores multi-value results as `[]Value` tuple (was single value only).
- `UnOp` handler uses `token.XXX` constants and handles `token.ARROW` (channel receive).
- `handleAlloc` uses a conservative 8-byte default (callers with concrete types use
  `Memory.Allocate` + `typeSizeOf` directly).

### Fixed

- `unsafe.Add` was silently ignored (treated as `<dynamic>` call); now correctly intercepted
  via `*ssa.Builtin` dispatch and triggers `UnsafePointerViolation` on out-of-bounds arithmetic.

## [0.1.0] - 2026-02-25

### Added

- Shadow memory system (`pkg/shadow`) with allocation tracking, pointer provenance,
  arena lifecycle management, and optional per-byte initialization tracking.
- Structured error types for all violation categories: use-after-free, double-free,
  arena double-free, out-of-bounds, uninitialized read, unsafe.Pointer violations (all
  six rules defined), arena pointer escape, and data race.
- SSA interpreter (`pkg/interpreter`) with support for core instruction types: `Alloc`,
  `Store`, `UnOp`, `FieldAddr`, `IndexAddr`, `Jump`, `If`, `Return`, `Panic`, `Call`,
  `Defer`, `Go`, `Send`, `Convert`, `ChangeType`, `MakeInterface`, `Phi`, `DebugRef`.
- Deferred call handling with correct LIFO execution order, including `arena.Free()`
  interception.
- Vector clock infrastructure for happens-before tracking in the interpreter.
- Composable detector framework (`pkg/detector`) with `ArenaDetector`, `BoundsDetector`,
  `UnsafeDetector`, and `RaceDetector`.
- Goroutine scheduling strategies (`pkg/scheduler`): `RoundRobin`, `Random` (seeded),
  and `PCT` (Probabilistic Concurrency Testing, Burckhardt et al. 2010).
- Report package (`pkg/report`) with structured `Finding` types, text and JSON output
  formats, severity levels, and CI-friendly exit codes.
- SSA loader utility (`internal/ssautil`) for loading Go packages into SSA form,
  including test package support.
- CLI entry point (`cmd/giri`) with flags for detector selection, scheduling strategy,
  output format, and execution limits.
- `BugDepth` configuration field wired from `-depth` CLI flag to PCT scheduler.
- `testdata/ub_patterns.go` documenting the UB patterns Giri is designed to detect.
- `CHANGELOG.md` in keepachangelog format.

### Fixed

- Removed seven root-level `.go` files that declared conflicting package names,
  causing `go build` to fail with "found packages X and Y" errors.
- Moved `report.go` from the root directory to `pkg/report/report.go`, resolving
  the missing import for `github.com/scttfrdmn/giri/pkg/report`.
- Generated `go.sum` via `go mod tidy`.

[Unreleased]: https://github.com/scttfrdmn/giri/compare/v0.9.0...HEAD
[0.9.0]: https://github.com/scttfrdmn/giri/compare/v0.8.0...v0.9.0
[0.8.0]: https://github.com/scttfrdmn/giri/compare/v0.7.1...v0.8.0
[0.7.1]: https://github.com/scttfrdmn/giri/compare/v0.7.0...v0.7.1
[0.7.0]: https://github.com/scttfrdmn/giri/compare/v0.6.2...v0.7.0
[0.6.2]: https://github.com/scttfrdmn/giri/compare/v0.6.1...v0.6.2
[0.6.1]: https://github.com/scttfrdmn/giri/compare/v0.6.0...v0.6.1
[0.6.0]: https://github.com/scttfrdmn/giri/compare/v0.5.1...v0.6.0
[0.5.1]: https://github.com/scttfrdmn/giri/compare/v0.5.0...v0.5.1
[0.5.0]: https://github.com/scttfrdmn/giri/compare/v0.4.0...v0.5.0
[0.4.0]: https://github.com/scttfrdmn/giri/compare/v0.3.1...v0.4.0
[0.3.1]: https://github.com/scttfrdmn/giri/compare/v0.3.0...v0.3.1
[0.3.0]: https://github.com/scttfrdmn/giri/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/scttfrdmn/giri/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/scttfrdmn/giri/releases/tag/v0.1.0
