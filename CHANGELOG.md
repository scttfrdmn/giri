# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.83.0] - 2026-03-06

### Added

- **`net/http` additions** (6 new intercepts):
  - `http.FS(fsys fs.FS) FileSystem` → opaque (Go 1.16)
  - `http.NewFileTransport(fs FileSystem) RoundTripper` → opaque
  - `http.NewFileTransportFS(fsys fs.FS) RoundTripper` → opaque (Go 1.22)
  - `http.AllowQuerySemicolons(h Handler) Handler` → opaque (Go 1.17)
  - `http.TimeoutHandler(h Handler, dt, msg) Handler` → opaque
  - `http.MaxBytesHandler(h Handler, n int64) Handler` → opaque (Go 1.20)
- **`testing.Benchmark`** (1 new intercept): probes the callback function with
  an opaque `*testing.B` sentinel; returns opaque `BenchmarkResult`.
  Note: benchmark callbacks must not access `b.N` directly — use method calls only.
- **`log/slog.NewLogLogger`** (1 new intercept): `slog.NewLogLogger(h, level) *log.Logger` → opaque.
- Integration test `http_testing_slog_complete`: exercises all 8 new intercepts; 0 violations.
- 1 new integration test (252 integration + 14 showcase = 266 total).

## [0.82.0] - 2026-03-06

### Added

- **`crypto/rand.Text`** (Go 1.24): `crypto/rand.Text() string` → `""`.
- **`bufio` split functions** (4 new intercepts):
  - `bufio.ScanLines`, `bufio.ScanWords`, `bufio.ScanBytes`, `bufio.ScanRunes`
    `(data []byte, atEOF bool) (advance int, token []byte, err error)` → `(0, nil, nil)` conservative.
- **`io` method completions** (5 new intercepts, covering `SectionReader`, `OffsetWriter`, `PipeReader`, `PipeWriter`):
  - `Read(p []byte) (int, error)` → `(0, nil)`
  - `ReadAt(p []byte, off int64) (int, error)` → `(0, nil)`
  - `Write(p []byte) (int, error)` → `(0, nil)`
  - `WriteAt(p []byte, off int64) (int, error)` → `(0, nil)`
  - `Seek(offset int64, whence int) (int64, error)` → `(0, nil)`
  - `Close() error` → `nil`
- **`net/http` additions** (8 new intercepts):
  - `http.FileServer(root FileSystem) Handler` → opaque
  - `http.NewResponseController(rw ResponseWriter) *ResponseController` → opaque
  - `(*ResponseController).Flush() error` → nil
  - `(*ResponseController).Hijack() (net.Conn, *bufio.ReadWriter, error)` → `(opaque, opaque, nil)`
  - `(*ResponseController).SetReadDeadline/SetWriteDeadline(time.Time) error` → nil
  - `(*Server).Shutdown(ctx) error` → nil
  - `(*Server).Close() error` → nil
  - `(*Server).Serve(net.Listener) error` → nil
- Integration test `rand_bufio_io_http_complete`: exercises all new intercepts; 0 violations.
- 1 new integration test (251 integration + 14 showcase = 265 total).

## [0.81.0] - 2026-03-06

### Added

- **`net` completions** (9 new intercepts):
  - `net.Listener.Accept() (Conn, error)` → `(opaque, nil)`
  - `net.Listener.Addr() Addr` → opaque
  - `net.DialTCP`, `net.DialUDP`, `net.DialUnix` → `(opaque, nil)` — typed connection constructors
  - `net.ListenTCP`, `net.ListenUDP`, `net.ListenUnix` → `(opaque, nil)` — typed listener constructors
  - `(*net.Dialer).DialContext(ctx, network, address)` → `(opaque, nil)`
- **`encoding/json.Number` methods** (2 new intercepts):
  - `(json.Number).Float64() (float64, error)` → `(0.0, nil)`
  - `(json.Number).Int64() (int64, error)` → `(0, nil)`
- **`runtime.Func` methods** (3 new intercepts + 1 fix):
  - `(*runtime.Func).Name() string` → `""`
  - `(*runtime.Func).Entry() uintptr` → `0`
  - `(*runtime.Func).FileLine(pc uintptr) (string, int)` → `("", 0)`
  - `runtime.FuncForPC` changed from `nil` → `opaque` so downstream method calls are exercised
- Integration test `net_json_runtime_complete`: exercises all 14 new intercepts; 0 violations.
- 1 new integration test (250 integration + 14 showcase = 264 total).

## [0.80.0] - 2026-03-06

### Added

- **`os.Root` completions** (Go 1.24): `os.OpenRoot(name) (*Root, error)` → `(opaque, nil)`;
  `(*Root).FS() fs.FS` → opaque. All other `*Root` methods (`Open`, `Create`, `OpenFile`, `Stat`,
  `Lstat`, `Mkdir`, `MkdirAll`, `Remove`, `RemoveAll`, `Rename`, `Close`, `Name`, `ReadFile`,
  `WriteFile`, `ReadDir`, `Link`, `Symlink`, `Readlink`, `Chmod`, `Chown`, `Lchown`, `Chtimes`)
  were already covered by existing `os` handler cases. 2 new intercepts.
- **`slices.Chunk`** (Go 1.23): `Chunk(s []E, n int) iter.Seq[[]E]` → opaque iterator.
  (Note: the function is `Chunk`, singular — not `Chunks`.) 1 new intercept.
- **`net/url.Userinfo` methods**: `(*Userinfo).Username() string` → `""`;
  `(*Userinfo).Password() (string, bool)` → `("", false)`. 2 new intercepts.
- Integration test `os_root_slices_maps_url_complete`: exercises all 5 new intercepts; 0 violations.
- 1 new integration test (249 integration + 14 showcase = 263 total).

## [0.79.0] - 2026-03-06

### Added

- **`fmt` completions** (issue #221): `Append(b []byte, a ...any) []byte` → `[]byte{}`
  (Go 1.19); `Appendf` → `[]byte{}`; `Appendln` → `[]byte{}`. 3 new intercepts.
- **`strings` completions** (issue #221, Go 1.24): `SplitSeq(s, sep string) iter.Seq[string]` →
  opaque; `FieldsSeq(s string) iter.Seq[string]` → opaque; `Lines(s string) iter.Seq[string]`
  → opaque. 3 new intercepts.
- **`slices` completions** (issue #221, Go 1.23/1.24): `Sorted[E cmp.Ordered](seq iter.Seq[E]) []E`
  → `[]Value{}`; `SortedFunc` → `[]Value{}`; `SortedStableFunc` → `[]Value{}`. 3 new intercepts.
- Integration test `fmt_strings_slices_complete`: exercises all 9 new intercepts; 0 violations.
- **3 new showcase programs** (issue #221):
  - `map_race`: concurrent unsynchronized writes to shared map — `data race` violation.
  - `context_cancel_leak`: `context.WithCancel` cancel func never called — `context-cancel-leak` violation.
  - `div_zero`: integer division by zero on untested code path — `division by zero` violation.
- 4 new tests (248 integration + 14 showcase = 262 total). Closes #221.

## [0.78.0] - 2026-03-06

### Added

- **`math` completions** (issue #220): `FMA(x, y, z float64) float64` → concrete passthrough
  (Go 1.14); `RoundToEven(x float64) float64` → concrete passthrough; `Erfcinv(x float64) float64`
  → concrete passthrough. 3 new intercepts.
  Note: `math.Erfinv` intentionally remains unmodeled — it serves as the `TestUnmodeledCallsReport` sentinel.
- **`bytes.Buffer` completions** (issue #220, Go 1.21): `Available() int` → 0;
  `AvailableBuffer() []byte` → nil; `Peek(n int) ([]byte, error)` → (nil, nil). 3 new intercepts.
- **`unicode` completions** (issue #220): `IsSymbol(r rune) bool` → concrete passthrough;
  `IsOneOf(ranges []*RangeTable, r rune) bool` → conservative `true`;
  `To(_case int, r rune) rune` → concrete passthrough. 3 new intercepts.
- Integration test `math_bytes_unicode_complete`: exercises all 9 new intercepts; 0 violations.
- 1 new integration test (247 integration + 11 showcase = 258 total).

## [0.77.0] - 2026-03-06

### Added

- **`encoding/binary` completions** (issue #219): `Encode` → `(int, error)` (Go 1.23);
  `Decode` → `(int, error)` (Go 1.23); `Append` → `([]byte, error)` (Go 1.23). 3 new intercepts.
- **`os` completions** (issue #219): `NewSyscallError` → nil; `CopyFS` (Go 1.23) → nil;
  `Expand` → `""`. 3 new intercepts.
- Integration test `binary_os_complete`: exercises all 6 new intercepts; 0 violations.
- **`CLAUDE.md` overhaul**: updated from stale "Phase 1 / SSA walker scaffolded" to reflect
  current v0.77.0 maturity; added Release Checklist (test count commands, CHANGELOG format,
  README tag verification), integration test conventions, and common gotchas.
- 1 new integration test (246 integration + 11 showcase = 257 total).

## [0.76.0] - 2026-03-06

### Added

- **`encoding/binary` completions** (issue #218): `AppendUint16`/`AppendUint32`/`AppendUint64`
  (Go 1.21 `AppendByteOrder` interface methods) → return dst slice; `AppendUvarint`/`AppendVarint`
  (Go 1.22, package-level) → return dst slice; `ReadVarint` → `(int64, error)`;
  `ReadUvarint` → `(uint64, error)`. 7 new intercepts.
- **`database/sql` completions** (issue #218): `Drivers` → `[]string{}`. 1 new intercept.
- **`math/big` completions** (issue #218): `MulRange` → opaque `*Int`; `Binomial` → opaque `*Int`.
  2 new intercepts.
- Integration test `binary_sql_bigint_complete`: exercises all new intercepts; 0 violations.
- README: update GitHub Action tag `@v0.17.0` → `@v0.76.0`; update stdlib coverage
  "60+ packages" → "170+ packages".
- 1 new integration test (245 integration + 11 showcase = 256 total).

## [0.75.0] - 2026-03-07

### Added

- **`reflect` package completions** (issue #217): `NewAt` → opaque; `Complex` → `complex128(0)`;
  `SetComplex` → noop; `CanComplex` (Go 1.20) → `bool`; `OverflowInt`/`OverflowUint`/`OverflowFloat`/
  `OverflowComplex` → `false`; `Pointer` → `uintptr`; `UnsafePointer` (Go 1.18) → opaque;
  `UnsafeAddr` → `uintptr`; `Select` → `(int, Value, bool)` 3-tuple; `Swapper` → opaque;
  `SetZero` (Go 1.20) → noop; `Equal` (Go 1.20) → `bool`. 13 new intercepts.
  Note: `Pointer`/`UnsafePointer`/`UnsafeAddr` correctly trigger Rule 5 when results survive GC
  points — this is expected correct behavior.
- **`log` package completions** (issue #217): `Output` → noop. 1 new intercept.
- Integration test `reflect_complete`: exercises all new reflect/log intercepts; 0 violations.
- 1 new integration test (244 integration + 11 showcase = 255 total).

## [0.74.0] - 2026-03-06

### Added

- **`net` package completions** (issue #216): `LookupAddr` → `([]string, error)`; `LookupSRV` →
  `(cname, []*SRV, error)`; `ParseMAC` → `(HardwareAddr, error)`; `Interfaces`/`InterfaceAddrs` →
  `([]T, error)`; `InterfaceByName` → `(*Interface, error)`; `net.IP` methods: `String`, `Equal`,
  `IsLoopback`, `IsGlobalUnicast`, `IsMulticast`, `IsLinkLocalUnicast`, `IsLinkLocalMulticast`,
  `IsInterfaceLocalMulticast`, `IsUnspecified`, `IsPrivate`, `To4`, `To16`, `Mask`; `IPNet.Contains`;
  `Addr.Network`; Conn interface: `Read`, `Write`, `Close`, `LocalAddr`, `RemoteAddr`,
  `SetDeadline`, `SetReadDeadline`, `SetWriteDeadline`. 27 new intercepts.
- **`net/url` package completions** (issue #216): `(*URL).Redacted()` → `string` (Go 1.15). 1 new.
- **`flag` package completions** (issue #216): `BoolFunc` (Go 1.20) → noop. 1 new.
- Integration test `net_url_flag_complete`: exercises all new intercepts; 0 violations.
- 1 new integration test (243 integration + 11 showcase = 254 total).

## [0.73.0] - 2026-03-06

### Added

- **`sort` completions** (issue #215): `SearchStrings`, `SearchInts`, `SearchFloat64s` → `int`;
  `StringsAreSorted`, `IntsAreSorted`, `Float64sAreSorted` (Go 1.21) → `bool`. 6 new intercepts.
- **`regexp` completions** (issue #215): `CompilePOSIX` → `(*Regexp, error)`; `MustCompilePOSIX` →
  `*Regexp`; `FindSubmatchIndex`/`FindAllSubmatchIndex` → `[]int`; `Expand`/`ExpandString` →
  `[]byte`. 6 new intercepts.
- **`io` completions** (issue #215): `NewOffsetWriter` (Go 1.20) → opaque `*OffsetWriter`;
  `(*SectionReader).Size()` → `int64`. 2 new intercepts.
- **`path/filepath` completions** (issue #215): `IsLocal` (Go 1.22) → concrete `bool`. 1 new intercept.
- Integration test `sort_regexp_io_complete`: exercises all new intercepts; 0 violations.
- 1 new integration test (242 integration + 11 showcase = 253 total).

## [0.72.0] - 2026-03-06

### Added

- **`time` package completions** (issue #214): `Compare` (Go 1.20) → `int`; `AppendFormat` (Go 1.20)
  → `[]byte`; `Clock` → `(hour, min, sec int)` 3-tuple; `Location` → opaque `*time.Location`;
  `IsDST` (Go 1.17) → `bool`; `ZoneBounds` (Go 1.19) → `(start, end time.Time)` 2-tuple;
  `AddDate` → opaque `time.Time`; `Duration.Abs` (Go 1.19) → `int64`; `LoadLocation` →
  `(*time.Location, error)`; `FixedZone` → opaque `*time.Location`. 10 new intercepts.
- Integration test `time_complete`: exercises all new `time` intercepts; 0 violations.
- 1 new integration test (241 integration + 11 showcase = 252 total).

## [0.71.0] - 2026-03-06

### Added

- **`strings` package completions** (issue #213): `Clone` → passthrough; `CutPrefix`/`CutSuffix`
  (Go 1.20) → `(string, bool)` tuples; `ContainsFunc` (Go 1.21) → pessimistic `true`;
  `FieldsFunc` → single-element `[]string`; `IndexFunc`/`LastIndexFunc` → `int` (pessimistic 0);
  `LastIndexAny`/`LastIndexByte` → concrete or 0; `SplitAfterN` → concrete or single slice;
  `Title` (deprecated) → concrete; `ToValidUTF8` → concrete; `TrimFunc`/`TrimLeftFunc`/`TrimRightFunc`
  → input passthrough. 15 new string intercepts.

- **`bytes` package completions** (issue #213): `ContainsFunc` → `true`; `FieldsFunc` → `[][]byte`;
  `IndexFunc`/`LastIndexFunc` → `int`; `LastIndexByte` → `int`; `Runes` → `[]rune`; `ToValidUTF8`
  → `[]byte`. 7 new bytes intercepts.

- **`strconv` package completions** (issue #213): `QuoteRune`/`QuoteRuneToASCII`/`QuoteRuneToGraphic`
  → concrete `string`; `QuoteToASCII`/`QuoteToGraphic` → concrete `string`; `QuotedPrefix` (Go 1.17)
  → `(string, error)`; `CanBackquote` → concrete `bool`; `IsPrint`/`IsGraphic` → concrete `bool`;
  `ParseComplex` → `(complex128, error)`; `FormatComplex` → `string`; `UnquoteChar` → 4-tuple;
  `AppendQuoteRune`/`AppendQuoteRuneToASCII`/`AppendQuoteRuneToGraphic`/`AppendQuoteToASCII`/
  `AppendQuoteToGraphic` added to the existing `AppendXxx` case. 13 new strconv intercepts.

- **Integration tests**: `strings_complete` (exercises 15 new string functions);
  `strconv_complete` (exercises 13 new strconv functions).

- Test count: 251 integration + 12 showcase = 263 total.

## [0.70.0] - 2026-03-06

### Added

- **`net/http` Header method disambiguation and additions** (issue #212):
  fix `case "Get"` split: `len(args)==1` → package-level `http.Get(url)` returns
  `(*Response, error)` tuple; `len(args)>=2` with nil receiver → `(http.Header).Get(key)`
  returns `string`; otherwise `(*http.Client).Get(url)` returns tuple.
  New cases: `Set`/`Add`/`Del` → noop; `Values` → `[]string{"value"}`;
  `Context` → opaque (for `(*http.Request).Context()`);
  `WriteHeader` → noop (for `http.ResponseWriter.WriteHeader`);
  `CanonicalHeaderKey` → string passthrough;
  `DetectContentType` → `"application/octet-stream"`;
  `MaxBytesReader` → opaque.

- **6 new `encoding/json` Decoder/Encoder option methods** (issue #212):
  `UseNumber` → noop; `DisallowUnknownFields` → noop;
  `InputOffset` → `int64(0)`; `Buffered` → opaque;
  `SetIndent` → noop; `SetEscapeHTML` → noop.

- **Integration tests**: `http_header` (exercises Header.Set/Add/Del/Values,
  Request.Context, MaxBytesReader, CanonicalHeaderKey, DetectContentType);
  `json_options` (exercises UseNumber, DisallowUnknownFields, InputOffset,
  Buffered, SetIndent, SetEscapeHTML).

- Test count: 249 integration + 12 showcase = 261 total.

### Fixed

- `(http.Header).Get(key)` no longer returns a `(*Response, error)` tuple when
  the receiver is nil (opaque field access). Closes #212.

## [0.69.0] - 2026-03-06

### Added

- **20 new `os.*` intercepts** (issue #211): `Lstat` → `(opaque,nil)`;
  `TempDir` → `"/tmp"`; `Hostname` → `("localhost",nil)`;
  `Getpid`/`Getuid`/`Getgid`/`Geteuid`/`Getegid` → real int values;
  `Getgroups` → `([],nil)`; `IsNotExist`/`IsExist`/`IsPermission`/`IsTimeout`
  → `false`; `ExpandEnv` → `""`; `Environ` → `[]`; `Clearenv` → noop;
  `Executable` → `("/tmp/giri-test",nil)`;
  `UserHomeDir`/`UserCacheDir`/`UserConfigDir` → `("/tmp",nil)`;
  `Pipe` → `(opaque,opaque,nil)`;
  `Readlink` → `("",nil)`; `Link`/`Symlink`/`Chtimes` → noop;
  `SameFile` → `false`; `Exit` → noop.

- **8 new `sync/atomic` method cases** (issue #211): `Load`, `Store`, `Add`,
  `Swap`, `CompareAndSwap`, `And`, `Or` added to `handleAtomicCall` for
  the Go 1.19+ concrete atomic struct types (`atomic.Int32`, `Int64`, `Uint32`,
  `Uint64`, `Uintptr`, `Bool`). Method names dispatch as bare names
  (e.g., `"Load"`) because `Function.Name()` strips the receiver prefix.
  Uses the same `valueStore` AllocID mechanism as the existing package-level
  atomic functions.

- **`bufio.ReadBytes`** (issue #211): `(*Reader).ReadBytes(delim byte)
  ([]byte, error)` — complementary to the existing `ReadString` case.

- **2 new integration tests**: `os_complete`, `atomic_generics`. Total:
  247 integration + 12 showcase = 259 tests.

### Closes

- issue #211

## [0.68.0] - 2026-03-06

### Added

- **14 new `math.*` intercepts** (issue #210): `Sincos`, `Asinh`, `Acosh`, `Atanh`,
  `Float64bits`, `Float64frombits`, `Float32bits`, `Float32frombits`,
  `Nextafter`, `Nextafter32`, `Jn`, `Y0`, `Y1`, `Yn` — all added to
  `handleMathCall` in `stdlib.go`. Concrete arguments produce real results;
  unknown arguments return conservative zero values.

- **Integration test** `testdata/integration/math_complete`: exercises all 14 new
  functions, verifying 0 violations. Total: 245 integration + 12 showcase = 257 tests.

- **Updated `unmodeled_demo`**: switched demo from `math.Sincos` (now modeled) to
  `math.Erfinv` (remains unmodeled); `TestUnmodeledCallsReport` updated to match.

### Closes

- issue #210

## [0.67.0] - 2026-03-06

### Added

- **`RunResult.UnmodeledCalls []string`** (issue #209): sorted, deduplicated list of
  `"pkg/path.FuncName"` strings for functions called without a Giri model. Two
  categories are tracked:
  1. **No SSA body** (`callee.Blocks == nil`): assembly-backed functions with no
     interpretable Go source — these return opaque zero values.
  2. **Cross-package, no intercept**: functions where `execStdlibCall` had no case
     AND the call crosses a package boundary — these are executed via pure SSA
     interpretation without a dedicated model.
  The cross-package filter suppresses intra-package helper calls to avoid noise from
  recursively-interpreted stdlib internals.

- **Assembly-backed invoke dispatch tracking**: when a method is resolved via interface
  dispatch but `fn.Blocks == nil` (assembly implementation), the method is now recorded
  in `UnmodeledCalls`.

- **`-v` CLI flag now prints unmodeled calls**: when `-v` is set, any entries in
  `RunResult.UnmodeledCalls` are printed to stderr with a count header.

- **`RunN()` merges unmodeled calls** across multiple runs, deduplicating by name.

- **Integration test**: `testdata/integration/unmodeled_demo` exercises `math.Erfinv`
  (absent from `handleMathCall`) to verify `UnmodeledCalls` is populated (1 entry).
  (Originally used `math.Sincos`; updated in v0.68.0 after `Sincos` was modeled.)

### Fixed

- `Interpreter.recordUnmodeled` helper added to deduplicate `unmodeledCalls` map
  updates (replaces duplicated inline code at two sites).

## [0.66.0] - 2026-03-06

### Added

- **`golang.org/x/text/encoding/korean` intercepts** (issue #205). `EUCKR.NewDecoder/
  NewEncoder` → opaque; `String` → ""; `ID` → (0, "").

- **`golang.org/x/text/encoding/simplifiedchinese` intercepts** (issue #205).
  `GB18030/GBK/HZGB2312.NewDecoder/NewEncoder` → opaque; `String/ID` stubs.

- **`golang.org/x/text/encoding/traditionalchinese` intercepts** (issue #205).
  `Big5.NewDecoder/NewEncoder` → opaque; `String/ID` stubs.

- **`golang.org/x/text/encoding/ianaindex` intercepts** (issue #206).
  `(*Index).Encoding(name)` → (opaque, nil); `(*Index).Name(enc)` → ("utf-8", nil).
  Note: method is `Encoding()` not `Get()`.

- **`golang.org/x/net/trace` intercepts** (issue #206). `New` → opaque `Trace`;
  `NewContext` → ctx passthrough; `FromContext` → (opaque, false); `NewEventLog` →
  opaque; `Trace/EventLog` method stubs; HTTP handler noops.

- **`golang.org/x/net/dns/dnsmessage` intercepts** (issue #207). `MustNewName` →
  opaque; `NewName` → (opaque, nil); `*Message.Pack/AppendPack` → ([]byte, nil);
  `Unpack` → nil; `*Parser.Start` → (Header, nil); `Question/AllQuestions/
  SkipAllQuestions` stubs; `*Builder` method stubs; `Name.String` → "".

- **`golang.org/x/net/http2/hpack` intercepts** (issue #208). `HuffmanEncodeLength`
  → 0; `HuffmanDecodeToString` → ("", nil); `HuffmanDecode` → (0, nil);
  `AppendHuffmanString` → passthrough dst; `NewEncoder/NewDecoder` → opaque;
  `*Encoder.WriteField/SetMaxDynamicTableSizeLimit` stubs;
  `*Decoder.Write/Close/Set*` stubs.

- **`golang.org/x/sync/syncmap` intercepts** (issue #208). `syncmap.Map` is a
  type alias for `sync.Map`; `Store/Delete/Swap/Clear` → noop; `Load/LoadAndDelete`
  → (nil, false); `LoadOrStore` → (v, false); `Range` → noop.

- 4 new integration tests: `text_encoding_cjk`, `text_ianaindex_trace`,
  `dns_message`, `hpack_syncmap`. Total: 243 integration + 12 showcase = 255 tests.

- Closes #205, #206, #207, #208.

## [0.65.0] - 2026-03-06

### Added

- **`golang.org/x/crypto/scrypt` intercepts** (issue #201). `Key` →
  ([]byte, nil).

- **`golang.org/x/crypto/chacha20` intercepts** (issue #201). Low-level
  stream cipher (distinct from `chacha20poly1305`). `NewUnauthenticatedCipher` →
  (opaque, nil); `HChaCha20` → ([]byte, nil); `*Cipher.XORKeyStream/SetCounter/Advance`
  noop.

- **`golang.org/x/crypto/xts` intercepts** (issue #201). Disk-encryption
  mode. `NewCipher` → (opaque, nil); `*Cipher.Encrypt/Decrypt` noop.

- **`golang.org/x/crypto/salsa20` intercepts** (issue #201). Package-level
  `XORKeyStream` noop.

- **`golang.org/x/text/message` intercepts** (issue #202). `NewPrinter` →
  opaque `*Printer`; `MatchLanguage` → opaque tag; `*Printer.Print/Printf/Println/
  Fprint/Fprintf/Fprintln` → (0, nil); `Sprint/Sprintf/Sprintln` → ""; `Set/SetString`
  → nil.

- **`golang.org/x/text/number` intercepts** (issue #202). `Decimal/Scientific/
  Engineering/Percent` → opaque option values.

- **`golang.org/x/text/currency` intercepts** (issue #202). `MustParseISO` →
  opaque Unit; `ParseISO` → (opaque, nil); `NarrowSymbol/Symbol/ISO` → opaque;
  `String` → ""; `Unit.Amount` → opaque.

- **`golang.org/x/text/unicode/bidi` intercepts** (issue #203).
  `AppendReverse` → passthrough in; `ReverseString` → passthrough;
  `LookupRune/Lookup/LookupString` → (opaque Properties, 1);
  Properties method stubs; `*Paragraph.SetBytes/SetString/Order` → (opaque, nil);
  `*Ordering.NumRuns` → 1; `Run/Direction` stubs.

- **`golang.org/x/text/unicode/runenames` intercepts** (issue #203). `Name` →
  "" (no UCD data loaded during interpretation).

- **`golang.org/x/text/secure/bidirule` intercepts** (issue #203).
  `Direction/DirectionString` → 0 (LeftToRight); `Valid/ValidString` → true.

- **`golang.org/x/text/secure/precis` intercepts** (issue #204).
  `NewFreeform/NewIdentifier/NewRestrictedProfile` → opaque `*Profile`;
  `*Profile.String` → (src passthrough, nil); `Bytes` → ([]byte, nil);
  `Append/AppendCompareKey` → ([]byte, nil); `Compare` → false;
  `CompareKey` → ("", nil); `NewTransformer` → opaque.

- **`golang.org/x/text/encoding/japanese` intercepts** (issue #204).
  `EUCJP/ShiftJIS/ISO2022JP.NewDecoder/NewEncoder` → opaque; `String` → "";
  `ID` → (0, "").

- **`golang.org/x/text/encoding/htmlindex` intercepts** (issue #204).
  `Get` → (opaque, nil); `Name` → ("utf-8", nil); `LanguageDefault` → "utf-8".

- 4 new integration tests: `crypto_stream`, `text_message_number`,
  `text_bidi_runenames`, `text_precis_encoding`. Total: 239 integration + 12
  showcase = 251 tests.

- Closes #201, #202, #203, #204.

## [0.64.0] - 2026-03-06

### Added

- **`golang.org/x/crypto/nacl/box` intercepts** (issue #197). `GenerateKey` →
  (pub, priv, nil); `Seal/SealAfterPrecomputation` → []byte; `SealAnonymous` →
  ([]byte, nil); `Open/OpenAfterPrecomputation/OpenAnonymous` → ([]byte, bool);
  `Precompute` noop.

- **`golang.org/x/crypto/nacl/secretbox` intercepts** (issue #197).
  `Seal` → []byte; `Open` → ([]byte, bool).

- **`golang.org/x/crypto/curve25519` intercepts** (issue #197). `X25519` →
  ([]byte, nil); `ScalarBaseMult/ScalarMult` noop.

- **`golang.org/x/crypto/poly1305` intercepts** (issue #197). Package-level
  `Sum` noop; `Verify` → false; `New` → opaque `*MAC`; `*MAC.Write/Size/Sum`.

- **`golang.org/x/crypto/blake2b` intercepts** (issue #198). `New/New256/New384/New512`
  → (opaque hash.Hash, nil); `NewXOF` → (opaque XOF, nil);
  `Sum256/Sum384/Sum512` → opaque; hash.Hash/XOF method stubs.

- **`golang.org/x/crypto/blake2s` intercepts** (issue #198). `New128/New256` →
  (opaque, nil); `Sum256` → opaque; hash.Hash method stubs.

- **`golang.org/x/crypto/ed25519` intercepts** (issue #198). Thin wrapper
  around stdlib `crypto/ed25519`; `GenerateKey` → (opaque pub, opaque priv, nil);
  `Sign` → []byte; `Verify` → false; `NewKeyFromSeed` → opaque.

- **`golang.org/x/text/encoding` intercepts** (issue #199).
  `HTMLEscapeUnsupported/ReplaceUnsupported` → opaque Encoder; Decoder/Encoder
  method stubs.

- **`golang.org/x/text/encoding/charmap` intercepts** (issue #199).
  `*Charmap.NewDecoder/NewEncoder` → opaque; `*Charmap.String` → "";
  `DecodeByte/EncodeRune/ID` stubs.

- **`golang.org/x/text/encoding/unicode` intercepts** (issue #199).
  `UTF16(endianness, bom)` → opaque Encoding; `BOMOverride` → opaque;
  `NewDecoder/NewEncoder` stubs.

- **`golang.org/x/text/collate` intercepts** (issue #200). `New/NewFromTable` →
  opaque `*Collator`; `CompareString/Compare` → 0; `Key/KeyFromString` → []byte;
  `Sort/SortStrings` noop; `Supported` → [].

- **`golang.org/x/text/search` intercepts** (issue #200). `New` → opaque
  `*Matcher`; `Index/IndexString` → (-1, -1); `Equal/EqualString` → false;
  `Compile/CompileString` → opaque; `FindAllIndex` → nil.

- 4 new integration tests: `nacl_curve_poly`, `blake2_ed25519`,
  `text_encoding_charmap`, `text_collate_search`. Total: 243 integration tests.

- Closes #197, #198, #199, #200.

## [0.63.0] - 2026-03-03

### Added

- **`golang.org/x/text/cases` intercepts** (issue #193). `Lower/Upper/Title/Fold`
  return opaque `Caser`; `Caser.String/Bytes` passthrough the input.

- **`golang.org/x/text/language` intercepts** (issue #193). `Parse/Make/MustParse`
  return opaque `Tag`; `ParseAcceptLanguage` returns opaque tag list;
  `NewMatcher/Match` supported; `Tag.String` returns `"en"`.

- **`golang.org/x/text/transform` intercepts** (issue #193). `String/Bytes/Append`
  passthrough the source; `NewReader/NewWriter/Chain/RemoveFunc` return opaque.

- **`golang.org/x/text/unicode/norm` intercepts** (issue #194). `Form.String/Bytes`
  passthrough input; `IsNormal/IsNormalString` return false (conservative);
  `Append/AppendString` passthrough dst.

- **`golang.org/x/text/width` intercepts** (issue #194). `Fold/Narrow/Widen`
  transformer vars return opaque; `Lookup/LookupRune/LookupString` return opaque
  Properties.

- **`golang.org/x/text/runes` intercepts** (issue #194). `Map/Remove/If/ReplaceIllFormed/In/NotIn/Predicate` return opaque transformer.

- **`golang.org/x/term` intercepts** (issue #195). `IsTerminal` → false;
  `GetSize` → (80, 24, nil); `ReadPassword` → ([]byte{}, nil);
  `MakeRaw` → (opaque, nil); `Restore/GetState` supported.

- **`golang.org/x/crypto/chacha20poly1305` intercepts** (issue #195).
  `New/NewX` → (opaque AEAD, nil); `NonceSize/Overhead/Seal/Open` methods.

- **`golang.org/x/crypto/argon2` intercepts** (issue #195).
  `Key/IDKey` → opaque []byte (no actual KDF computation).

- **`golang.org/x/crypto/ssh` intercepts** (issue #196). `Dial` → (opaque, nil);
  `ParsePrivateKey/ParsePublicKey/ParseAuthorizedKey/ParseKnownHosts` →
  opaque results; `NewSignerFromKey/NewPublicKey/FingerprintSHA256`;
  `InsecureIgnoreHostKey/FixedHostKey`; `*Client` and `*Session` methods.

- 4 new integration tests: `text_cases_language`, `text_unicode_norm`,
  `term_chacha_argon`, `crypto_ssh`. Total: 239 integration tests.

- Closes #193, #194, #195, #196.

## [0.62.0] - 2026-03-02

### Changed (Performance)

- **`exec.go`: O(n²) → O(1) runQueue removal** (issue #189). Replaced
  `append(runQueue[:i], runQueue[i+1:]...)` with swap-last-and-shrink at both
  dequeue sites (main run loop and test run loop). Removal is O(1) since the
  scheduler, not the queue order, determines which GID runs next.

- **`report.go`: single-pass `htmlEscape`** (issue #189). Replaced three
  sequential `strings.ReplaceAll` calls with a package-level
  `strings.NewReplacer` that scans the string once.

### Refactored

- **`interpreter.go`: vector clock helpers** (issue #190). Extracted two
  helpers — `snapshotClock(g *Goroutine) map[int64]uint64` and
  `mergeClock(g *Goroutine, src map[int64]uint64)` — replacing six identical
  inline clock copy/merge blocks in `handleSyncCall` (Unlock, Done, Signal,
  Broadcast snapshot; Lock and Wait merge).

- **`stdlib.go`: package-level `stdlibOpaque` sentinel** (issue #191). Added
  `var stdlibOpaque = Value{Raw: struct{}{}}` and replaced 98 local
  `opaque := Value{Raw: struct{}{}}` declarations (one per handler call) with
  `opaque := stdlibOpaque`.

- **`loader.go`: `buildSSAFrom` helper** (issue #191). Extracted a
  `buildSSAFrom(initial []*packages.Package) (*ssa.Program, []*ssa.Package)`
  helper, consolidating the four identical `ssautil.AllPackages` + `prog.Build()`
  call-pairs in `LoadProgram`, `LoadTestProgram`, `LoadAllPrograms`, and
  `LoadTestPrograms`.

### Fixed

- **`stdlib.go`: removed dead `if interp.valueStore != nil` guards** (issue #191).
  `valueStore` is always initialized in `New()` and `newWithArena()`; the four
  defensive nil checks in `handleAtomicCall` were unreachable and removed.

### Added

- **`Scheduler.OnFinish(gid int64)` lifecycle hook** (issue #192). Added
  `OnFinish` to the `Scheduler` interface; `RoundRobin` and `Random` implement
  it as a noop; `PCT` uses it to `delete(s.priorities, gid)`, preventing
  unbounded map growth for programs that spawn many goroutines. The interpreter
  calls `OnFinish` at all four `GoroutineFinished` transition sites.

## [0.61.0] - 2026-03-02

### Added

- **`golang.org/x/sys/unix` intercepts** (issue #185): `Getpid`/`Getuid`/`Geteuid`/`Getgid`
  return realistic int values; `Getenv`/`Getwd` → safe zero tuples; file ops
  `Open`/`Close`/`Read`/`Write`/`Stat`/`Lstat` → `(int/0, nil)`; network ops
  `Socket`/`Connect`/`Bind`/`Listen`/`Accept`; byte helpers
  `ByteSliceFromString`/`ByteSliceToString`/`BytePtrFromString`/`BytePtrToString`;
  signal/process `Kill`/`Getppid`; misc `Chmod`/`Chown`/`Mkdir`/`Remove`/`Rename`/`Dup`.
  Requires `go get golang.org/x/sys` (already in transitive module graph).

- **`golang.org/x/net/html` intercepts** (issue #186): `Parse`/`ParseFragment` →
  `(*Node, nil)`; `NewTokenizer` → opaque; `Render` → nil; `EscapeString`/`UnescapeString`
  concrete passthrough; `Tokenizer` methods `Next`/`Token`/`Raw`/`Text`/`TagName`/`TagAttr`.

- **`golang.org/x/net/publicsuffix` intercepts** (issue #186): `PublicSuffix` →
  `("com", true)`; `EffectiveTLDPlusOne` → `(domain, nil)`.

- **`golang.org/x/net/idna` intercepts** (issue #186): `ToASCII`/`ToUnicode` →
  `(s, nil)`; `New`/`Lookup` → opaque.

- **`golang.org/x/net/proxy` intercepts** (issue #186): `SOCKS5` → `(Dialer, nil)`;
  `Direct` → opaque; `Dial` → `(Conn, nil)`; `RegisterDialerType` → noop;
  `*PerHost.AddFromString`/`AddNetwork`/`AddZone`/`AddHost` → noops.

- **`golang.org/x/net/netutil` intercepts** (issue #186): `LimitListener` → opaque.

- **`golang.org/x/mod/semver` intercepts** (issue #187): `IsValid`/`Canonical`/`Major`/
  `MajorMinor`/`Prerelease`/`Build` concrete passthrough; `Compare` → int;
  `Max` → string; `Sort` → noop.

- **`golang.org/x/mod/module` intercepts** (issue #187): `CheckPath`/`CheckImportPath`/
  `CheckFilePath` → nil; `EscapePath`/`EscapeVersion` → `("", nil)`;
  `CanonicalVersion` → string; `IsPseudoVersion` → false.

- **`golang.org/x/mod/modfile` intercepts** (issue #187): `Parse`/`ParseLax` →
  `(*File, nil)`; `AutoQuote`/`ModulePath` → string; `Format` → `([], nil)`;
  `*File` mutation methods → noops.

- **`crypto` top-level intercepts** (issue #188): `RegisterHash` → noop;
  `SignMessage` → `([]byte, nil)`; `Hash.New`/`Available`/`String`/`Size` →
  safe values.

- **`golang.org/x/net/http/httpguts` intercepts** (issue #188):
  `HeaderValuesContainsToken` → false; `IsTokenRune` → false;
  `ValidHeaderFieldName`/`ValidHeaderFieldValue`/`ValidHostHeader`/`ValidTrailerHeader` → true;
  `PunycodeHostPort` → `(host, nil)`.

- **`golang.org/x/net/html/charset` intercepts** (issue #188):
  `DetermineEncoding` → `(opaque, "utf-8", false)`; `NewReader` → opaque;
  `NewReaderLabel` → `(opaque, nil)`.

- **`testing/cryptotest` intercepts** (issue #188): `SetGlobalRandom` → noop;
  `TestAEAD`/`TestHash`/`TestStream`/`TestBlock` → noops.

- Four new integration tests: `sys_unix`, `net_html_publicsuffix`,
  `mod_semver_module`, `crypto_httpguts`; 227 total (216 integration + 11 showcase).

### Changed

- `go.mod`: added `golang.org/x/net` and `golang.org/x/sys` as direct dependencies
  (both were already in the transitive module graph via `golang.org/x/tools`).

### Fixed

- `httpguts` handler uses correct function names (`ValidHeaderFieldName` not
  `IsValidHeaderFieldName`).

## [0.60.0] - 2026-03-02

### Added

- **`crypto/des` intercepts** (issue #181): `NewCipher`/`NewTripleDESCipher` →
  `(cipher.Block, nil)`.

- **`crypto/rc4` intercepts** (issue #181): `NewCipher` → `(*Cipher, nil)`;
  `*Cipher.XORKeyStream`/`Reset` → noops.

- **`crypto/pbkdf2` intercepts** (issue #181): Generic `Key[H]` → `([]byte, nil)`
  (Go 1.24+, formerly `golang.org/x/crypto/pbkdf2`).

- **`crypto/hkdf` intercepts** (issue #181): `Extract`/`Expand`/`Key` → `([]byte, nil)`
  (Go 1.24+, formerly `golang.org/x/crypto/hkdf`).

- **`crypto/sha3` intercepts** (issue #182): `New224`/`New256`/`New384`/`New512` →
  opaque `*SHA3`; `NewSHAKE128`/`NewSHAKE256`/`NewCSHAKE128`/`NewCSHAKE256` → opaque;
  `Sum224`/`Sum256`/`Sum384`/`Sum512` → opaque; `SumSHAKE128`/`SumSHAKE256` →
  `[]byte{}`; `Write`/`Sum`/`Reset`/`Size`/`BlockSize`/`Read`/`Clone` methods.
  (Go 1.24+)

- **`crypto/hpke` intercepts** (issue #182): KEM constructors (`DHKEM`/`MLKEM768`/
  `MLKEM1024`/etc.)/`NewKEM`; KDF constructors (`HKDFSHA256`/etc.)/`NewKDF`; AEAD
  constructors (`AES128GCM`/`AES256GCM`/`ChaCha20Poly1305`/`ExportOnly`)/`NewAEAD`;
  `Seal`/`Open` → `([]byte, nil)`. (Go 1.24+)

- **`crypto/mlkem` intercepts** (issue #182): `GenerateKey768`/`GenerateKey1024` →
  `(*DecapsulationKey*, nil)`; `NewDecapsulationKey768`/`1024` and
  `NewEncapsulationKey768`/`1024`; `Encapsulate` → `(ciphertext, sharedKey, nil)`;
  `Decapsulate` → `(sharedKey, nil)`. (Go 1.24+)

- **`crypto/fips140` intercepts** (issue #182): `Enabled`/`Enforced` → false;
  `Version` → ""; `WithoutEnforcement` probes callback. (Go 1.24+)

- **`database/sql/driver` intercepts** (issue #183): `IsValue`/`IsScanValue` → true;
  `DefaultParameterConverter` → opaque.

- **`crypto/x509/pkix` intercepts** (issue #183): `Name.String` → `"<pkix.Name>"`;
  `Name.ToRDNSequence` → opaque; `Name.FillFromRDNSequence`/`AppendPKCS7` → noop/`[]byte`.

- **`image/color/palette` intercepts** (issue #183): `Plan9`/`WebSafe` are package-level
  variables accessed via globals; handler is a safe noop for any function calls.

- **`time/tzdata` intercepts** (issue #183): import-side-effect-only package; handler is
  a noop (no exported functions).

- **`structs` intercepts** (issue #184): marker-type-only package (no exported
  functions); handler is a noop. (Go 1.24+)

- **`weak` intercepts** (issue #184): `Make[T]` → opaque `Pointer[T]`;
  `Pointer.Value` → nil (GC may collect). (Go 1.24+)

- **`testing/slogtest` intercepts** (issue #184): `TestHandler` → nil error;
  `Run` → noop. (Go 1.21+)

- **`testing/synctest` intercepts** (issue #184): `Test` probes callback with opaque
  `*testing.T`; `Wait` → noop. Note: the function is `Test` not `Run`. (Go 1.24+)

- 4 new integration tests: `crypto_des_hkdf`, `crypto_sha3_hpke`, `sqldriver_pkix`,
  `weak_synctest`; **212 integration + 11 showcase = 223 total tests**.

- Closes issues #181, #182, #183, #184.

## [0.59.0] - 2026-03-02

### Added

- **`go/build/constraint` intercepts** (issue #177): `IsGoBuild`/`IsPlusBuild` →
  bool; `Parse`/`ParseLine` → `(Expr, nil)`; `GoVersion` → string; `PlusBuildLines` →
  `([]string, nil)`; `AndExpr`/`OrExpr`/`NotExpr`/`TagExpr` constructors → opaque.

- **`go/doc/comment` intercepts** (issue #177): `(*Parser).Parse` → opaque `*Doc`;
  `(*Printer).HTML`/`Markdown`/`Text`/`Comment` → `[]byte`; `DefaultLookupPackage` →
  `("", false)`. (Go 1.19+)

- **`text/template/parse` intercepts** (issue #177): `New` → opaque `*Tree`;
  `Parse` → `(map[string]*Tree, nil)`; `(*Tree).Copy`/`ErrorContext` handled.

- **`debug/gosym` intercepts** (issue #178): `NewLineTable` → opaque `*LineTable`;
  `NewTable` → `(*Table, nil)`; `*Table.PCToFunc`/`PCToLine`/`LineToPC`/`LookupFunc`/
  `LookupSym`/`Syms` handled.

- **`debug/plan9obj` intercepts** (issue #178): `Open`/`NewFile` → `(*File, nil)`;
  `*File.Close`/`Symbols`/`Section`/`DWARF`; `*Section.Data` handled.

- **`runtime/metrics` intercepts** (issue #178): `All` → `[]Description{}`; `Read` →
  noop; `Descriptions` → empty map (Go 1.16+).

- **`runtime/coverage` intercepts** (issue #178): `ClearCounters`/`WriteCounters`/
  `WriteCountersDir`/`WriteMetaDir`/`WriteMeta` → noops (Go 1.20+).

- **`net/http/cgi` intercepts** (issue #179): `Serve` → noop; `Request` →
  `(*Request, nil)`; `RequestFromMap` → `(*Request, nil)`.

- **`net/http/fcgi` intercepts** (issue #179): `Serve` → noop; `ProcessEnv` →
  empty `map[string]string`.

- **`encoding/ascii85` intercepts** (issue #179): `Encode` → int (input length);
  `Decode` → `(ndst, nsrc int, nil)`; `MaxEncodedLen` → concrete calculation;
  `NewEncoder`/`NewDecoder` → opaque; `Write`/`Close`/`Read` methods handled.

- **`index/suffixarray` intercepts** (issue #179): `New` → opaque `*Index`;
  `*Index.Bytes`/`Lookup`/`FindAllIndex` → empty slices; `Read`/`Write`/`Len` handled.

- **`log/syslog` intercepts** (issue #179): `New`/`Dial`/`NewLogger` →
  `(*Writer, nil)`; `*Writer.Write`/`Close`/`Emerg`/`Alert`/`Crit`/`Err`/`Warning`/
  `Notice`/`Info`/`Debug` handled (unix-only package, safe values on all platforms).

- **`crypto/dsa` intercepts** (issue #180): `GenerateParameters`/`GenerateKey` →
  nil error; `Sign` → `(*big.Int, *big.Int, nil)`; `Verify` → false.

- **`crypto/elliptic` intercepts** (issue #180): `P224`/`P256`/`P384`/`P521` →
  opaque `Curve`; `GenerateKey` → `(priv, x, y, nil)`; `Marshal`/`MarshalCompressed` →
  `[]byte{}`; `Unmarshal`/`UnmarshalCompressed` → `(opaque, opaque)`;
  `Params`/`IsOnCurve`/`Add`/`Double`/`ScalarMult`/`ScalarBaseMult` handled.

- **`hash/crc64` intercepts** (issue #180): `MakeTable` → opaque `*Table`;
  `New` → opaque `hash.Hash64`; `Checksum`/`Update` → uint64(0); hash methods
  (`Write`/`Sum64`/`Sum`/`Reset`/`Size`/`BlockSize`).

- **`golang.org/x/crypto/bcrypt` intercepts** (issue #180): `GenerateFromPassword` →
  `([]byte{}, nil)`; `CompareHashAndPassword` → nil; `Cost` → `(10, nil)`.

- **`golang.org/x/net/http2` intercepts** (issue #180): `ConfigureServer`/
  `ConfigureTransport`/`ConfigureTransports` → noops; `NewFramer` → opaque;
  `Framer.ReadFrame`/`WriteData`/`WriteHeaders` and other frame write methods → noops.

- 4 new integration tests: `go_constraint_template`, `debug_gosym_metrics`,
  `cgi_ascii85_suffixarray`, `crypto_elliptic_crc64`; **208 integration + 11 showcase = 219 total tests**.

- Closes issues #177, #178, #179, #180.

## [0.58.0] - 2026-03-02

### Added

- **`crypto/subtle` intercepts** (issue #173): `ConstantTimeCompare` → int;
  `ConstantTimeByteEq`/`ConstantTimeLessOrEq`/`ConstantTimeSelect` → int;
  `ConstantTimeCopy` → noop; `XORBytes` → int (length of x); `WithDataIndependentTiming`
  probes the callback function.

- **`hash/maphash` intercepts** (issue #173): `MakeSeed` → opaque `Seed`;
  package-level `Bytes`/`String` → uint64; `*Hash.Write`/`WriteByte`/`WriteString` →
  (n, nil); `Sum64` → uint64(0); `Sum32` → uint32(0); `Reset`/`SetSeed` → noop;
  `Seed` method → opaque.

- **`regexp/syntax` intercepts** (issue #173): `Parse` → `(*Regexp, nil)`;
  `MustParse` → opaque; `Compile` → `(*Prog, nil)`; `SimplifyRegexp` → opaque;
  `IsWordChar` → false; `*Regexp.String`/`CapNames`/`MaxCap`/`Prefix` handled.

- **`unique` intercepts** (issue #173): `Make[T]` → opaque `Handle[T]`;
  `Handle.Value` → passthrough (Go 1.23+).

- **`go/printer` intercepts** (issue #174): `Fprint`/`Fprintf` → `(int, nil)`;
  `Sprint` → `("", nil)`.

- **`go/constant` intercepts** (issue #174): `MakeInt64`/`MakeUint64`/`MakeFloat64`/
  `MakeString`/`MakeBool`/`MakeFromLiteral`/`MakeImag` constructors;
  `BinaryOp`/`UnaryOp`/`Shift`/`ToComplex`/`Real`/`Imag` → opaque;
  `Compare` → false; `StringVal` → ""; `Int64Val`/`Uint64Val`/`Float64Val`/`Float32Val` →
  `(0, true)`; `BoolVal` → false; `Kind`/`Sign`/`BitLen`/`String`/`ExactString` methods.

- **`go/scanner` intercepts** (issue #174): `(*Scanner).Init` → noop;
  `(*Scanner).Scan` → `(Pos, Token, string)` zero values; `ErrorList.Error`/`Err`/`Len`.

- **`go/version` intercepts** (issue #174): `Compare` → concrete int comparison;
  `IsValid` → checks "go" prefix; `Lang` → string passthrough; `Max` → string max
  (Go 1.22+).

- **`debug/buildinfo` intercepts** (issue #175): `ReadFile`/`Read` → `(*BuildInfo, nil)`;
  `BuildInfo.String`/`MarshalText`/`UnmarshalText` handled.

- **`debug/dwarf` intercepts** (issue #175): `New` → `(*Data, nil)`;
  `*Data.Reader`/`Type`/`Ranges`/`LineReader`; `*Reader.Next`/`SeekPC`/`SkipChildren`/
  `Seek`/`AddressSize`; `*Entry.Val`/`AttrField`.

- **`debug/elf` intercepts** (issue #175): `Open`/`NewFile` → `(*File, nil)`;
  `*File.Section`/`SectionByType`/`Symbols`/`DynamicSymbols`/`ImportedSymbols`/
  `ImportedLibraries`/`DWARF`/`Segments`/`Sections`/`Close`; `*Section.Data`/`ReadAt`.

- **`debug/macho` intercepts** (issue #175): `Open`/`NewFile` → `(*File, nil)`;
  `*File.Section`/`Segment`/`Symtab`/`Dysymtab`/`DWARF`/`ImportedSymbols`/
  `ImportedLibraries`/`Close`; `*Section.Data`/`ReadAt`.

- **`debug/pe` intercepts** (issue #175): `Open`/`NewFile` → `(*File, nil)`;
  `*File.Section`/`DWARF`/`ImportedSymbols`/`ImportedLibraries`/`Close`;
  `*Section.Data`/`ReadAt`.

- **`testing/quick` intercepts** (issue #176): `Check`/`CheckCustom` probe the
  property function with zero-value args and return nil error; `Value` → `(reflect.Value, false)`.

- **`mime/quotedprintable` intercepts** (issue #176): `NewReader` → opaque;
  `NewWriter` → opaque; `*Reader.Read`/`*Writer.Write`/`Close` handled.

- **`net/http/httptrace` intercepts** (issue #176): `WithClientTrace` → returns
  first arg (context) unchanged; `ContextClientTrace` → opaque `*ClientTrace`.

- **`net/rpc/jsonrpc` intercepts** (issue #176): `NewClient` → opaque `*rpc.Client`;
  `Dial` → `(*Client, nil)`; `NewServerCodec`/`NewClientCodec` → opaque; `ServeConn` → noop.

- 4 new integration tests: `crypto_subtle_maphash`, `go_printer_constant`,
  `debug_binary`, `quick_quotedprintable`; **204 integration + 11 showcase = 215 total tests**.

- Closes issues #173, #174, #175, #176.

## [0.57.0] - 2026-03-02

### Added

- **`strings.NewReplacer` fix** (issue #169): Previously returned `Value{},false`
  causing false positives when code called `r.Replace(s)`. Now returns opaque
  non-nil `*Replacer`; `Replace` and `WriteString` method calls dispatch through
  existing string-handler cases.

- **`io/ioutil` intercepts** (issue #169): `ReadAll`/`ReadFile` → `([]byte, nil)`;
  `WriteFile` → nil error; `TempFile` → `(*File, nil)`; `TempDir` → `("/tmp/...", nil)`;
  `NopCloser` → opaque `io.ReadCloser`; `ReadDir` → `([]FileInfo, nil)`.

- **`compress/bzip2` intercepts** (issue #170): `NewReader` → opaque `io.Reader`.

- **`compress/flate` intercepts** (issue #170): `NewReader`/`NewReaderDict` →
  opaque `io.ReadCloser`; `NewWriter`/`NewWriterDict` → `(*Writer, nil)`;
  `Read`/`Write`/`Close`/`Flush`/`Reset` handled.

- **`compress/lzw` intercepts** (issue #170): `NewReader` → opaque `io.ReadCloser`;
  `NewWriter` → opaque `io.WriteCloser`; `Read`/`Write`/`Close` handled.

- **`go/types` intercepts** (issue #171): constructors (`NewPackage`/`NewScope`/
  `NewNamed`/`NewStruct`/`NewInterface`/`NewSignature`/`NewFunc`/`NewVar`/
  `NewConst`/`NewTypeName`/`NewArray`/`NewSlice`/`NewMap`/`NewChan`/`NewPointer`/
  `NewTuple`/`NewParam`); `NewChecker` + `(*Config).Check` → `(*Package, nil)`;
  predicate functions (`Implements`/`AssignableTo`/`ConvertibleTo`/`Identical`);
  `TypeString`/`ObjectString`; `NewMethodSet`/`LookupFieldOrMethod`/`MissingMethod`;
  method sets for `Scope`/`Named`/`Object`/`Interface`/`Struct`/`Signature`/`Tuple`.

- **`go/importer` intercepts** (issue #171): `Default`/`For`/`ForCompiler` →
  opaque `Importer`; `Import` → `(*types.Package, nil)`.

- **`go/build` intercepts** (issue #171): `Import`/`ImportDir` →
  `(*Package, nil)`; `IsDir`/`IsLocalImport` → false; `MatchFile` → `(false, nil)`;
  `SrcDirs` → empty slice.

- **`go/doc` intercepts** (issue #171): `New`/`NewFromFiles` → `(*Package, nil)`;
  `Examples` → empty slice; `Synopsis` → concrete first-80-chars passthrough;
  `ToHTML`/`ToText`/`ToMarkdown` → noops; `IsPredeclared` → false.

- **`net/http/cookiejar` intercepts** (issue #172): `New` → `(*Jar, nil)`;
  `SetCookies` → noop; `Cookies` → empty `[]*http.Cookie`.

- 4 new integration tests: `ioutil_replacer`, `compress_extras`, `go_types_build`,
  `cookiejar_http`; **200 integration + 11 showcase = 211 total tests**.

- Closes issues #169, #170, #171, #172.

## [0.56.0] - 2026-03-02

### Added

- **`net/http/httptest` intercepts** (issue #165): `NewRecorder` → opaque
  `*ResponseRecorder`; `NewServer`/`NewTLSServer`/`NewUnstartedServer` → opaque
  `*Server`; `*ResponseRecorder` methods (`WriteHeader`/`Write`/`WriteString`/
  `Flush`/`Header`/`Result`) return safe defaults; `*Server.Close`/`Start`/
  `StartTLS`/`Client` handled.

- **`net/http/httputil` intercepts** (issue #165): `NewSingleHostReverseProxy`
  → opaque `*ReverseProxy`; `DumpRequest`/`DumpRequestOut`/`DumpResponse` →
  `([]byte, nil)`; `NewChunkedReader`/`NewChunkedWriter` → opaque;
  `*ReverseProxy.ServeHTTP` noop. Note: `NewReverseProxy` does not exist in Go
  1.21+ — the function is `NewSingleHostReverseProxy`.

- **`net/rpc` intercepts** (issue #166): `Dial`/`DialHTTP`/`DialHTTPPath` →
  `(*Client, nil)`; `*Client.Call` → nil error; `*Client.Go` → opaque `*Call`;
  `*Client.Close` → nil error; `Register`/`RegisterName` → nil error;
  `NewServer` → opaque; `ServeConn`/`Accept`/`HandleHTTP` noop.

- **`runtime/pprof` extended + `net/http/pprof` intercepts** (issue #167):
  Extended `runtime/pprof` handler with `Profiles`/`NewProfile`/`*Profile.Name`/
  `*Profile.Count`/`*Profile.Add`/`*Profile.Remove`/`*Profile.WriteTo`/
  `Handler`. `net/http/pprof` functions (`Index`/`Cmdline`/`Profile`/`Symbol`/
  `Trace`/`Handler`) modeled as noops. Note: `debug/pprof` does not exist in Go
  stdlib — the packages are `runtime/pprof` and `net/http/pprof`.

- **`plugin` intercepts** (issue #168): `Open` → `(*Plugin, nil)`;
  `*Plugin.Lookup` → `(Symbol, nil)`.

- **`golang.org/x/sync/semaphore` intercepts** (issue #168): `NewWeighted` →
  opaque `*Weighted`; `Acquire` → nil (modeled as immediate success);
  `TryAcquire` → true; `Release` → noop.

- 4 new integration tests: `httptest_httputil`, `net_rpc`, `pprof_extended`,
  `plugin_semaphore`; **196 integration + 11 showcase = 207 total tests**.

- Closes issues #165, #166, #167, #168.

## [0.55.0] - 2026-03-02

### Added

- **`fmt.Scan`/`Scanf`/`Scanln`/`Fscan`/`Fscanf`/`Fscanln` intercepts** (issue #161):
  stdin-reading and reader-reading variants return `(0, nil)` matching the
  already-handled `Sscan`/`Sscanf`/`Sscanln` family.

- **`net/smtp` intercepts** (issue #162): `Dial`/`NewClient` → `(*Client, nil)`;
  `SendMail` → nil; `PlainAuth`/`CRAMMD5Auth` → opaque Auth; `*Client` methods
  (`Auth`, `Mail`, `Rcpt`, `Data`, `Quit`, `Close`, `Reset`, `StartTLS`, etc.)
  return nil error.

- **`net/mail` intercepts** (issue #162): `ParseAddress` → `(*Address, nil)`;
  `ParseAddressList` → `([]*Address, nil)`; `NewReader`/`ReadMessage` → opaque/nil.

- **`net/textproto` intercepts** (issue #162): `NewConn`/`NewReader`/`NewWriter`
  → opaque; `CanonicalMIMEHeaderKey` attempts concrete title-case passthrough;
  `TrimString` delegates to `strings.TrimSpace`; `*Conn`/`*Reader` methods noop.

- **`go/token` intercepts** (issue #163): `NewFileSet`/`NewFile`/`AddFile` →
  opaque; `Lookup` → IDENT constant; `Pos`/`Position`/`Line`/`Offset` → concrete
  int; `FileSet.Iterate` probes callback with opaque `*File`; `IsValid` → true.

- **`go/ast` intercepts** (issue #163): `Inspect` and `Walk` probe the visitor
  callback once with an opaque node so goroutine-body violations surface;
  `IsExported` checks first character case for concrete string args; other helpers
  (`Print`/`Fprint`/`NewIdent`/`MergePackageFiles`) return safe defaults.

- **`go/parser` intercepts** (issue #163): `ParseFile` → `(*ast.File, nil)`;
  `ParseDir` → `(map, nil)`; `ParseExpr`/`ParseExprFrom` → `(Expr, nil)`.

- **`go/format` intercepts** (issue #163): `Source`/`Node` → `([]byte, nil)`.

- **`syscall` intercepts** (issue #164): `Getpid`/`Getppid` → real PID;
  `Getuid`/`Getgid` → real UID/GID; `Getenv` → `(val, found)` pair;
  `Getpagesize` → 4096; `Open`/`Close`/`Read`/`Write`/`Stat`/`Lstat` → safe
  defaults; `Exit` marks all goroutines Panicked; socket/mmap/pipe ops noop.

- **`testing/iotest` intercepts** (issue #164): `ErrReader`/`NewReadLogger`/
  `NewWriteLogger`/`OneByteReader`/`HalfReader`/`DataErrReader`/`TruncateWriter`
  → opaque reader/writer; `TestReader` → nil error.

- **`testing/fstest` intercepts** (issue #164): `TestFS` → nil error;
  `MapFS` method calls (`Open`/`ReadDir`/`ReadFile`/`Stat`/`Sub`/`Glob`) →
  safe `(opaque, nil)` or `([]Value, nil)` returns.

- 4 new integration tests: `fmt_scan`, `net_mail_smtp`, `go_tooling`,
  `syscall_testing`; **192 integration + 11 showcase = 203 total tests**.

- Closes issues #161, #162, #163, #164.

## [0.54.0] - 2026-03-02

### Added

- **`golang.org/x/sync/errgroup` intercepts** (issue #157): `WithContext` returns
  `(*Group, ctx)`; `Go`/`TryGo` probe the callback closure synchronously so
  goroutine body violations surface; `Wait` returns nil error.

- **`golang.org/x/sync/singleflight` intercepts** (issue #157): `Do` probes the
  callback closure once and returns `(opaque, false, nil)`; `DoChan` does the
  same and returns a `chan Result` with a pre-populated result; `Forget` is a noop.

- **`encoding/gob` intercepts** (issue #158): `NewEncoder`/`NewDecoder` return
  opaque encoder/decoder; `Encode`/`EncodeValue`/`Decode`/`DecodeValue` are noops
  (return nil error); `Register`/`RegisterName` are noops.

- **`encoding/base32` intercepts** (issue #158): `EncodeToString`/`DecodeString`
  call real `base32.StdEncoding` for concrete byte-slice inputs; `Encode`/`Decode`
  are noops; `NewEncoding`/`WithPadding` return opaque encoding handles.

- **`image` + `image/color` intercepts** (issue #159): `NewRGBA`/`NewNRGBA`/
  `NewGray`/`NewCMYK`/`NewAlpha`/`NewPaletted` return opaque non-nil images;
  `Rect`/`Pt` return opaque rectangle/point; `color.RGBA.RGBA()` returns 4 zero
  uint32 values; `color.Model` converters are noops.

- **`image/png` + `image/jpeg` + `image/gif` intercepts** (issue #159): `Encode`
  returns nil error; `Decode`/`DecodeConfig` return `(opaque, nil)`; `jpeg.Encode`
  accepts options arg; `gif.EncodeAll`/`gif.DecodeAll` are similarly handled.

- **`expvar` intercepts** (issue #160): `NewInt`/`NewFloat`/`NewString`/`NewMap`
  return opaque `*Var`; `Add`/`Set`/`Value`/`String`/`Get`/`Do` are noops or
  return safe defaults; `Handler` returns nil.

- **`text/tabwriter` intercepts** (issue #160): `NewWriter` returns opaque writer;
  `Write`/`Flush`/`Init` are noops (return nil error).

- **`text/scanner` intercepts** (issue #160): `Init`/`Peek`/`Scan`/`TokenText`/
  `Pos`/`IsValid` all return safe default values; `Scan` returns `scanner.EOF`.

- 4 new integration tests: `errgroup_singleflight`, `encoding_gob`, `image_png`,
  `expvar_tabwriter`; **188 integration + 11 showcase = 199 total tests**.

- Closes issues #157, #158, #159, #160.

## [0.53.0] - 2026-03-02

### Added

- **`math/rand/v2` intercepts** (issue #153, Go 1.22+): `Int64`, `Float64`,
  `Float32`, `Uint64`, `IntN`, `N[T]`, `Perm`, `Shuffle` — uses `interp.rng`
  for bounded generators so results are deterministic under a fixed seed.
  `New`/`NewChaCha8`/`NewPCG` return opaque non-nil sources.

- **`encoding/pem` intercepts** (issue #154): `Decode` attempts real PEM
  parsing for concrete byte-slice args (returns concrete `*Block` + rest);
  falls back to `(opaque, nil)` for opaque inputs. `EncodeToMemory` returns
  a non-empty PEM sentinel. `Encode` is a noop.

- **`encoding/asn1` intercepts** (issue #154): `Marshal` returns a minimal
  DER TLV; `Unmarshal`/`UnmarshalWithParams` attempt real ASN.1 parsing on
  concrete byte inputs and fall back to `([], nil)` for opaque inputs.

- **`crypto/rsa` intercepts** (issue #155): `GenerateKey`/`GenerateMultiPrimeKey`
  → `(*PrivateKey, nil)`; `SignPSS`/`SignPKCS1v15` → `([]byte, nil)`;
  `VerifyPSS`/`VerifyPKCS1v15` → nil error; `EncryptOAEP`/`DecryptOAEP` →
  `([]byte, nil)`.

- **`crypto/ecdsa` intercepts** (issue #155): `GenerateKey` → `(*PrivateKey, nil)`;
  `SignASN1` → `([]byte, nil)`; `Verify`/`VerifyASN1` → `false` (conservative).

- **`crypto/ed25519` intercepts** (issue #155): `GenerateKey` → `(pub, priv, nil)`;
  `NewKeyFromSeed` → opaque key; `Sign` → `[]byte`; `Verify` → `false`.

- **`crypto/ecdh` intercepts** (issue #155, Go 1.20+): curve constructors
  `P256`/`P384`/`P521`/`X25519`, `GenerateKey`/`NewPrivateKey`/`NewPublicKey`,
  `ECDH` — all return opaque non-nil values with nil errors.

- **`crypto/x509` intercepts** (issue #155): `ParseCertificate`/`ParseCertificates`,
  `ParsePKCS1PrivateKey`/`ParsePKCS8PrivateKey`/`ParseECPrivateKey`/`ParsePKIXPublicKey`,
  marshal variants, `NewCertPool`/`SystemCertPool`, `CreateCertificate`.

- **`runtime/pprof` intercepts** (issue #156): `Lookup` → opaque `*Profile`;
  `StartCPUProfile`/`StopCPUProfile`/`WriteHeapProfile` → nil. All noops.

- **`runtime/trace` intercepts** (issue #156): `Start`/`Stop`, `NewTask` →
  `(ctx, *Task)`, `NewRegion` → opaque `*Region`, `Log`/`Logf`, `IsEnabled`
  → `false`.

- New integration tests: `rand_v2`, `encoding_pem`, `crypto_asymmetric`,
  `runtime_pprof` (4 tests, 187 total).

## [0.52.0] - 2026-03-01

### Added

- **`math/bits` intercepts** (issue #149): All bit-manipulation functions with
  concrete `uint64` passthrough: `LeadingZeros`, `TrailingZeros`, `OnesCount`,
  `RotateLeft`, `Reverse`, `ReverseBytes`, `Len`, `UintSize`. Multi-return
  arithmetic: `Add64`, `Sub64`, `Mul64`, `Div64` return `(result, carry/borrow)`
  tuples with concrete passthrough. Non-concrete args return a non-zero sentinel.

- **`math/cmplx` intercepts** (issue #149): 20+ complex-number functions with
  concrete `complex128` passthrough: `Abs`, `Phase`, `Polar`, `Rect`, `Conj`,
  `Exp`, `Log`, `Log10`, `Sqrt`, `Pow`, `Sin`/`Cos`/`Tan`,
  `Sinh`/`Cosh`/`Tanh`, `Asin`/`Acos`/`Atan`/`Asinh`/`Acosh`/`Atanh`,
  `IsNaN`, `IsInf`, `NaN`, `Inf`. Extends complex number support from v0.44.0
  to the full `math/cmplx` API.

- **`html` intercepts** (issue #150): `EscapeString` and `UnescapeString` with
  concrete string passthrough via the real stdlib.

- **`unicode/utf16` intercepts** (issue #150): `IsSurrogate`, `EncodeRune`,
  `DecodeRune` with concrete passthrough; `Encode`, `Decode`, `AppendRune`
  return opaque non-nil slices.

- **`os/user` intercepts** (issue #151): `Current`, `Lookup`, `LookupId`
  return `(opaque-*User, nil)`; `LookupGroup`, `LookupGroupId` return
  `(opaque-*Group, nil)`. Method calls on the returned objects return opaque
  non-nil strings.

- **`runtime/debug` intercepts** (issue #151): `Stack` returns a non-empty
  byte slice; `SetGCPercent`, `SetMemoryLimit`, `SetMaxStack`, `SetMaxThreads`
  return the previous value; `ReadGCStats`, `FreeOSMemory`, `PrintStack` are
  noops; `ReadBuildInfo` returns `(*BuildInfo, true)`.

- **`net/netip` intercepts** (issue #152, Go 1.18+): `ParseAddr` /
  `MustParseAddr`, `AddrFrom4` / `AddrFrom16`, `IPv4Unspecified` /
  `IPv6Unspecified` and other constructors return opaque `Addr` values.
  `AddrPortFrom` / `ParseAddrPort`, `PrefixFrom` / `ParsePrefix` similarly
  return opaque values. Predicate methods (`IsValid`→`true`,
  `IsLoopback`/`IsMulticast`/etc.→`false`) provide conservative returns.

- New integration tests: `math_bits`, `math_cmplx`, `html_escape`, `net_netip`
  (4 tests, 183 total).

## [0.51.0] - 2026-03-01

### Added

- **Go version awareness** (issue #153): `interpreter.Program` now carries a
  `GoVersion string` field (e.g. `"go1.23"`) populated from the module's
  `go.mod` via `packages.NeedModule`. All four loaders (`LoadProgram`,
  `LoadTest`, `LoadAllPrograms`, `LoadTestPrograms`) populate the field.
  The CLI displays the Go version in the "Interpreting..." status line:
  `Interpreting main (go1.23) with roundrobin scheduler (seed=0)...`

- **`iter` package intercepts** (issue #154, Go 1.23+): `iter.Pull` and
  `iter.Pull2` return a conservative (opaque next, opaque stop) pair so
  programs that convert push-based iterators to pull-based ones do not
  produce false positives. `stop` is automatically respected via `defer`.

- New integration test: `iter_pull` — exercises `iter.Pull` and `iter.Pull2`
  with sequences derived from `slices.Values` and `slices.All`; expected
  0 violations.

## [0.50.0] - 2026-03-01

### Added

- **`slices` package intercepts** (issue #149, Go 1.21+): `Contains`, `Index`,
  `Equal`, `Sort`, `SortStable`, `SortFunc`, `SortStableFunc`, `Reverse`,
  `Clone`, `Compact`, `CompactFunc`, `Clip`, `Grow`, `Insert`, `Delete`,
  `DeleteFunc`, `Replace`, `Concat`, `Repeat`, `Max`, `Min`, `MaxFunc`,
  `MinFunc`, `IsSorted`, `IsSortedFunc`, `BinarySearch`, `BinarySearchFunc`,
  `ContainsFunc`, `IndexFunc`, `EqualFunc`, `Collect`, `All`, `Values`,
  `Backward`, `AppendSeq`. Callback-accepting functions (`SortFunc`,
  `BinarySearchFunc`, `CompactFunc`, `DeleteFunc`, etc.) probe the callback
  with representative arguments to surface violations inside it.

- **`maps` package intercepts** (issue #150, Go 1.21+): `Clone`, `Copy`,
  `DeleteFunc`, `Equal`, `EqualFunc`, `Collect`, `All`, `Insert`. `Keys` and
  `Values` return opaque iterator values (`iter.Seq`). `DeleteFunc` probes its
  callback.

- **`cmp` package intercepts** (issue #151, Go 1.21+): `Compare` with concrete
  passthrough for `int64`, `float64`, and `string` comparisons; `Less` with
  concrete passthrough; `Or` returns the first non-zero argument.

- **`log/slog` package intercepts** (issue #152, Go 1.21+): All package-level
  logging functions (`Debug`, `Info`, `Warn`, `Error`, and `*Context` variants)
  are noops. `New`, `NewTextHandler`, `NewJSONHandler` return opaque non-nil
  values. `Default`, `SetDefault`, and `Logger` method calls handled.

### Fixed

- **Generic instantiation intercept** (issue #149): Calls to generic stdlib
  functions like `slices.Contains[[]int int]` have `callee.Package() == nil`
  in SSA (the package lives on the origin function). `execCall` now falls back
  to `callee.Origin().Package()` and `callee.Origin().Name()` when
  `callee.Package()` is nil, ensuring all four new packages — and any future
  generic stdlib packages — are correctly intercepted. Without this fix, Giri
  would attempt to execute the instantiated SSA body, returning incorrect values
  and causing false violations.

## [0.49.0] - 2026-03-01

### Fixed

- **`LoadAllPrograms`: unguarded `initial[0].Fset` access** (self-analysis,
  follow-up to v0.48.0): `LoadAllPrograms` called
  `ParseSuppressions(initial[0].Fset, initial)` before checking
  `len(initial) > 0`. When `packages.Load` returns an empty slice (e.g. under
  Giri's `go/packages` intercept), this caused an index-out-of-bounds /
  nil-pointer-deref at `ssautil/loader.go:250`.

  Fixed by extracting `fset` with the `len(initial) > 0` guard (already present
  two lines below, and correctly used in `LoadTestPrograms`) and passing that
  `fset` to `ParseSuppressions`. No behaviour change for real runs; eliminates
  the false positive in Giri's own self-analysis.

  Giri now reports **0 violations** when run on its own source tree
  (`giri ./...`).

## [0.48.0] - 2026-03-01

### Added

- **Package `init()` called before `main()`** (issue #146): Giri now invokes the
  main package's synthesized `init()` function before running `main()`. This
  correctly initializes package-level variables whose values come from function
  calls (e.g. `var ch = make(chan int, 1)`, `var s = strings.NewReplacer(...)`).

  Dependency package inits (e.g. `runtime.init()`, `fmt.init()`) would crash
  Giri if executed, so they are suppressed: any call to a function literally
  named `"init"` that reaches `execStdlibCall` returns immediately. User-defined
  init functions are renamed `init$1`, `init$2`, etc. in SSA and are therefore
  NOT filtered — they execute normally as part of the main package's init body.

  New integration test: `init_pkg_global` — `var ch = make(chan int, 1)` at
  package level; `main()` sends 42 and receives it → 0 violations (proves init
  correctly initialized the buffered channel).

- **`flag.*` intercepts now preserve default values** (issue #146): `flag.Bool`,
  `flag.String`, `flag.Int`, `flag.Int64`, `flag.Uint`, `flag.Uint64`,
  `flag.Float64`, and `flag.Duration` now initialize the returned pointer to the
  caller-specified default value instead of the zero value.

- **Native-pointer dereference in `handleLoad`** (issue #146): When loading
  through an untracked value whose `Raw` field is a native Go pointer to a
  primitive type (`*bool`, `*string`, `*int`, `*int64`, `*uint64`, `*float64`,
  `*uint`), `handleLoad` now dereferences it automatically. This allows code like
  `if *flagVerbose { ... }` to see the actual flag value rather than the pointer.

### Removed

- **Known Limitation resolved**: Issue #146 ("Package init() not called before
  main()") is now fixed. The warning previously documented in v0.47.0 has been
  removed.

## [0.47.0] - 2026-03-01

### Fixed

- **`handleLoad` uninitialized global: false out-of-bounds replaced by correct
  nil-pointer-deref** (issue #147): When a package-level global pointer is never
  written (because Giri starts from `main()` without calling `init()`), the
  `valueStore` has no entry for that allocation. The old fallthrough path returned
  the *container's* shadow pointer as the loaded value, inheriting the container's
  8-byte provenance. A subsequent dereference with the pointed-to type size (e.g.
  16 bytes for `string`) then called `CheckAccess` against the container allocation
  → false out-of-bounds.

  Fixed by returning `Value{}` (zero/nil) when `addr.Provenance.Offset == 0` and
  `valueStore` has no entry, instead of inheriting provenance. Uninitialized globals
  now correctly produce nil-pointer-deref on the first dereference — the right
  diagnosis for the root cause.

  Two new integration tests: `global_nil_ptr` (uninitialized `*string` global → 1
  nil-pointer-deref violation), `global_nil_ptr_valid` (global initialized in
  `main()` → 0 violations).

  *Discovered by running Giri on itself (`giri ./...` in the Giri source tree).
  See issue #148 for the second self-analysis violation.*

- **False nil-pointer-deref inside `golang.org/x/tools/go/packages.Load`** (issue
  #148): Programs that import `go/packages` (linters, code generators, build tools)
  triggered a nil-pointer-deref deep inside `packages.Load` because Giri attempted
  to execute the full `packages.Load` implementation. That function calls
  `go list` via `os/exec`, which is not possible inside the interpreter; some
  internal state that would normally be non-nil was nil in Giri's model.

  Added an intercept for `golang.org/x/tools/go/packages`: `Load` returns an
  empty package list and nil error; `NeedXxx` constants return an opaque non-zero
  value; all other function names return safe noops.

  *Discovered by running Giri on itself.*

### Known Limitation

- **Package `init()` not called before `main()`** (issue #146): Giri starts
  execution directly from `main()` without running the package's synthesized
  `init()` function. This means package-level variable declarations with
  function-call initializers (e.g. `var flagStrategy = flag.String(...)`) remain
  at their zero values during analysis. Accessing these uninitialized pointers now
  correctly reports nil-pointer-deref (improved by #147) rather than a spurious
  out-of-bounds, but the root cause is still present. Tracked in issue #146.

## [0.46.0] - 2026-03-01

### Fixed

- **Complex128 unary negation** (issue #144): `ssa.UnOp` with `token.SUB` handled `int64`
  and `float64` but not `complex128`. The expression `-c` where `c` is `complex128` returned
  `c` unchanged, causing any comparison like `neg != complex(-1,-2)` to go the wrong way.
  Added `} else if c, ok := operand.Raw.(complex128); ok { ... Value{Raw: -c} }` in the
  `token.SUB` handler. Also added defensive `int→complex` and `float→complex` cases in
  `convertValue` (e.g. `complex128(42)` → `42+0i`).

  Two new integration tests: `complex_neg` (unary negation + double-negation + zero-complex
  canaries), `complex_conv` (`complex64 ↔ complex128` widening/narrowing conversions).

- **`ssa.Select` receive readiness and `recvOk` consistency with v0.45.0 fix** (issue #145):
  The `ssa.Select` handler checked only `ch.hasPending` when deciding whether a receive case
  was ready, ignoring `ch.pendingCount > 0` (buffered channels) and `ch.closed` (closed
  channels always readable). Also, the `recvOk` value was not updated to use the same
  formula introduced in v0.45.0 for `token.ARROW CommaOk`.

  Fixed readiness: `ch.hasPending || ch.pendingCount > 0 || ch.closed`.
  Fixed recvOk: `!ch.closed || ch.hasPending || ch.pendingCount > 0` (computed before
  `handleChannelRecv`, consistent with `token.ARROW CommaOk`).

  Two new integration tests: `select_recv_ok` (select on closed buffered channel with pending
  items → `ok=true`), `select_recv_closed` (select on closed empty channel → `ok=false`).

## [0.45.0] - 2026-03-01

### Fixed

- **`string → []rune` and `[]rune → string` conversions** (issue #142): `convertValue` handled
  `string ↔ []byte` but not `string ↔ []rune`, a common Unicode text-processing pattern.
  Added two new cases in `convertValue` parallel to the existing byte-slice cases:
  - `string → []rune`: checks `dstSlice.Elem().Kind() == types.Int32`, converts with `[]rune(s)`
    and wraps each rune as `Value{Raw: int64(r)}`
  - `[]rune → string`: checks `srcSlice.Elem().Kind() == types.Int32`, reassembles from
    `[]Value` by casting each element with `rune(toInt64(r))`

  Two new integration tests: `string_to_rune` (Unicode string→rune-slice with canary on
  length and rune value), `rune_to_string` (round-trip and explicit rune-slice construction).

- **Range-over-channel: `for x := range ch` silently skipped all iterations** (issue #143):
  Go SSA lowers `for range ch` as `ssa.UnOp` with `token.ARROW, CommaOk=true` (NOT
  `ssa.Range`+`ssa.Next`). The `CommaOk` path always returned `ok=true`, causing an
  infinite loop for pre-populated closed channels and silently-zero iterations for
  conceptually-looping channels.

  Fixed the `token.ARROW, CommaOk=true` handler to compute `recvOk` **before** calling
  `handleChannelRecv` (so the last real item returns `ok=true`), using
  `recvOk = !ch.closed || ch.hasPending || ch.pendingCount > 0`. This correctly terminates
  the range loop when the channel is closed and fully drained.

  Two new integration tests: `range_chan` (pre-populated buffered channel, counts 3
  iterations via false-positive canary), `range_chan_valid` (empty closed channel → 0
  iterations, no false positive).

## [0.44.0] - 2026-03-01

### Fixed

- **Missing `token.AND_NOT` (`&^`) in `evalBinOp`** (issue #140): The bit-clear operator
  `a &^ b` was not handled in the integer arithmetic block of `evalBinOp`, causing any use
  of `&^` to return `Value{}`. This made comparisons involving bit-clearing produce non-bool
  results, sending `ssa.If` down the default (true) branch and triggering false-positive
  violations. Added `case token.AND_NOT: return Value{Raw: xi &^ yi}` after the existing
  `token.XOR` case.

  Two new integration tests: `and_not` (false-positive canary using `&^` result in
  comparison), `and_not_valid` (idiomatic bit-flag clearing patterns).

- **Complex number support: `real`/`imag`/`complex` builtins + `complex128` arithmetic**
  (issue #141): Four related gaps resolved:
  - `constToValue`: added `case constant.Complex:` to convert `go/constant` complex literals
    to `complex128` using `constant.Real`/`constant.Imag` + `constant.Float64Val`.
  - `execBuiltin`: added `"real"`, `"imag"`, and `"complex"` cases to extract/construct
    `complex128` values.
  - `evalBinOp`: added a `complex128` arithmetic block supporting `+`, `-`, `*`, `/`, `==`,
    `!=` on complex operands.

  Two new integration tests: `complex_builtins` (round-trip `complex`/`real`/`imag`
  with false-positive canaries), `complex_arith` (add, sub, mul, equality on `complex128`).

## [0.43.0] - 2026-03-01

### Fixed

- **`len(map)` + `len(chan)` + `cap(chan)` returning `Value{}`** (issue #138): The `len` and
  `cap` builtins in `execBuiltin` only handled `*SliceValue` and `string`. For maps and
  channels the builtins returned `Value{}`, causing `evalBinOp` to produce a non-bool result
  for comparisons like `len(m) == 0`; `ssa.If` then took the default (true) branch for any
  condition, inserting false-positive violations inside "empty" guards.

  Added three cases:
  - `case map[interface{}]Value:` → `int64(len(sv))` for `len`
  - `case ChanID:` → `int64(ch.pendingCount)` for `len`
  - `case ChanID:` → `int64(ch.capacity)` for `cap`

  Two new integration tests: `len_map_chan` (false-positive canary with non-empty
  map/channel), `len_map_chan_zero` (genuinely-empty cases).

- **Integer truncation in `convertValue`** (issue #139): Integer-to-integer conversions
  (e.g. `int8(300)`) passed through unchanged, returning the full 64-bit value instead of
  applying Go's well-defined bit-width truncation. This caused programs relying on
  wrap-around semantics to take incorrect control-flow branches.

  Added `int → int` case inside the existing `if srcIsBasic && dstIsBasic` block:
  switches on `dstBasic.Kind()` and applies the appropriate Go cast (`int8(n)`, `uint8(n)`,
  etc.), then rewraps as `int64`. Key example: `int8(256) == 0` now evaluates to `true`.

  Two new integration tests: `int_truncate` (uses `int8(256)=0` canary — OOB fires only if
  truncation is skipped), `int_truncate_valid` (widening and small-value conversions, 0
  violations).

### Closes
- Issue #138: `len(map)` / `len(chan)` / `cap(chan)` incorrect
- Issue #139: integer truncation in `convertValue`

## [0.42.0] - 2026-03-01

### Added

- **`make(map[K]V, n)` negative size hint detection** (issue #136): `make(map[string]int, n)`
  where `n < 0` now reports a `make-invalid` violation. The Go runtime panics with
  "makemap: size out of range" for negative hints.

  `ssa.MakeMap.Reserve` is an optional operand that the interpreter previously ignored
  entirely. The handler now checks `Reserve != nil && toInt64(reserveVal) < 0`, recording
  `InvalidMakeArgError{Kind: "map-cap", Value: n}`. The existing `classifyError` path
  already maps `*InvalidMakeArgError` to category `"make-invalid"`, so no report changes
  were needed.

  Two new integration tests: `make_map_neg` (1 violation), `make_map_valid` (0 violations).

- **Range-over-array iteration** (issue #137): `for i, v := range [N]T{...}` now correctly
  executes the loop body N times. Previously `rangeIter.advance()` had no case for arrays
  and always returned `(false, {}, {})`, silently skipping all iterations and hiding any
  violations inside the loop body.

  Two changes:
  1. Added `arrayLen int` field to `rangeIter`; `ssa.Range` populates it when `inst.X`
     has type `[N]T` or `*[N]T` by extracting `types.Array.Len()`.
  2. `advance()` checks `ri.arrayLen > 0` first; yields indices 0…N−1 and returns key only
     (element values are loaded by the loop body via `ssa.Index`/`ssa.IndexAddr`).

  Four new integration tests: `make_map_neg`, `make_map_valid`, `range_array` (0 violations;
  the divide-by-zero at the end fires only if the loop was skipped — regression test for the
  silent-skip bug), `range_array_race` (1 data race violation from two sibling goroutines
  writing a shared counter inside range-over-array loops).

### Closes
- Issue #136: `make(map[K]V, n)` where n < 0
- Issue #137: range-over-array silent skip

## [0.41.0] - 2026-03-01

### Added

- **Slice element OOB against declared length** (issue #134): `s[i]` where `i >= len(s)`
  now reports an `out-of-bounds` violation (severity: ERROR), even when `i < cap(s)`.

  The `ssa.IndexAddr` handler for slices previously checked nil/nil-backing but relied on
  `handleIndexAddr` → `CheckAccess` for bounds, which validates against the allocation size
  (capacity × elemSize), not the declared length. A resliced or under-populated slice like
  `make([]int, 3, 10)` would silently allow access at index 7.

  A new `sliceOOB bool` flag (alongside `nilSlice` and `arrayOOB`) fires when
  `sv.Backing != nil && (indexVal < 0 || indexVal >= sv.Len)`, recording
  `OutOfBoundsError{AllocSize: sv.Len}` and setting `g.Panicked = true`.

  Two new integration tests: `slice_elem_oob` (1 violation), `slice_elem_valid` (0 violations).

- **`make([]T, len, cap)` with len > cap detection** (issue #135): `make([]int, 10, 3)`
  now reports a `make-invalid` violation. The `ssa.MakeSlice` handler previously clamped
  `capN = lenN` silently. The violation is recorded before clamping so execution continues
  conservatively. Uses existing `InvalidMakeArgError` with `Kind: "slice-len-gt-cap"`.

  As with negative-length tests, the Go compiler rejects constant-folded len>cap at compile
  time, so the integration test uses helper functions `makeLen()`/`makeCap()` to produce
  the values at runtime.

  Two new integration tests: `make_len_gt_cap` (1 violation), `make_len_eq_cap` (0 violations).

### Closes

- Issue #134 — Slice element OOB beyond declared length
- Issue #135 — `make([]T, len, cap)` with len > cap

## [0.40.0] - 2026-03-01

### Added

- **Array pointer bounds detection** (issue #133): indexing `p[i]` where `p` is `*[N]T`
  and `i >= N` or `i < 0` now reports an `out-of-bounds` violation (severity: ERROR).

  The `ssa.IndexAddr` handler for the `*types.Pointer → *types.Array` case already
  computed `elemSize` from `arr.Elem()` but never used `arr.Len()` for a bounds check.
  A new `arrayOOB bool` flag (mirroring `nilSlice`) accumulates the OOB state and
  short-circuits `handleIndexAddr` when set.

  Two new integration tests: `array_index_oob` (1 violation), `array_index_valid`
  (0 violations). The OOB test uses `wantCategory: "out-of-bounds"` (report-category
  style, enabled by #132).

### Fixed

- **Test framework: `wantCategory` now matches report category OR error message substring**
  (issue #132): previously `wantCategory` was checked only via
  `strings.Contains(v.Error(), wantCategory)`, requiring test authors to know the exact
  wording of error messages (e.g. `"nil pointer"` instead of `"nil-pointer-deref"`).
  This caused a subtle failure in v0.39.0's `fieldaddr_nil_struct` test.

  `report.CategoryFor(err error) string` is now exported from `pkg/report`. The test
  `wantCategory` check now accepts either style:
  - Legacy: `wantCategory: "nil pointer"` — substring of `v.Error()`
  - Preferred: `wantCategory: "nil-pointer-deref"` — exact match of `report.CategoryFor(v)`

### Closes

- Issue #132 — Test framework category check inconsistency
- Issue #133 — Array pointer out-of-bounds detection

## [0.39.0] - 2026-03-01

### Added

- **`FieldAddr` nil struct pointer detection** (issue #130): accessing a field on a nil
  struct pointer (`var p *T; _ = p.Field`) now reports a `nil-pointer-deref` violation
  (severity: ERROR).

  `handleFieldAddr` previously returned `Value{}` silently when `base.Provenance == nil`,
  making no distinction between a nil pointer and an opaque/external value. The fix applies
  the same `base.Raw == nil` guard already present in `handleLoad`/`handleStore`: only a
  truly nil base (both `Raw` and `Provenance` are nil) triggers the violation; opaque
  values from stdlib intercepts (where `Raw == struct{}{}`) are correctly left alone.

  Two new integration tests: `fieldaddr_nil_struct` (1 violation), `fieldaddr_valid`
  (0 violations).

- **`unsafe.String` argument validation** (issue #131): `unsafe.String(ptr *byte, len)`
  (Go 1.20+) now validates its arguments, matching the behavior added for `unsafe.Slice`
  in v0.38.0.

  - `len < 0` → reports `InvalidUnsafeArgError{Arg: "len"}`, category `"unsafe-slice"`
  - `ptr == nil && len != 0` → reports `InvalidUnsafeArgError{Arg: "ptr"}`, category `"unsafe-slice"`
  - `ptr == nil && len == 0` → valid, returns opaque string

  A new `case "String"` in `execBuiltin` handles this. Note: the Go compiler rejects
  constant negative lengths at compile time, so tests use a helper function to produce
  a runtime-negative value.

  Three new integration tests: `unsafe_string_neg` (1 violation), `unsafe_string_nil`
  (1 violation), `unsafe_string_valid` (0 violations).

### Closes

- Issue #130 — `FieldAddr` nil struct pointer dereference
- Issue #131 — `unsafe.String` negative length and nil pointer

## [0.38.0] - 2026-02-28

### Added

- **`unsafe.Slice` negative length detection** (issue #128): `unsafe.Slice(ptr, n)` where
  `n < 0` now reports an `unsafe-slice` violation (severity: ERROR).

  At runtime Go panics with `"unsafe.Slice: len out of range"`. The interpreter previously
  created a `SliceValue` with a negative `Len`/`Cap`, silently masking the bug. A new check
  in `execBuiltin` case `"Slice"` fires `InvalidUnsafeArgError{Arg: "len"}` before the
  `SliceValue` is constructed.

  New integration test: `unsafe_slice_neg` (1 violation).

- **`unsafe.Slice` nil pointer detection** (issue #129): `unsafe.Slice(nil, n)` where
  `n != 0` now reports an `unsafe-slice` violation (severity: ERROR).

  At runtime Go panics with `"unsafe.Slice: ptr is nil"`. The interpreter previously
  returned `Value{}` silently when the pointer's `Provenance == nil`. A new check
  fires `InvalidUnsafeArgError{Arg: "ptr"}` before the silent return.

  New integration tests: `unsafe_slice_nil` (1 violation), `unsafe_slice_valid` (0 violations).

### New Error Type

- `shadow.InvalidUnsafeArgError{Op, Arg, Value, Site, GID}`: covers both `unsafe.Slice`
  argument violations (negative `len` and nil `ptr` with non-zero `len`). Classified as
  category `"unsafe-slice"` in report output.

### Closes

- Issue #128 — `unsafe.Slice` negative length
- Issue #129 — `unsafe.Slice` nil pointer with non-zero length

## [0.37.0] - 2026-02-28

### Added

- **Nil slice element access detection** (issue #126): `s[i]` where `s` is a nil
  slice now reports an `out-of-bounds` violation (severity: ERROR).

  A nil slice (`var s []T`) has `Len=0, Cap=0` and no backing allocation. In Go,
  accessing any element panics: `"runtime error: index out of range [0] with length 0"`.
  Previously Giri silently returned `Value{}` or reported an unrelated "nil pointer
  dereference" at the subsequent dereference. The fix is in `ssa.IndexAddr` (the SSA
  instruction used for slice element addresses): when the base type is a slice and
  `base.Raw == nil` (uninitialized) or `base.Raw.(*SliceValue).Backing == nil` (nil
  slice value), the error is reported immediately and accurately.

  Two new integration tests: `nil_slice_index` (1 violation), `slice_index_valid`
  (0 violations).

- **Unlock of unlocked mutex detection** (issue #127): `sync.Mutex.Unlock()` and
  `sync.RWMutex.RUnlock()` when the mutex is not locked now report a `mutex-unlock`
  violation (severity: ERROR).

  New error type in `pkg/shadow`: `MutexUnlockError{Op, Site, GID}`.

  In Go, calling `Unlock()` on a mutex that is not locked panics at runtime:
  `"sync: unlock of unlocked mutex"`. The `mutexState.locked` field already tracked
  lock state; this release adds a check before the unlock logic fires. The goroutine
  is marked Panicked to match real Go behavior.

  Two new integration tests: `mutex_unlock_unowned` (1 violation — double-unlock),
  `mutex_unlock_valid` (0 violations — correct lock/unlock pattern).
  137 total integration tests.

### Fixed

- `TryRLock` intercept now sets `ms.locked = true` (mirroring `TryLock`) so a
  subsequent `RUnlock` after a successful `TryRLock` does not false-positive as
  "unlock of unlocked mutex".

## [0.36.0] - 2026-02-28

### Added

- **String index out-of-bounds detection** (issue #124): `s[i]` where `i < 0` or
  `i >= len(s)` now reports an `out-of-bounds` violation (severity: ERROR).

  Unlike slice indexing (which passes through the shadow memory allocator's
  `CheckAccess`), strings are stored as raw Go strings in Giri with no backing
  allocation, so bounds must be checked explicitly. In Go this panics:
  `"runtime error: index out of range [N] with length M"`.

  Two new integration tests: `string_index_oob` (1 violation), `string_index_valid`
  (0 violations).

- **Negative shift count detection** (issue #125): `x << n` or `x >> n` where
  `n < 0` now reports a `negative-shift` violation (severity: ERROR).

  New error type in `pkg/shadow`: `NegativeShiftError{Count, Site, GID}`.

  In Go 1.13+, shifting by a negative runtime-determined value panics:
  `"runtime error: negative shift count"`. Previously Giri silently converted
  `n` to `uint`, producing an enormous shift. The goroutine is now marked
  Panicked to halt execution, matching real Go behavior.

  Two new integration tests: `negative_shift` (1 violation), `valid_shift`
  (0 violations). 133 total integration tests.

## [0.35.0] - 2026-02-28

### Added

- **Nil channel operation detection** (issue #122): Giri now reports a `nil-channel`
  violation (severity: ERROR) when a nil channel is used in any of these positions:
  - `close(nil)` — panics in Go: "close of nil channel"
  - Send on nil (`nil <- v`) — blocks forever in Go (deadlock)
  - Receive from nil (`<-nil`) — blocks forever in Go (deadlock)

  New error type in `pkg/shadow`: `NilChannelError{Op, Site, GID}` where `Op` is
  `"close"`, `"send"`, or `"receive"`.

  Three new integration tests: `nil_channel_close`, `nil_channel_send`,
  `nil_channel_recv` (1 violation each).

- **`make()` negative argument detection** (issue #123): Giri now reports a
  `make-invalid` violation (severity: ERROR) when `make()` is called with a negative
  length or capacity argument. In Go this panics at runtime:
  - `make([]T, -1)` → "makeslice: len out of range"
  - `make([]T, 0, -1)` → "makeslice: cap out of range"
  - `make(chan T, -1)` → "makechan: size out of range"

  New error type in `pkg/shadow`: `InvalidMakeArgError{Kind, Value, Site, GID}` where
  `Kind` is `"slice-len"`, `"slice-cap"`, or `"chan-cap"`.

  Two new integration tests: `make_invalid_len` (1 violation), `make_valid`
  (0 violations). 129 total integration tests.

## [0.34.0] - 2026-02-28

### Added

- **Context cancel leak detection** (issue #120): Giri now tracks cancel functions
  returned by `context.WithCancel`, `context.WithTimeout`, and `context.WithDeadline`.
  If a cancel function is never called before the program exits, Giri reports a
  `context-cancel-leak` violation (severity: WARNING). Calling cancel via `defer cancel()`
  or directly suppresses the report.

  New error type in `pkg/shadow`: `ContextCancelLeakError{Site, GID}`.

  Implementation: `cancelFuncID` (opaque value returned by intercepted context functions),
  `DeferredCall.DynCallVal` (stores non-closure dynamic defer targets like `defer cancel()`),
  `Interpreter.newCancelFunc/callCancelFunc` helpers, `Finish()` leak check.

  Two new integration tests: `context_cancel_ok` (0 violations), `context_cancel_leak`
  (1 violation). 123 total integration tests.

- **HTML report format** (issue #121): `-format html` produces a self-contained HTML
  report with inline CSS — no external resources required. Features include:
  - Color-coded severity badges (ERROR=red, WARNING=yellow, INFO=blue)
  - Collapsible stack traces via `<button>` toggle
  - Summary bar with per-severity and per-category counts
  - Replay seed display for PCT violations
  - Works identically in CI and local review

  New constant: `report.FormatHTML = 3`.
  New CLI usage: `giri -format html ./... > giri-report.html`

### Fixed

- `executeDeferred` now handles non-closure dynamic defer targets (e.g. `defer cancel()`)
  via the new `DeferredCall.DynCallVal` field. Previously, dynamic defers that were not
  closures were silently discarded.

## [0.33.0] - 2026-02-28

### Added

- **`giri -test ./...` — test function analysis** (issue #118): Run Giri directly on
  existing `TestXxx(*testing.T)` functions without writing standalone `package main`
  programs. Giri discovers test functions, runs each through the interpreter with an
  opaque `*testing.T`, and reports violations tagged with the test name.

  New public API in `pkg/interpreter`:

  ```go
  // TestFunc identifies a single TestXxx function.
  type TestFunc struct {
      Name string
      Fn   *ssa.Function
  }

  // TestRunResult holds the result of running one test function.
  type TestRunResult struct {
      Name       string
      Violations []error
      MemStats   shadow.MemoryStats
  }
  func (r *TestRunResult) Passed() bool

  // RunTests runs each function in prog.TestFuncs independently.
  func RunTests(prog *Program, config Config) []*TestRunResult
  ```

  New field on `Program`:

  ```go
  TestFuncs []TestFunc // populated by ssautil.LoadTestPrograms
  ```

  New `ssautil` function:

  ```go
  func LoadTestPrograms(patterns []string) ([]*interpreter.Program, error)
  ```

  `isTestFunc` validates that the function has signature `func(*testing.T)` (not
  just a name prefix), so `TestHelper(t, x)` style helpers are correctly excluded.

  CLI: `-test` flag; stderr shows `--- PASS: TestFoo` / `--- FAIL: TestFoo (N violation(s))`.

  Integration test: `test_mode` package with `TestSafeAdd` (0 violations) and
  `TestCounterRace` (1 data-race violation).

- **PCT replay seeds** (issue #119): When `RunN` finds a violation during its
  multi-run PCT sweep, it tags the violation with the random seed that triggered it.

  - New field `ReproSeed int64` on `ViolationWithStack`; set by `RunN` on first
    discovery for each unique violation.
  - New method `ReproSeedValue() int64` on `ViolationWithStack` (satisfies the
    `reproSeeder` interface in `pkg/report` without an import cycle).
  - New field `ReproSeed int64` on `Finding` (JSON key `repro_seed`).
  - Text report prints `replay: giri -strategy pct -seed <N> ./...` for any
    finding with a non-zero seed, turning PCT from a one-shot oracle into a
    reproducible debugger.

### Closes

- #118 (`giri -test` test function analysis), #119 (PCT replay seeds)

## [0.32.0] - 2026-02-28

### Added

- **Project-level `.giri.json` configuration file** (issue #115): Commit team
  settings to the repository instead of duplicating flags in every CI script,
  Makefile, and developer README.

  Fields mirror CLI flags (`format`, `strategy`, `seed`, `runs`, `depth`,
  `race`, `unsafe`, `arena`, `init`, `verbose`, `max_steps`, `max_goroutines`).
  CLI flags always override file values. The file is loaded from the working
  directory at startup; a missing file is silently ignored.

  ```json
  {
    "format":   "sarif",
    "strategy": "pct",
    "runs":     100,
    "seed":     42,
    "race":     true,
    "unsafe":   true
  }
  ```

  Four unit tests cover: missing file, valid file, invalid JSON, and field
  application precedence.

- **`CONTRIBUTING.md`** (issue #116): Development setup, commit conventions,
  PR workflow, and step-by-step guides for adding stdlib intercepts and
  integration tests.

- **`SECURITY.md`** (issue #116): Responsible disclosure process via GitHub
  private security advisory, response timeline, and scope definition.

- **GitHub issue templates** (issue #117): Structured YAML forms for bug
  reports (`bug_report.yml`), feature requests (`feature_request.yml`), and
  questions (`question.yml`) in `.github/ISSUE_TEMPLATE/`.

### Changed

- **README**: Added `.giri.json` configuration reference table, Contributing
  section, corrected Phase 2 unsafe.Pointer Rules 5 & 6 status to checked,
  updated stdlib intercept package count (60+), updated integration test count
  (120+).

### Closes

- #115 (`.giri.json` config file), #116 (community health files), #117 (issue templates)

## [0.31.0] - 2026-02-27

### Added

- **Custom intercept API** (issue #113): Users can now model external,
  private, or generated-code package functions without modifying Giri's core.

  New public types in `pkg/interpreter`:

  ```go
  // InterceptFunc is called instead of executing the function body.
  type InterceptFunc func(args []Value) (Value, bool)

  // CustomIntercepts maps "pkgPath" → {"funcName" → InterceptFunc}.
  type CustomIntercepts map[string]map[string]InterceptFunc
  ```

  New field in `Config`:

  ```go
  // Intercepts are checked before built-in stdlib handlers, so they
  // can also override stdlib behavior.
  Intercepts CustomIntercepts
  ```

  Usage:

  ```go
  cfg := interpreter.DefaultConfig()
  cfg.Intercepts = interpreter.CustomIntercepts{
      "github.com/myco/mylib": {
          "Compute": func(args []interpreter.Value) (interpreter.Value, bool) {
              return interpreter.Value{Raw: int64(0)}, true
          },
      },
  }
  result := interpreter.Run(prog, cfg)
  ```

- **Integration test for custom intercepts** (issue #114):
  `testdata/integration/custom_intercept/` contains a `locallib` sub-package
  whose `Compute` and `MustAlloc` functions are intercepted via
  `Config.Intercepts` in the test, demonstrating the end-to-end flow. 120
  total integration tests.

## [0.30.0] - 2026-02-27

### Added

- **`io/fs` + `embed` intercepts** (issue #109): `handleFsCall` handles
  `io/fs` standalone functions (`ReadFile`, `ReadDir`, `Stat`, `WalkDir`,
  `Glob`, `Sub`, `ValidPath`, `FileInfoToDirEntry`) and `embed.FS` methods
  (`Open`, `ReadFile`, `ReadDir`), plus `fs.File`/`fs.DirEntry`/`fs.FileInfo`
  methods (`Name`, `IsDir`, `Type`, `Info`, `Mode`, `ModTime`, `Size`,
  `Sys`, `Read`, `Close`). `os.DirFS` added to `handleOSCall`. New
  integration test: `fs_embed`.

- **`archive/zip` + `archive/tar` intercepts** (issue #110):
  `handleArchiveCall` handles `zip.OpenReader`, `zip.NewReader`,
  `zip.NewWriter`, `*zip.Writer` create/copy/close methods, `*zip.Reader`
  open/decompress methods; and `tar.NewReader`, `tar.NewWriter`,
  `*tar.Reader.Next`/`Read`, `*tar.Writer.WriteHeader`/`Write`/`Flush`/`Close`,
  `tar.FileInfoHeader`. New integration test: `zip_archive`.

- **`mime` + `mime/multipart` intercepts** (issue #111): `handleMimeCall`
  handles `mime.TypeByExtension` (with concrete-string lookup for common
  extensions), `ExtensionsByType`, `AddExtensionType`, `FormatMediaType`,
  `ParseMediaType`, `WordEncoder.Encode`, `WordDecoder.Decode`/`DecodeHeader`;
  and `multipart.NewReader`/`NewWriter`, `*Reader.NextPart`/`NextRawPart`/
  `ReadForm`, `*Writer.CreateFormFile`/`CreateFormField`/`CreatePart`/
  `WriteField`/`Boundary`/`SetBoundary`/`FormDataContentType`/`Close`,
  `*Part.Read`/`FileName`/`FormName`. New integration test: `mime_multipart`.

- **`crypto/aes` + `crypto/cipher` + `crypto/hmac` intercepts** (issue #112):
  `handleSymCryptoCall` handles `aes.NewCipher` → `(cipher.Block, nil)`;
  `cipher.NewGCM`/`NewGCMWithNonceSize`/`NewGCMWithTagSize` → `(AEAD, nil)`,
  `cipher.NewCTR`/`NewOFB`/`NewCFBEncrypter`/`NewCFBDecrypter` → `Stream`,
  `cipher.NewCBCEncrypter`/`NewCBCDecrypter` → `BlockMode`,
  AEAD `Seal`/`Open`/`NonceSize`/`Overhead`, BlockMode `CryptBlocks`,
  Stream `XORKeyStream`; and `hmac.New` → `hash.Hash`, `hmac.Equal`,
  HMAC `Write`/`Sum`/`Reset`/`Size`/`BlockSize`. New integration test:
  `aes_cipher`.

- 4 new integration tests (119 total).

## [0.29.0] - 2026-02-27

### Added

- **golangci-lint v2 configuration** (issue #105): `.golangci.yml` enables
  `govet`, `staticcheck`, `ineffassign`, `misspell`, `revive`, `gocyclo`,
  `unconvert`, `errorlint`, `nilerr`, `unused`; `gofmt` formatter; exclusion
  rules for test files, testdata/, and large SSA dispatch functions.
  Updated `.github/workflows/ci.yml` with a dedicated `lint` job using
  `golangci/golangci-lint-action@v6`.

- **Fuzz tests** (issue #106): 5 new fuzz targets covering core hot paths:
  - `pkg/shadow`: `FuzzAllocateCheckAccess`, `FuzzMarkInitializedCheckAccess`,
    `FuzzDerivePointer` — fuzz allocation/free/bounds sequences.
  - `pkg/interpreter`: `FuzzExecStdlibCall` (stdlib dispatch with random
    pkg/name pairs), `FuzzToInt64` (value conversion).
  - Seed-corpus-only run added to CI.
  - **Bug found by fuzzer**: `bytes.Join` panicked on nil args (index OOB).
    Fixed guard in `handleBytesCall`.

- **Benchmark tests** (issue #107): 11 new benchmarks covering hot paths:
  - `pkg/shadow`: `BenchmarkAllocate`, `BenchmarkCheckAccessValid`,
    `BenchmarkCheckAccessOOB`, `BenchmarkMarkInitialized`,
    `BenchmarkAllocateFree`, `BenchmarkCheckAccessContended`.
  - `pkg/detector`: `BenchmarkRaceDetectorNoRace`,
    `BenchmarkRegistryCheckAccess`.
  - `pkg/interpreter`: `BenchmarkStdlibDispatchHit`,
    `BenchmarkStdlibDispatchMiss`, `BenchmarkToInt64`.

- **Expanded unit tests** (issue #108): new coverage for previously-untested
  paths:
  - `pkg/shadow`: `Poison`, `TrackPointer`/`GetProvenance`, `GetArena`,
    `LiveArenas`, `LiveAllocations`, `Stats.String` (coverage: 67% → 84%).
  - `pkg/report`: all 12 `classifyError` branches, `unsupported Format`,
    text/no-violations path, summary counts, JSON schema fields, stack-trace
    rendering (coverage: 63% → 76%).
  - `pkg/detector`: `BoundsDetector.CheckFinalize`, `RaceDetector.CheckFinalize`,
    `UnsafeDetector.RecordReflectConversion`/`ClearAllUintptrConversions`,
    `DefaultRegistry.List`/`CheckAccess`/`Finalize` (coverage: 59% → 80%).
  - `internal/ssautil`: 3 new tests for `ParseSuppressions` (coverage: 0% → 12%).

### Fixed

- **Lint issues resolved** (issue #105):
  - `pkg/interpreter/stdlib.go`: `strings.Title` → `strings.ToTitle` (SA1019),
    `runtime.GOROOT()` → `os.Getenv("GOROOT")` (SA1019), 4 misspellings fixed.
  - `pkg/interpreter/interpreter.go`: `gofmt` alignment fix.
  - `pkg/detector/detector.go`: doc comments added to all exported methods;
    `BoundsDetector.CheckFinalize` / `RaceDetector.CheckFinalize` unused params
    now use blank identifier `_`.
  - `pkg/scheduler/scheduler.go`: doc comments added to all exported functions
    and interface-implementation methods; unused `gid` params use `_`.
  - `pkg/report/report.go`: `fmt.Fprintf`/`Fprintln` errors now propagated via
    `textWriter` helper; `classifyError` type switch annotated with
    `//nolint:errorlint` (errors are pre-unwrapped at call site).
  - `internal/ssautil/loader.go`: `fn.WriteTo` return now explicitly discarded.
  - 4 `gofmt`-only test-data files reformatted.

## [0.28.0] - 2026-02-27

### Added

- **`crypto/tls` intercepts** (issue #101): New `handleTLSCall` covers
  `Dial`/`DialWithDialer` → (`*Conn`, nil); `Client`/`Server` → opaque;
  `Listen`/`NewListener` → (`*Listener`, nil); `LoadX509KeyPair`/`X509KeyPair` →
  (opaque, nil); `*Conn` methods: `Read`/`Write` → (n, nil), `Close`/`Handshake`/
  `VerifyHostname` → nil, `ConnectionState` → opaque, `RemoteAddr`/`LocalAddr` →
  opaque, `SetDeadline`/`SetReadDeadline`/`SetWriteDeadline` → nil.
  Integration test: `tls_dial`.

- **`database/sql` intercepts** (issue #102): New `handleSQLCall` covers
  `Open` → (`*DB`, nil); `Named` → opaque; `*DB`: `Query`/`QueryContext` →
  (`*Rows`, nil), `QueryRow`/`QueryRowContext` → `*Row`, `Exec`/`ExecContext` →
  (Result, nil), `Prepare`/`PrepareContext` → (`*Stmt`, nil), `Begin`/`BeginTx` →
  (`*Tx`, nil), `Ping`/`Close` → nil; `*Rows`: `Next` → false, `Scan` → nil,
  `Err` → nil, `Close` → nil, `Columns` → []; `*Row`: `Scan` → nil;
  `*Tx`: `Commit`/`Rollback`/`Exec`/`Query` → appropriate zero values;
  `Result.LastInsertId`/`RowsAffected` → (0, nil). Integration test: `sql_query`.

- **`strings.NewReader` + `*strings.Reader` method intercepts** (issue #103):
  `NewReader` → opaque; `Read`/`ReadAt` → (n, nil), `ReadByte` → (0, nil),
  `ReadRune` → (0, 1, nil), `UnreadByte`/`UnreadRune` → nil, `Seek` → (0, nil),
  `Size`/`Len` → 0, `WriteTo` → (0, nil). Added to existing `handleStringsCall`.

- **`bytes.NewReader`, `bytes.NewBuffer`, `bytes.NewBufferString` + method intercepts**
  (issue #103): New constructors return opaque; `*bytes.Reader` methods:
  `Read`/`ReadAt` → (n, nil), `Seek` → (0, nil), `Size` → 0. Added to existing
  `handleBytesCall`. Integration test: `strings_reader` (covers both packages).

- **`testing.T` method intercepts** (issue #104): New `handleTestingCall` covers
  `Fatal`/`Fatalf`/`FailNow` → marks goroutine Panicked; `Run` → probes callback
  fn with sentinel `*testing.T` (uses real gid); `Log`/`Logf`/`Error`/`Errorf`/
  `Skip`/`Skipf`/`SkipNow` → noop; `Helper`/`Parallel`/`Cleanup` → noop;
  `Failed`/`Skipped` → false; `Name` → ""; `TempDir` → "/tmp".
  Integration test: `testing_helper`.

## [0.27.0] - 2026-02-27

### Added

- **`encoding/binary` intercepts** (issue #97): New `handleBinaryCall` covers
  `Read`/`Write` (noop, nil error), `Size` (returns 8), `PutUvarint`/`PutVarint`
  (returns bytes written), `Uvarint`/`Varint` (returns (0, 1)),
  `AppendUvarint`/`AppendVarint` (returns input slice); ByteOrder method calls
  `Uint16`/`Uint32`/`Uint64`/`PutUint16`/`PutUint32`/`PutUint64`/`String`.
  Integration test: `binary_readwrite`.

- **`hash/crc32`, `hash/fnv`, `hash/adler32` intercepts** (issue #98): New
  `handleHashExtCall` (shared across all three packages) covers constructors
  (`NewIEEE`, `New`, `NewCastagnoli`, `New32`, `New32a`, `New64`, `New64a`,
  `New128`, `New128a`, `MakeTable`), package-level helpers (`Checksum`,
  `ChecksumIEEE`), and all `hash.Hash`/`hash.Hash32`/`hash.Hash64` methods
  (`Write` → (n, nil), `Sum` → input slice, `Sum32`/`Sum64` → 0, `Reset`,
  `Size` → 4, `BlockSize` → 64). Integration test: `hash_crc32`.

- **`container/list`, `container/heap`, `container/ring` intercepts** (issue #99):
  New `handleContainerCall` dispatches by package path:
  - `container/list`: `New` → opaque; `PushFront`/`PushBack`/`InsertBefore`/
    `InsertAfter` → opaque element; `Init`/`Remove`/`MoveToFront`/`MoveToBack`/
    `MoveBefore`/`MoveAfter`/`PushFrontList`/`PushBackList` → noop;
    `Front`/`Back` → opaque; `Len` → 0; `(*Element).Next`/`Prev` → opaque
  - `container/heap`: `Init`/`Fix`/`Push` → noop; `Pop`/`Remove` → opaque
  - `container/ring`: `New` → opaque; `Next`/`Prev`/`Move`/`Link`/`Unlink` →
    opaque; `Len` → 0; `Do` → probes callback with sentinel (uses real `gid`)
  Integration test: `container_list`.

- **`math/big` intercepts** (issue #100): New `handleMathBigCall` covers:
  - Constructors: `NewInt`/`NewFloat`/`NewRat` → opaque
  - Arithmetic/set methods shared by `*Int`, `*Float`, `*Rat`: `Add`/`Sub`/
    `Mul`/`Div`/`Mod`/`Rem`/`Abs`/`Neg`/`Inv`/bitwise ops/`Exp`/`GCD`/`Sqrt`/
    `Set*` methods → return receiver (args[0])
  - Extractors: `Int64`/`Uint64` → 0; `BitLen`/`Bit` → 0; `Bytes` → [];
    `Text`/`String` → "0"; `Float64`/`Float32` → (0.0, 0)
  - Comparisons: `Cmp`/`CmpAbs` → 0; `Sign` → 1; `ProbablyPrime` → true
  - `*big.Float`: `Prec`/`Mode`/`Acc`/`MinPrec`; `IsInf`/`IsNaN` → false
  - `*big.Rat`: `Num`/`Denom` → opaque; `FloatString` → "0";
    `RatString` → "0/1"; `IsInt` → false
  Integration test: `math_big`.

## [0.26.0] - 2026-02-27

### Added

- **`time` extras** (issue #93): Extended `handleTimeCall` with full coverage:
  - `time.Tick` — returns pre-populated channel (like `time.After`)
  - `time.NewTicker`/`time.NewTimer` — return opaque values (were previously
    returning nil `Value{}`); `Ticker.Stop`/`Reset` and `Timer.Stop`/`Reset`
    return `false` (bool)
  - `time.Now` — returns opaque `time.Time` (was nil)
  - `time.Since`/`time.Until` — return `int64(1)` nanosecond (non-zero duration)
  - `time.ParseDuration` — returns `(1ns, nil)`
  - `time.Parse`/`ParseInLocation` — returns `(opaque time.Time, nil)`
  - `(time.Time)` methods: `Add`/`Round`/`Truncate`/`In`/`UTC`/`Local` → opaque;
    `Sub` → `int64(0)`; `Before`/`Equal`/`IsZero` → `false`; `Format`/`String`
    → `""`; year/month/day/... → `int64(0)`; `Zone` → `("", 0)`
  - `(time.Duration)` methods: `Hours`/`Minutes`/`Seconds` → `float64(0)`;
    `Milliseconds`/`Microseconds`/`Nanoseconds` → `int64(0)`
  - Disambiguation: `time.After(d)` vs `(time.Time).After(u)` by arg count;
    `time.Unix(sec, nsec)` vs `(time.Time).Unix()` by arg count
  Integration test: `time_ticker`.

- **`*os.File` method intercepts** (issue #94): `handleOSCall` now intercepts
  method calls on the `*os.File` returned by `Open`/`Create`/`OpenFile`:
  - `Open`/`Create`/`CreateTemp`/`OpenFile` now return `(opaque *File, nil)`
    instead of `(nil, nil)` so method calls dispatch back to this intercept
  - `Read`/`ReadAt` → `(len(p), nil)` (pessimistic); `Write`/`WriteAt` →
    `(len(p), nil)`; `WriteString` → `(len(s), nil)`
  - `Close`/`Sync`/`Chmod`/`Chown`/`Truncate`/`Chdir` → `nil error`
  - `Stat` → `(opaque os.FileInfo, nil)`; `Seek` → `(0, nil)`
  - `Name` → `""`; `Fd` → `3`
  - `ReadDir`/`Readdirnames`/`Readdir` → `([], nil)`
  Integration test: `os_file_rw`.

- **`net/http` client intercepts** (issue #95): New `handleHTTPCall` covers the
  HTTP client API surface:
  - Package-level: `Get`/`Post`/`Head`/`PostForm` → `(*Response, nil)`;
    `NewRequest`/`NewRequestWithContext` → `(*Request, nil)`
  - `(*http.Client).Do` → `(*Response, nil)`;
    `ListenAndServe`/`ListenAndServeTLS` → noop
  - `NewServeMux` → opaque; `Handle`/`HandleFunc`/`ServeHTTP` → noop
  - `Error`/`Redirect`/`NotFound`/`ServeFile` → noop; `StatusText` → `""`
  - Request methods: `FormValue`/`PostFormValue` → `""`; `WithContext`/`Clone`
    → opaque; `ParseForm` → noop
  Note: direct field access on `*http.Response` (e.g. `resp.StatusCode`) goes
  through SSA `FieldAddr` on an opaque value and cannot be resolved; tests should
  use function-call patterns only. Integration test: `http_client`.

- **`os/signal` intercepts** (issue #96): New `handleSignalCall`:
  - `signal.Notify(ch, sig...)` — pre-populates the channel with a pending value
    (like `time.After`) so goroutines waiting on it proceed without triggering
    goroutine-leak violations; marks `channelSenders`
  - `signal.Stop`/`Ignore`/`Reset` → noop
  - `signal.NotifyContext` → `(opaque ctx, opaque cancelFunc)`
  Integration test: `signal_notify`.

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

[Unreleased]: https://github.com/scttfrdmn/giri/compare/v0.31.0...HEAD
[0.31.0]: https://github.com/scttfrdmn/giri/compare/v0.30.0...v0.31.0
[0.30.0]: https://github.com/scttfrdmn/giri/compare/v0.29.0...v0.30.0
[0.29.0]: https://github.com/scttfrdmn/giri/compare/v0.9.0...v0.29.0
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
