// Package interpreter - stdlib.go intercepts standard library calls that would
// otherwise return opaque zero values (#42, #43).
//
// When the interpreter encounters a call to strings.*, strconv.*, or fmt.*,
// the callee has no interpretable SSA body (Blocks == nil). Without intercepts,
// execCall returns Value{} for these, causing downstream branches to always
// take the false/zero path and missing reachable violations.
//
// For concrete arguments the real Go stdlib function is called directly.
// For non-concrete arguments (Value{Raw: nil}) a pessimistic non-zero return is
// used: bool predicates return true, string-returning functions return a
// non-empty sentinel, numeric functions return a non-zero sentinel.
package interpreter

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"math"
	"math/rand"
	"net"
	"net/url"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/scttfrdmn/giri/pkg/shadow"
	"golang.org/x/tools/go/ssa"
)

// execStdlibCall intercepts standard library function calls in packages
// "strings", "strconv", "fmt", "time", "os", "math/rand", "bytes",
// "errors", "sort", "sync/atomic", "io", "bufio", "log",
// "encoding/hex", "encoding/base64", "encoding/xml", "encoding/csv",
// "crypto/rand", "crypto/md5", "crypto/sha1", "crypto/sha256",
// "path/filepath", "path", "net", "net/url", "text/template", "html/template",
// "reflect", "flag", "runtime", "os/exec", "compress/gzip", and
// "compress/zlib".
// Returns (result, true) when intercepted, (Value{}, false) otherwise.
//
// gid and site are required by handlers that invoke user callbacks
// (e.g. sort.Slice calls the less function via execFunction).
func (interp *Interpreter) execStdlibCall(gid int64, site, pkgPath, name string, args []Value) (Value, bool) {
	switch pkgPath {
	case "strings":
		return interp.handleStringsCall(name, args)
	case "strconv":
		return interp.handleStrconvCall(name, args)
	case "fmt":
		return interp.handleFmtCall(name, args)
	case "time":
		return interp.handleTimeCall(name, args)
	case "os":
		return interp.handleOSCall(name, args)
	case "math/rand":
		return interp.handleMathRandCall(name, args)
	case "bytes":
		return interp.handleBytesCall(name, args)
	case "errors":
		return interp.handleErrorsCall(name, args)
	case "sort":
		return interp.handleSortCall(gid, name, args, site)
	case "encoding/json":
		return interp.handleJSONCall(name, args)
	case "regexp":
		return interp.handleRegexpCall(gid, name, args, site)
	case "math":
		return interp.handleMathCall(name, args)
	case "unicode/utf8":
		return interp.handleUTF8Call(name, args)
	case "unicode":
		return interp.handleUnicodeCall(name, args)
	case "context":
		return interp.handleContextCall(name, args)
	case "sync/atomic":
		return interp.handleAtomicCall(name, args)
	case "io":
		return interp.handleIOCall(name, args)
	case "bufio":
		return interp.handleBufioCall(name, args)
	case "log":
		return interp.handleLogCall(gid, name, args)
	case "encoding/hex":
		return interp.handleHexCall(name, args)
	case "encoding/base64":
		return interp.handleBase64Call(name, args)
	case "crypto/rand":
		return interp.handleCryptoRandCall(name, args)
	case "crypto/md5", "crypto/sha1", "crypto/sha256", "crypto/sha512":
		return interp.handleHashCall(pkgPath, name, args)
	case "path/filepath":
		return interp.handleFilepathCall(name, args)
	case "path":
		return interp.handlePathCall(name, args)
	case "net":
		return interp.handleNetCall(name, args)
	case "text/template", "html/template":
		return interp.handleTemplateCall(name, args)
	case "encoding/xml":
		return interp.handleXMLCall(name, args)
	case "encoding/csv":
		return interp.handleCSVCall(name, args)
	case "reflect":
		return interp.handleReflectCall(name, args)
	case "flag":
		return interp.handleFlagCall(name, args)
	case "runtime":
		return interp.handleRuntimeCall(name, args)
	case "net/url":
		return interp.handleNetURLCall(name, args)
	case "os/exec":
		return interp.handleExecCall(name, args)
	case "compress/gzip":
		return interp.handleGzipCall(name, args)
	case "compress/zlib":
		return interp.handleZlibCall(name, args)
	}
	return Value{}, false
}

// handleStringsCall models strings.* functions.
func (interp *Interpreter) handleStringsCall(name string, args []Value) (Value, bool) {
	s0, s0ok := stdlibArgString(args, 0)
	s1, s1ok := stdlibArgString(args, 1)

	switch name {
	case "Contains":
		if s0ok && s1ok {
			return Value{Raw: strings.Contains(s0, s1)}, true
		}
		return Value{Raw: true}, true // pessimistic: assume it contains
	case "HasPrefix":
		if s0ok && s1ok {
			return Value{Raw: strings.HasPrefix(s0, s1)}, true
		}
		return Value{Raw: true}, true
	case "HasSuffix":
		if s0ok && s1ok {
			return Value{Raw: strings.HasSuffix(s0, s1)}, true
		}
		return Value{Raw: true}, true
	case "EqualFold":
		if s0ok && s1ok {
			return Value{Raw: strings.EqualFold(s0, s1)}, true
		}
		return Value{Raw: true}, true
	case "ContainsAny", "ContainsRune":
		return Value{Raw: true}, true
	case "Index":
		if s0ok && s1ok {
			return Value{Raw: int64(strings.Index(s0, s1))}, true
		}
		return Value{Raw: int64(0)}, true // pessimistic: "found at position 0"
	case "LastIndex":
		if s0ok && s1ok {
			return Value{Raw: int64(strings.LastIndex(s0, s1))}, true
		}
		return Value{Raw: int64(0)}, true
	case "IndexByte", "IndexRune", "IndexAny":
		return Value{Raw: int64(0)}, true
	case "Count":
		if s0ok && s1ok {
			return Value{Raw: int64(strings.Count(s0, s1))}, true
		}
		return Value{Raw: int64(1)}, true // pessimistic: "found 1 occurrence"
	case "TrimSpace":
		if s0ok {
			return Value{Raw: strings.TrimSpace(s0)}, true
		}
		return Value{Raw: "x"}, true
	case "Trim":
		if s0ok && s1ok {
			return Value{Raw: strings.Trim(s0, s1)}, true
		}
		return Value{Raw: "x"}, true
	case "TrimLeft":
		if s0ok && s1ok {
			return Value{Raw: strings.TrimLeft(s0, s1)}, true
		}
		return Value{Raw: "x"}, true
	case "TrimRight":
		if s0ok && s1ok {
			return Value{Raw: strings.TrimRight(s0, s1)}, true
		}
		return Value{Raw: "x"}, true
	case "TrimPrefix":
		if s0ok && s1ok {
			return Value{Raw: strings.TrimPrefix(s0, s1)}, true
		}
		return Value{Raw: "x"}, true
	case "TrimSuffix":
		if s0ok && s1ok {
			return Value{Raw: strings.TrimSuffix(s0, s1)}, true
		}
		return Value{Raw: "x"}, true
	case "ToUpper":
		if s0ok {
			return Value{Raw: strings.ToUpper(s0)}, true
		}
		return Value{Raw: "X"}, true
	case "ToLower":
		if s0ok {
			return Value{Raw: strings.ToLower(s0)}, true
		}
		return Value{Raw: "x"}, true
	case "ToTitle":
		if s0ok {
			return Value{Raw: strings.ToTitle(s0)}, true
		}
		return Value{Raw: "X"}, true
	case "Replace":
		s2, s2ok := stdlibArgString(args, 2)
		if s0ok && s1ok && s2ok {
			n := -1
			if nv, ok := stdlibArgInt(args, 3); ok {
				n = int(nv)
			}
			return Value{Raw: strings.Replace(s0, s1, s2, n)}, true
		}
		return Value{Raw: "x"}, true
	case "ReplaceAll":
		s2, s2ok := stdlibArgString(args, 2)
		if s0ok && s1ok && s2ok {
			return Value{Raw: strings.ReplaceAll(s0, s1, s2)}, true
		}
		return Value{Raw: "x"}, true
	case "Repeat":
		if s0ok {
			n := 1
			if nv, ok := stdlibArgInt(args, 1); ok {
				n = int(nv)
			}
			return Value{Raw: strings.Repeat(s0, n)}, true
		}
		return Value{Raw: "x"}, true
	case "Split":
		if s0ok && s1ok {
			return Value{Raw: stringsToValues(strings.Split(s0, s1))}, true
		}
		return Value{Raw: []Value{{Raw: "x"}}}, true
	case "SplitN":
		if s0ok && s1ok {
			n, _ := stdlibArgInt(args, 2)
			return Value{Raw: stringsToValues(strings.SplitN(s0, s1, int(n)))}, true
		}
		return Value{Raw: []Value{{Raw: "x"}}}, true
	case "SplitAfter":
		if s0ok && s1ok {
			return Value{Raw: stringsToValues(strings.SplitAfter(s0, s1))}, true
		}
		return Value{Raw: []Value{{Raw: "x"}}}, true
	case "Fields":
		if s0ok {
			return Value{Raw: stringsToValues(strings.Fields(s0))}, true
		}
		return Value{Raw: []Value{{Raw: "x"}}}, true
	case "Join":
		// args[0] is []string (stored as []Value), args[1] is sep.
		if s1ok {
			if sl, ok := args[0].Raw.([]Value); ok {
				parts := make([]string, len(sl))
				for i, v := range sl {
					if s, ok := v.Raw.(string); ok {
						parts[i] = s
					} else {
						parts[i] = "?"
					}
				}
				return Value{Raw: strings.Join(parts, s1)}, true
			}
		}
		return Value{Raw: "x"}, true
	case "Map":
		// args[0] is the mapping func (uninterpretable); args[1] is the string.
		// Return the string as-is (identity approximation).
		if s1ok {
			return Value{Raw: s1}, true
		}
		return Value{Raw: "x"}, true
	case "Compare":
		if s0ok && s1ok {
			return Value{Raw: int64(strings.Compare(s0, s1))}, true
		}
		return Value{Raw: int64(0)}, true
	case "Cut":
		if s0ok && s1ok {
			before, after, found := strings.Cut(s0, s1)
			return Value{Raw: []Value{{Raw: before}, {Raw: after}, {Raw: found}}}, true
		}
		return Value{Raw: []Value{{Raw: "x"}, {Raw: "x"}, {Raw: true}}}, true
	case "NewReplacer":
		// Returns a *strings.Replacer; method calls on it are not modelable here.
		return Value{}, false

	// strings.Builder method calls (#79): receiver = args[0], other args follow.
	case "WriteString":
		// b.WriteString(s) (int, error)
		n := 0
		if s, ok := stdlibArgString(args, 1); ok {
			n = len(s)
		}
		return Value{Raw: []Value{{Raw: int64(n)}, {}}}, true
	case "WriteByte":
		// b.WriteByte(c byte) error
		return Value{}, true
	case "WriteRune":
		// b.WriteRune(r rune) (int, error)
		return Value{Raw: []Value{{Raw: int64(1)}, {}}}, true
	case "Write":
		// b.Write(p []byte) (int, error)
		return Value{Raw: []Value{{Raw: int64(0)}, {}}}, true
	case "String":
		// b.String() string
		return Value{Raw: ""}, true
	case "Len":
		// b.Len() int
		return Value{Raw: int64(0)}, true
	case "Cap":
		// b.Cap() int
		return Value{Raw: int64(0)}, true
	case "Reset":
		// b.Reset()
		return Value{}, true
	case "Grow":
		// b.Grow(n int)
		return Value{}, true
	}
	return Value{}, false
}

// handleStrconvCall models strconv.* functions.
func (interp *Interpreter) handleStrconvCall(name string, args []Value) (Value, bool) {
	switch name {
	case "Itoa":
		if n, ok := stdlibArgInt(args, 0); ok {
			return Value{Raw: strconv.Itoa(int(n))}, true
		}
		return Value{Raw: "0"}, true

	case "Atoi":
		if s, ok := stdlibArgString(args, 0); ok {
			n, err := strconv.Atoi(s)
			if err != nil {
				return Value{Raw: []Value{{Raw: int64(0)}, {Raw: err.Error()}}}, true
			}
			return Value{Raw: []Value{{Raw: int64(n)}, {}}}, true
		}
		// Non-concrete: return sentinel success (1, nil).
		return Value{Raw: []Value{{Raw: int64(1)}, {}}}, true

	case "FormatInt":
		if n, ok := stdlibArgInt(args, 0); ok {
			base := 10
			if b, ok2 := stdlibArgInt(args, 1); ok2 {
				base = int(b)
			}
			return Value{Raw: strconv.FormatInt(n, base)}, true
		}
		return Value{Raw: "0"}, true

	case "FormatUint":
		if n, ok := stdlibArgInt(args, 0); ok {
			base := 10
			if b, ok2 := stdlibArgInt(args, 1); ok2 {
				base = int(b)
			}
			return Value{Raw: strconv.FormatUint(uint64(n), base)}, true
		}
		return Value{Raw: "0"}, true

	case "FormatBool":
		if len(args) > 0 {
			if b, ok := args[0].Raw.(bool); ok {
				return Value{Raw: strconv.FormatBool(b)}, true
			}
		}
		return Value{Raw: "true"}, true

	case "ParseInt":
		if s, ok := stdlibArgString(args, 0); ok {
			base := 10
			if b, ok2 := stdlibArgInt(args, 1); ok2 {
				base = int(b)
			}
			bitSize := 64
			if bs, ok2 := stdlibArgInt(args, 2); ok2 {
				bitSize = int(bs)
			}
			n, err := strconv.ParseInt(s, base, bitSize)
			if err != nil {
				return Value{Raw: []Value{{Raw: int64(0)}, {Raw: err.Error()}}}, true
			}
			return Value{Raw: []Value{{Raw: n}, {}}}, true
		}
		return Value{Raw: []Value{{Raw: int64(1)}, {}}}, true

	case "ParseUint":
		if s, ok := stdlibArgString(args, 0); ok {
			base := 10
			if b, ok2 := stdlibArgInt(args, 1); ok2 {
				base = int(b)
			}
			bitSize := 64
			if bs, ok2 := stdlibArgInt(args, 2); ok2 {
				bitSize = int(bs)
			}
			n, err := strconv.ParseUint(s, base, bitSize)
			if err != nil {
				return Value{Raw: []Value{{Raw: uint64(0)}, {Raw: err.Error()}}}, true
			}
			return Value{Raw: []Value{{Raw: int64(n)}, {}}}, true
		}
		return Value{Raw: []Value{{Raw: int64(1)}, {}}}, true

	case "ParseFloat":
		if s, ok := stdlibArgString(args, 0); ok {
			bitSize := 64
			if bs, ok2 := stdlibArgInt(args, 1); ok2 {
				bitSize = int(bs)
			}
			f, err := strconv.ParseFloat(s, bitSize)
			if err != nil {
				return Value{Raw: []Value{{Raw: float64(0)}, {Raw: err.Error()}}}, true
			}
			return Value{Raw: []Value{{Raw: f}, {}}}, true
		}
		return Value{Raw: []Value{{Raw: float64(1.0)}, {}}}, true

	case "ParseBool":
		if s, ok := stdlibArgString(args, 0); ok {
			b, err := strconv.ParseBool(s)
			if err != nil {
				return Value{Raw: []Value{{Raw: false}, {Raw: err.Error()}}}, true
			}
			return Value{Raw: []Value{{Raw: b}, {}}}, true
		}
		return Value{Raw: []Value{{Raw: true}, {}}}, true

	case "FormatFloat":
		if f, ok := stdlibArgFloat(args, 0); ok {
			fmtByte := byte('g')
			if len(args) > 1 {
				switch v := args[1].Raw.(type) {
				case int32:
					fmtByte = byte(v)
				case int64:
					fmtByte = byte(v)
				}
			}
			prec := -1
			if p, ok2 := stdlibArgInt(args, 2); ok2 {
				prec = int(p)
			}
			bitSize := 64
			if bs, ok2 := stdlibArgInt(args, 3); ok2 {
				bitSize = int(bs)
			}
			return Value{Raw: strconv.FormatFloat(f, fmtByte, prec, bitSize)}, true
		}
		return Value{Raw: "0"}, true

	case "Quote":
		if s, ok := stdlibArgString(args, 0); ok {
			return Value{Raw: strconv.Quote(s)}, true
		}
		return Value{Raw: `"x"`}, true

	case "Unquote":
		if s, ok := stdlibArgString(args, 0); ok {
			u, err := strconv.Unquote(s)
			if err != nil {
				return Value{Raw: []Value{{Raw: ""}, {Raw: err.Error()}}}, true
			}
			return Value{Raw: []Value{{Raw: u}, {}}}, true
		}
		return Value{Raw: []Value{{Raw: "x"}, {}}}, true

	case "AppendInt", "AppendUint", "AppendFloat", "AppendBool", "AppendQuote":
		// Returns []byte; return the dst slice unchanged.
		if len(args) > 0 {
			return args[0], true
		}
		return Value{}, true
	}
	return Value{}, false
}

// handleFmtCall models fmt.* functions.
func (interp *Interpreter) handleFmtCall(name string, args []Value) (Value, bool) {
	switch name {
	case "Sprintf":
		if len(args) == 0 {
			return Value{Raw: ""}, true
		}
		if format, ok := stdlibArgString(args, 0); ok {
			return Value{Raw: fmt.Sprintf(format, valuesToInterfaces(args[1:])...)}, true
		}
		return Value{Raw: "<fmt.Sprintf>"}, true

	case "Errorf":
		if len(args) == 0 {
			return Value{Raw: fmt.Errorf("<fmt.Errorf>")}, true
		}
		if format, ok := stdlibArgString(args, 0); ok {
			return Value{Raw: fmt.Errorf(format, valuesToInterfaces(args[1:])...)}, true
		}
		return Value{Raw: fmt.Errorf("<fmt.Errorf>")}, true

	case "Sprint":
		return Value{Raw: fmt.Sprint(valuesToInterfaces(args)...)}, true

	case "Sprintln":
		return Value{Raw: fmt.Sprintln(valuesToInterfaces(args)...)}, true

	case "Println", "Printf", "Print", "Fprintln", "Fprintf", "Fprint":
		// Return (1, nil): 1 byte written, nil error (#65).
		// Callers checking err != nil take the non-error path; callers
		// checking n == 0 see a plausible non-zero byte count.
		return Value{Raw: []Value{{Raw: int64(1)}, {}}}, true

	case "Sscanf", "Sscan", "Sscanln":
		// Return (0, nil): 0 items scanned, nil error.
		return Value{Raw: []Value{{Raw: int64(0)}, {}}}, true
	}
	return Value{}, false
}

// stdlibArgString extracts a concrete string Value at index i.
func stdlibArgString(args []Value, i int) (string, bool) {
	if i >= len(args) {
		return "", false
	}
	s, ok := args[i].Raw.(string)
	return s, ok
}

// stdlibArgInt extracts a concrete integer Value at index i.
// Recognises the int family that toInt64 handles.
func stdlibArgInt(args []Value, i int) (int64, bool) {
	if i >= len(args) {
		return 0, false
	}
	switch args[i].Raw.(type) {
	case int64, int, int32, int16, int8, uint64, uint32, uint16, uint8, uint:
		return toInt64(args[i]), true
	}
	return 0, false
}

// stdlibArgFloat extracts a concrete float Value at index i.
func stdlibArgFloat(args []Value, i int) (float64, bool) {
	if i >= len(args) {
		return 0, false
	}
	switch v := args[i].Raw.(type) {
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case int64:
		return float64(v), true
	}
	return 0, false
}

// stringsToValues converts a []string to []Value.
func stringsToValues(ss []string) []Value {
	vs := make([]Value, len(ss))
	for i, s := range ss {
		vs[i] = Value{Raw: s}
	}
	return vs
}

// handleTimeCall models time.* functions (#45).
// time.After returns a channel that immediately has a value (simulates a fired timer).
// time.Sleep is a noop.
func (interp *Interpreter) handleTimeCall(name string, args []Value) (Value, bool) {
	switch name {
	case "After":
		// Create a buffered channel with capacity 1 and pre-populate it so that
		// any select case waiting on time.After fires immediately.
		chanID := interp.createChannel(1)
		if ch, ok := interp.channels[chanID]; ok {
			ch.hasPending = true
			ch.pendingCount = 1
		}
		interp.channelSenders[chanID] = true
		return Value{Raw: chanID}, true
	case "Sleep", "NewTimer", "NewTicker", "Since", "Now", "Unix":
		// Noop — no side effects the interpreter needs to model.
		return Value{}, true
	}
	return Value{}, false
}

// handleOSCall models os.* functions (#62).
// os.Exit is handled separately in execCall (it needs to stop all goroutines).
// This intercept covers environment and filesystem queries.
func (interp *Interpreter) handleOSCall(name string, args []Value) (Value, bool) {
	switch name {
	case "Getenv":
		// Return empty string for any env var — conservative but safe.
		return Value{Raw: ""}, true
	case "LookupEnv":
		// Return ("", false): key not found.
		return Value{Raw: []Value{{Raw: ""}, {Raw: false}}}, true
	case "Setenv", "Unsetenv":
		// Return nil error.
		return Value{}, true
	case "Getwd":
		// Return ("/tmp", nil) — a valid directory path.
		return Value{Raw: []Value{{Raw: "/tmp"}, {}}}, true
	case "MkdirAll", "MkdirTemp", "Remove", "RemoveAll", "Rename":
		// File-system mutations: noop with nil error.
		return Value{}, true
	case "Open", "Create", "OpenFile":
		// File operations: return (nil, nil) — callers checking the error get nil,
		// avoiding cascading panics; actual File methods are external and return zero.
		return Value{Raw: []Value{{}, {}}}, true
	case "ReadFile", "WriteFile":
		// Bulk I/O: return ([]byte{}, nil).
		return Value{Raw: []Value{{Raw: []Value{}}, {}}}, true
	}
	return Value{}, false
}

// handleMathRandCall models math/rand.* functions (#64).
// Uses the interpreter's per-run RNG (seeded from config.RandomSeed) for
// deterministic, reproducible values without interpreting stdlib internals.
func (interp *Interpreter) handleMathRandCall(name string, args []Value) (Value, bool) {
	rng := interp.rng
	if rng == nil {
		rng = rand.New(rand.NewSource(0))
	}
	switch name {
	case "Intn", "Int31n":
		if n, ok := stdlibArgInt(args, 0); ok && n > 0 {
			return Value{Raw: rng.Int63n(n)}, true
		}
		return Value{Raw: int64(0)}, true
	case "Int63n":
		if n, ok := stdlibArgInt(args, 0); ok && n > 0 {
			return Value{Raw: rng.Int63n(n)}, true
		}
		return Value{Raw: int64(0)}, true
	case "Int63":
		return Value{Raw: rng.Int63()}, true
	case "Int", "Int31":
		return Value{Raw: int64(rng.Int31())}, true
	case "Uint32":
		return Value{Raw: int64(rng.Uint32())}, true
	case "Uint64":
		return Value{Raw: int64(rng.Uint64())}, true
	case "Float64":
		return Value{Raw: rng.Float64()}, true
	case "Float32":
		return Value{Raw: float64(rng.Float32())}, true
	case "Seed":
		// Noop: we control the seed via config.RandomSeed.
		return Value{}, true
	case "New":
		// Return opaque Value — method calls on the returned *Rand are handled
		// via the same math/rand intercept (same package path).
		return Value{}, true
	case "NewSource":
		return Value{}, true
	case "Perm":
		if n, ok := stdlibArgInt(args, 0); ok && n >= 0 {
			vs := make([]Value, n)
			for i := range vs {
				vs[i] = Value{Raw: int64(i)}
			}
			return Value{Raw: vs}, true
		}
		return Value{Raw: []Value{}}, true
	case "Shuffle":
		// Noop: element ordering doesn't affect memory safety.
		return Value{}, true
	case "Read":
		// Return (n, nil) where n = length of the slice arg (if known).
		n := int64(0)
		if len(args) > 0 {
			if sv, ok := args[0].Raw.(*SliceValue); ok {
				n = int64(sv.Len)
			}
		}
		return Value{Raw: []Value{{Raw: n}, {}}}, true
	case "NormFloat64", "ExpFloat64":
		return Value{Raw: rng.NormFloat64()}, true
	}
	return Value{}, false
}

// handleBytesCall models bytes.* functions (#66).
// Mirrors handleStringsCall: for concrete byte-slice arguments the equivalent
// strings function is used (treating []byte as string); for opaque arguments
// conservative/pessimistic values are returned.
func (interp *Interpreter) handleBytesCall(name string, args []Value) (Value, bool) {
	// Helper: extract bytes arg as string (best-effort; opaque if not concrete).
	asStr := func(i int) (string, bool) {
		if i >= len(args) {
			return "", false
		}
		switch v := args[i].Raw.(type) {
		case string:
			return v, true
		case []byte:
			return string(v), true
		}
		return "", false
	}
	s0, s0ok := asStr(0)
	s1, s1ok := asStr(1)

	switch name {
	// --- Predicates (return true conservatively when args are opaque) ---
	case "Contains":
		if s0ok && s1ok {
			return Value{Raw: strings.Contains(s0, s1)}, true
		}
		return Value{Raw: true}, true
	case "ContainsAny":
		if s0ok && s1ok {
			return Value{Raw: strings.ContainsAny(s0, s1)}, true
		}
		return Value{Raw: true}, true
	case "ContainsRune":
		return Value{Raw: true}, true
	case "HasPrefix":
		if s0ok && s1ok {
			return Value{Raw: strings.HasPrefix(s0, s1)}, true
		}
		return Value{Raw: true}, true
	case "HasSuffix":
		if s0ok && s1ok {
			return Value{Raw: strings.HasSuffix(s0, s1)}, true
		}
		return Value{Raw: true}, true
	case "Equal":
		if s0ok && s1ok {
			return Value{Raw: s0 == s1}, true
		}
		return Value{Raw: true}, true
	case "EqualFold":
		if s0ok && s1ok {
			return Value{Raw: strings.EqualFold(s0, s1)}, true
		}
		return Value{Raw: true}, true
	case "Compare":
		if s0ok && s1ok {
			return Value{Raw: int64(strings.Compare(s0, s1))}, true
		}
		return Value{Raw: int64(0)}, true

	// --- Index functions (return 0 / non-negative for opaque args) ---
	case "Count":
		if s0ok && s1ok {
			return Value{Raw: int64(strings.Count(s0, s1))}, true
		}
		return Value{Raw: int64(1)}, true
	case "Index":
		if s0ok && s1ok {
			return Value{Raw: int64(strings.Index(s0, s1))}, true
		}
		return Value{Raw: int64(0)}, true
	case "IndexAny":
		if s0ok && s1ok {
			return Value{Raw: int64(strings.IndexAny(s0, s1))}, true
		}
		return Value{Raw: int64(0)}, true
	case "IndexByte":
		return Value{Raw: int64(0)}, true
	case "IndexRune":
		return Value{Raw: int64(0)}, true
	case "LastIndex":
		if s0ok && s1ok {
			return Value{Raw: int64(strings.LastIndex(s0, s1))}, true
		}
		return Value{Raw: int64(0)}, true
	case "LastIndexAny":
		if s0ok && s1ok {
			return Value{Raw: int64(strings.LastIndexAny(s0, s1))}, true
		}
		return Value{Raw: int64(0)}, true

	// --- Transforming functions (return input unchanged for opaque args) ---
	case "ToLower":
		if s0ok {
			return Value{Raw: []byte(strings.ToLower(s0))}, true
		}
		return Value{Raw: args[0].Raw}, true
	case "ToUpper":
		if s0ok {
			return Value{Raw: []byte(strings.ToUpper(s0))}, true
		}
		return Value{Raw: args[0].Raw}, true
	case "ToTitle":
		if s0ok {
			return Value{Raw: []byte(strings.ToTitle(s0))}, true
		}
		return Value{Raw: args[0].Raw}, true
	case "Title":
		if s0ok {
			return Value{Raw: []byte(strings.Title(s0))}, true //nolint:staticcheck
		}
		return Value{Raw: args[0].Raw}, true
	case "TrimSpace":
		if s0ok {
			return Value{Raw: []byte(strings.TrimSpace(s0))}, true
		}
		return Value{Raw: args[0].Raw}, true
	case "Trim", "TrimLeft", "TrimRight":
		if s0ok {
			return Value{Raw: args[0].Raw}, true
		}
		return Value{Raw: args[0].Raw}, true
	case "TrimFunc", "TrimLeftFunc", "TrimRightFunc", "Map":
		return Value{Raw: args[0].Raw}, true
	case "TrimPrefix":
		if s0ok && s1ok {
			return Value{Raw: []byte(strings.TrimPrefix(s0, s1))}, true
		}
		return Value{Raw: args[0].Raw}, true
	case "TrimSuffix":
		if s0ok && s1ok {
			return Value{Raw: []byte(strings.TrimSuffix(s0, s1))}, true
		}
		return Value{Raw: args[0].Raw}, true
	case "Replace":
		if s0ok && s1ok {
			s2, _ := asStr(2)
			n := -1
			if len(args) >= 4 {
				n = int(toInt64(args[3]))
			}
			return Value{Raw: []byte(strings.Replace(s0, s1, s2, n))}, true
		}
		return Value{Raw: args[0].Raw}, true
	case "ReplaceAll":
		if s0ok && s1ok {
			s2, _ := asStr(2)
			return Value{Raw: []byte(strings.ReplaceAll(s0, s1, s2))}, true
		}
		return Value{Raw: args[0].Raw}, true
	case "Repeat":
		if s0ok {
			n := 1
			if len(args) >= 2 {
				n = int(toInt64(args[1]))
			}
			return Value{Raw: []byte(strings.Repeat(s0, n))}, true
		}
		return Value{Raw: args[0].Raw}, true

	// --- Splitting functions (return single-element slice for opaque args) ---
	case "Split", "SplitN", "SplitAfter", "SplitAfterN":
		if s0ok && s1ok {
			parts := strings.Split(s0, s1)
			vs := make([]Value, len(parts))
			for i, p := range parts {
				vs[i] = Value{Raw: []byte(p)}
			}
			return Value{Raw: vs}, true
		}
		return Value{Raw: []Value{args[0]}}, true
	case "Fields":
		if s0ok {
			parts := strings.Fields(s0)
			vs := make([]Value, len(parts))
			for i, p := range parts {
				vs[i] = Value{Raw: []byte(p)}
			}
			return Value{Raw: vs}, true
		}
		return Value{Raw: []Value{args[0]}}, true
	case "Join":
		return Value{Raw: args[0].Raw}, true
	case "Cut":
		if s0ok && s1ok {
			before, after, found := strings.Cut(s0, s1)
			return Value{Raw: []Value{{Raw: []byte(before)}, {Raw: []byte(after)}, {Raw: found}}}, true
		}
		return Value{Raw: []Value{args[0], {Raw: []byte{}}, {Raw: false}}}, true
	case "CutPrefix":
		if s0ok && s1ok {
			after, found := strings.CutPrefix(s0, s1)
			return Value{Raw: []Value{{Raw: []byte(after)}, {Raw: found}}}, true
		}
		return Value{Raw: []Value{args[0], {Raw: false}}}, true
	case "CutSuffix":
		if s0ok && s1ok {
			before, found := strings.CutSuffix(s0, s1)
			return Value{Raw: []Value{{Raw: []byte(before)}, {Raw: found}}}, true
		}
		return Value{Raw: []Value{args[0], {Raw: false}}}, true
	case "Clone":
		return Value{Raw: args[0].Raw}, true

	// bytes.Buffer method calls (#79): receiver = args[0], other args follow.
	case "WriteString":
		// buf.WriteString(s) (int, error)
		n := 0
		if s, ok := stdlibArgString(args, 1); ok {
			n = len(s)
		}
		return Value{Raw: []Value{{Raw: int64(n)}, {}}}, true
	case "WriteByte":
		// buf.WriteByte(c byte) error
		return Value{}, true
	case "WriteRune":
		// buf.WriteRune(r rune) (int, error)
		return Value{Raw: []Value{{Raw: int64(1)}, {}}}, true
	case "Write":
		// buf.Write(p []byte) (int, error)
		return Value{Raw: []Value{{Raw: int64(0)}, {}}}, true
	case "String":
		// buf.String() string
		return Value{Raw: ""}, true
	case "Bytes":
		// buf.Bytes() []byte
		return Value{Raw: []byte(nil)}, true
	case "Len":
		// buf.Len() int
		return Value{Raw: int64(0)}, true
	case "Cap":
		// buf.Cap() int
		return Value{Raw: int64(0)}, true
	case "Reset":
		// buf.Reset()
		return Value{}, true
	case "Truncate":
		// buf.Truncate(n int)
		return Value{}, true
	case "Grow":
		// buf.Grow(n int)
		return Value{}, true
	case "ReadFrom":
		// buf.ReadFrom(r io.Reader) (int64, error)
		return Value{Raw: []Value{{Raw: int64(0)}, {}}}, true
	case "WriteTo":
		// buf.WriteTo(w io.Writer) (int64, error)
		return Value{Raw: []Value{{Raw: int64(0)}, {}}}, true
	case "ReadByte":
		// buf.ReadByte() (byte, error)
		return Value{Raw: []Value{{Raw: int64(0)}, {}}}, true
	case "ReadRune":
		// buf.ReadRune() (rune, int, error)
		return Value{Raw: []Value{{Raw: int64(0)}, {Raw: int64(0)}, {}}}, true
	case "ReadString":
		// buf.ReadString(delim byte) (string, error)
		return Value{Raw: []Value{{Raw: ""}, {}}}, true
	case "ReadBytes":
		// buf.ReadBytes(delim byte) ([]byte, error)
		return Value{Raw: []Value{{Raw: []byte(nil)}, {}}}, true
	case "Next":
		// buf.Next(n int) []byte
		return Value{Raw: []byte(nil)}, true
	case "UnreadByte":
		// buf.UnreadByte() error
		return Value{}, true
	case "UnreadRune":
		// buf.UnreadRune() error
		return Value{}, true
	}
	return Value{}, false
}

// handleErrorsCall models errors.* functions (#67).
func (interp *Interpreter) handleErrorsCall(name string, args []Value) (Value, bool) {
	switch name {
	case "New":
		msg := "<error>"
		if s, ok := stdlibArgString(args, 0); ok {
			msg = s
		}
		return Value{Raw: fmt.Errorf("%s", msg)}, true //nolint:err113

	case "Is":
		// Conservative: compare error strings if both are concrete errors.
		if len(args) >= 2 {
			if args[0].Raw == nil && args[1].Raw == nil {
				return Value{Raw: false}, true
			}
			e1, ok1 := args[0].Raw.(error)
			e2, ok2 := args[1].Raw.(error)
			if ok1 && ok2 {
				return Value{Raw: e1.Error() == e2.Error()}, true
			}
			// One or both not concrete — conservatively return false (no match).
		}
		return Value{Raw: false}, true

	case "As":
		// Conservative: always return false (no unwrapping chain modelled).
		return Value{Raw: false}, true

	case "Unwrap":
		// Conservative: no wrapping chain; return nil.
		return Value{}, true

	case "Join":
		// errors.Join(errs ...error) — return first non-nil if any.
		for _, a := range args {
			if a.Raw != nil {
				return Value{Raw: a.Raw}, true
			}
		}
		return Value{}, true
	}
	return Value{}, false
}

// handleSortCall models sort.* functions (#68).
// For functions that accept a user callback (less func, f func), the callback
// is probed once with representative arguments to surface any violations in it.
func (interp *Interpreter) handleSortCall(gid int64, name string, args []Value, site string) (Value, bool) {
	// probeCallback invokes a function-value arg at position argIdx with the
	// given explicit callArgs. For closures, FreeVars follow the explicit params
	// (matching how execFunction binds free variables: params[0..n-1] then freeVars[0..m-1]).
	probeCallback := func(argIdx int, callArgs []Value) {
		if argIdx >= len(args) {
			return
		}
		switch fn := args[argIdx].Raw.(type) {
		case *ssa.Function:
			if fn.Blocks != nil {
				interp.execFunction(gid, fn, callArgs)
			}
		case *ClosureValue:
			// Params come first, then free vars (see execFunction param binding).
			all := append(callArgs, fn.FreeVars...)
			interp.execFunction(gid, fn.Fn, all)
		}
	}

	switch name {
	case "Slice", "SliceStable":
		// Probe comparator with (0, 1) to detect violations in the callback.
		probeCallback(1, []Value{{Raw: int64(0)}, {Raw: int64(1)}})
		return Value{}, true

	case "SliceIsSorted":
		probeCallback(1, []Value{{Raw: int64(0)}, {Raw: int64(1)}})
		return Value{Raw: true}, true

	case "Search":
		// sort.Search(n int, f func(int) bool) int: probe f with n/2.
		n := int64(0)
		if len(args) > 0 {
			n = toInt64(args[0])
		}
		mid := n / 2
		probeCallback(1, []Value{{Raw: mid}})
		return Value{Raw: mid}, true

	case "Strings", "Ints", "Float64s":
		// Noop — sort in-place, no memory safety implications.
		return Value{}, true

	case "Sort", "Stable", "Reverse", "IsSorted":
		// sort.Interface operations — noop.
		return Value{}, true

	case "Find":
		// sort.Find(n int, cmp func(int) int) (int, bool): probe cmp with 0.
		probeCallback(1, []Value{{Raw: int64(0)}})
		return Value{Raw: []Value{{Raw: int64(0)}, {Raw: false}}}, true
	}
	return Value{}, false
}

// handleJSONCall models encoding/json.* functions (#70).
// Marshal and MarshalIndent return ([]byte, nil). Unmarshal returns nil error.
// Decoder/Encoder creation and methods return opaque or nil values.
func (interp *Interpreter) handleJSONCall(name string, args []Value) (Value, bool) {
	switch name {
	case "Marshal", "MarshalIndent":
		// Returns ([]byte, error). Return non-nil bytes so callers checking len(b) > 0 succeed.
		return Value{Raw: []Value{{Raw: []byte(`null`)}, {}}}, true

	case "Unmarshal":
		// Returns error (nil). We don't model the actual deserialization into the target.
		return Value{}, true

	case "NewDecoder", "NewEncoder":
		// Return an opaque value so method calls on the result are intercepted
		// via the same "encoding/json" package path.
		return Value{Raw: struct{}{}}, true

	case "Decode", "Encode":
		// Decoder.Decode / Encoder.Encode: return nil error.
		return Value{}, true

	case "More":
		// Conservative: always more tokens.
		return Value{Raw: true}, true

	case "Token":
		// Returns (Token, error): return ("", nil).
		return Value{Raw: []Value{{Raw: ""}, {}}}, true

	case "Valid":
		return Value{Raw: true}, true

	case "Compact", "Indent", "HTMLEscape":
		// Returns error (nil for Compact/Indent); HTMLEscape is void.
		return Value{}, true

	case "Number":
		return Value{Raw: ""}, true
	}
	return Value{}, false
}

// handleRegexpCall models regexp.* package-level functions and *Regexp methods (#71).
// For package-level Match* that return (bool, error), a tuple is returned.
// For method calls (receiver = args[0] is an opaque Regexp), a plain bool is returned.
// The two cases are distinguished by whether args[0] is a string (package-level pattern).
func (interp *Interpreter) handleRegexpCall(gid int64, name string, args []Value, site string) (Value, bool) {
	// probeCallback invokes a function-value at position argIdx with the given callArgs.
	probeCallback := func(argIdx int, callArgs []Value) {
		if argIdx >= len(args) {
			return
		}
		switch fn := args[argIdx].Raw.(type) {
		case *ssa.Function:
			if fn.Blocks != nil {
				interp.execFunction(gid, fn, callArgs)
			}
		case *ClosureValue:
			all := append(callArgs, fn.FreeVars...)
			interp.execFunction(gid, fn.Fn, all)
		}
	}

	// isPackageLevel: true when args[0] is a string (the pattern argument of
	// regexp.MatchString/Match/etc.) rather than a *Regexp receiver.
	isPackageLevel := len(args) > 0
	if len(args) > 0 {
		_, isPackageLevel = args[0].Raw.(string)
	}

	switch name {
	case "Compile":
		// (expr string) (*Regexp, error)
		return Value{Raw: []Value{{Raw: struct{}{}}, {}}}, true

	case "MustCompile":
		// (expr string) *Regexp
		return Value{Raw: struct{}{}}, true

	case "Match":
		if isPackageLevel {
			return Value{Raw: []Value{{Raw: true}, {}}}, true
		}
		return Value{Raw: true}, true

	case "MatchString":
		if isPackageLevel {
			return Value{Raw: []Value{{Raw: true}, {}}}, true
		}
		return Value{Raw: true}, true

	case "MatchReader":
		if isPackageLevel {
			return Value{Raw: []Value{{Raw: true}, {}}}, true
		}
		return Value{Raw: true}, true

	case "QuoteMeta":
		if s, ok := stdlibArgString(args, 0); ok {
			return Value{Raw: regexp_quoteMeta(s)}, true
		}
		return Value{Raw: ""}, true

	case "FindString":
		// receiver = args[0], src = args[1]
		return Value{Raw: ""}, true

	case "FindStringIndex":
		return Value{Raw: []Value{{Raw: int64(0)}, {Raw: int64(0)}}}, true

	case "FindStringSubmatch":
		return Value{Raw: []Value{}}, true

	case "FindAllString":
		return Value{Raw: []Value{}}, true

	case "FindAllStringSubmatch":
		return Value{Raw: []Value{}}, true

	case "FindAllStringIndex":
		return Value{Raw: []Value{}}, true

	case "ReplaceAllString":
		// receiver = args[0], src = args[1], repl = args[2]; return src unchanged.
		if len(args) > 1 {
			if s, ok := args[1].Raw.(string); ok {
				return Value{Raw: s}, true
			}
		}
		return Value{Raw: ""}, true

	case "ReplaceAllLiteralString":
		// Return the replacement string.
		if len(args) > 2 {
			if s, ok := args[2].Raw.(string); ok {
				return Value{Raw: s}, true
			}
		}
		return Value{Raw: ""}, true

	case "ReplaceAllStringFunc":
		// Probe the callback with "" to surface violations inside it.
		probeCallback(2, []Value{{Raw: ""}})
		if len(args) > 1 {
			return args[1], true
		}
		return Value{Raw: ""}, true

	case "ReplaceAll":
		if len(args) > 1 {
			return args[1], true
		}
		return Value{Raw: []byte{}}, true

	case "ReplaceAllLiteral":
		if len(args) > 2 {
			return args[2], true
		}
		return Value{Raw: []byte{}}, true

	case "Split":
		if len(args) > 1 {
			return Value{Raw: []Value{args[1]}}, true
		}
		return Value{Raw: []Value{}}, true

	case "SubexpNames":
		return Value{Raw: []Value{}}, true

	case "SubexpIndex":
		return Value{Raw: int64(-1)}, true

	case "NumSubexp":
		return Value{Raw: int64(0)}, true

	case "String":
		return Value{Raw: ""}, true

	case "Longest", "Copy":
		return Value{Raw: struct{}{}}, true

	case "Find", "FindIndex", "FindSubmatch", "FindAll", "FindAllIndex", "FindAllSubmatch":
		return Value{Raw: []Value{}}, true
	}
	return Value{}, false
}

// regexp_quoteMeta is a thin wrapper around regexp.QuoteMeta used from handleRegexpCall.
// It lives here to avoid a package-level import of "regexp" that would add a large import.
func regexp_quoteMeta(s string) string {
	const special = `\.+*?()|[]{}^$`
	var b strings.Builder
	for _, r := range s {
		if strings.ContainsRune(special, r) {
			b.WriteRune('\\')
		}
		b.WriteRune(r)
	}
	return b.String()
}

// handleMathCall models math.* functions (#72).
// For concrete float64 arguments the real math function is called directly.
// For opaque arguments a conservative (non-NaN, non-Inf) sentinel is returned.
func (interp *Interpreter) handleMathCall(name string, args []Value) (Value, bool) {
	x, xok := stdlibArgFloat(args, 0)
	y, yok := stdlibArgFloat(args, 1)

	switch name {
	case "Abs":
		if xok {
			return Value{Raw: math.Abs(x)}, true
		}
		return Value{Raw: float64(0)}, true

	case "Floor":
		if xok {
			return Value{Raw: math.Floor(x)}, true
		}
		return Value{Raw: float64(0)}, true

	case "Ceil":
		if xok {
			return Value{Raw: math.Ceil(x)}, true
		}
		return Value{Raw: float64(0)}, true

	case "Round":
		if xok {
			return Value{Raw: math.Round(x)}, true
		}
		return Value{Raw: float64(0)}, true

	case "Trunc":
		if xok {
			return Value{Raw: math.Trunc(x)}, true
		}
		return Value{Raw: float64(0)}, true

	case "Sqrt":
		if xok {
			return Value{Raw: math.Sqrt(x)}, true
		}
		return Value{Raw: float64(1)}, true

	case "Cbrt":
		if xok {
			return Value{Raw: math.Cbrt(x)}, true
		}
		return Value{Raw: float64(1)}, true

	case "Pow":
		if xok && yok {
			return Value{Raw: math.Pow(x, y)}, true
		}
		return Value{Raw: float64(1)}, true

	case "Pow10":
		n, nok := stdlibArgInt(args, 0)
		if nok {
			return Value{Raw: math.Pow10(int(n))}, true
		}
		return Value{Raw: float64(1)}, true

	case "Log":
		if xok {
			return Value{Raw: math.Log(x)}, true
		}
		return Value{Raw: float64(0)}, true

	case "Log2":
		if xok {
			return Value{Raw: math.Log2(x)}, true
		}
		return Value{Raw: float64(0)}, true

	case "Log10":
		if xok {
			return Value{Raw: math.Log10(x)}, true
		}
		return Value{Raw: float64(0)}, true

	case "Log1p":
		if xok {
			return Value{Raw: math.Log1p(x)}, true
		}
		return Value{Raw: float64(0)}, true

	case "Exp":
		if xok {
			return Value{Raw: math.Exp(x)}, true
		}
		return Value{Raw: float64(1)}, true

	case "Exp2":
		if xok {
			return Value{Raw: math.Exp2(x)}, true
		}
		return Value{Raw: float64(1)}, true

	case "Expm1":
		if xok {
			return Value{Raw: math.Expm1(x)}, true
		}
		return Value{Raw: float64(0)}, true

	case "Sin":
		if xok {
			return Value{Raw: math.Sin(x)}, true
		}
		return Value{Raw: float64(0)}, true

	case "Cos":
		if xok {
			return Value{Raw: math.Cos(x)}, true
		}
		return Value{Raw: float64(1)}, true

	case "Tan":
		if xok {
			return Value{Raw: math.Tan(x)}, true
		}
		return Value{Raw: float64(0)}, true

	case "Asin":
		if xok {
			return Value{Raw: math.Asin(x)}, true
		}
		return Value{Raw: float64(0)}, true

	case "Acos":
		if xok {
			return Value{Raw: math.Acos(x)}, true
		}
		return Value{Raw: float64(0)}, true

	case "Atan":
		if xok {
			return Value{Raw: math.Atan(x)}, true
		}
		return Value{Raw: float64(0)}, true

	case "Atan2":
		if xok && yok {
			return Value{Raw: math.Atan2(x, y)}, true
		}
		return Value{Raw: float64(0)}, true

	case "Sinh", "Cosh", "Tanh":
		if xok {
			switch name {
			case "Sinh":
				return Value{Raw: math.Sinh(x)}, true
			case "Cosh":
				return Value{Raw: math.Cosh(x)}, true
			case "Tanh":
				return Value{Raw: math.Tanh(x)}, true
			}
		}
		return Value{Raw: float64(0)}, true

	case "Hypot":
		if xok && yok {
			return Value{Raw: math.Hypot(x, y)}, true
		}
		return Value{Raw: float64(0)}, true

	case "Min":
		if xok && yok {
			return Value{Raw: math.Min(x, y)}, true
		}
		return Value{Raw: float64(0)}, true

	case "Max":
		if xok && yok {
			return Value{Raw: math.Max(x, y)}, true
		}
		return Value{Raw: float64(0)}, true

	case "Mod":
		if xok && yok {
			return Value{Raw: math.Mod(x, y)}, true
		}
		return Value{Raw: float64(0)}, true

	case "Remainder":
		if xok && yok {
			return Value{Raw: math.Remainder(x, y)}, true
		}
		return Value{Raw: float64(0)}, true

	case "Dim":
		if xok && yok {
			return Value{Raw: math.Dim(x, y)}, true
		}
		return Value{Raw: float64(0)}, true

	case "Inf":
		sign, sok := stdlibArgInt(args, 0)
		if sok {
			return Value{Raw: math.Inf(int(sign))}, true
		}
		return Value{Raw: math.Inf(1)}, true

	case "IsInf":
		if xok {
			sign := int64(0)
			if n, nok := stdlibArgInt(args, 1); nok {
				sign = n
			}
			return Value{Raw: math.IsInf(x, int(sign))}, true
		}
		return Value{Raw: false}, true

	case "IsNaN":
		if xok {
			return Value{Raw: math.IsNaN(x)}, true
		}
		return Value{Raw: false}, true

	case "NaN":
		return Value{Raw: math.NaN()}, true

	case "Signbit":
		if xok {
			return Value{Raw: math.Signbit(x)}, true
		}
		return Value{Raw: false}, true

	case "Copysign":
		if xok && yok {
			return Value{Raw: math.Copysign(x, y)}, true
		}
		return Value{Raw: float64(0)}, true

	case "Logb":
		if xok {
			return Value{Raw: math.Logb(x)}, true
		}
		return Value{Raw: float64(0)}, true

	case "Ilogb":
		if xok {
			return Value{Raw: int64(math.Ilogb(x))}, true
		}
		return Value{Raw: int64(0)}, true

	case "Frexp":
		if xok {
			frac, exp := math.Frexp(x)
			return Value{Raw: []Value{{Raw: frac}, {Raw: int64(exp)}}}, true
		}
		return Value{Raw: []Value{{Raw: float64(0)}, {Raw: int64(0)}}}, true

	case "Ldexp":
		if xok {
			exp, eok := stdlibArgInt(args, 1)
			if eok {
				return Value{Raw: math.Ldexp(x, int(exp))}, true
			}
		}
		return Value{Raw: float64(0)}, true

	case "Modf":
		if xok {
			i, f := math.Modf(x)
			return Value{Raw: []Value{{Raw: float64(i)}, {Raw: f}}}, true
		}
		return Value{Raw: []Value{{Raw: float64(0)}, {Raw: float64(0)}}}, true

	case "J0", "J1":
		if xok {
			switch name {
			case "J0":
				return Value{Raw: math.J0(x)}, true
			case "J1":
				return Value{Raw: math.J1(x)}, true
			}
		}
		return Value{Raw: float64(0)}, true

	case "Gamma":
		if xok {
			return Value{Raw: math.Gamma(x)}, true
		}
		return Value{Raw: float64(1)}, true

	case "Lgamma":
		if xok {
			lg, sign := math.Lgamma(x)
			return Value{Raw: []Value{{Raw: lg}, {Raw: int64(sign)}}}, true
		}
		return Value{Raw: []Value{{Raw: float64(0)}, {Raw: int64(1)}}}, true

	case "Erf", "Erfc":
		if xok {
			switch name {
			case "Erf":
				return Value{Raw: math.Erf(x)}, true
			case "Erfc":
				return Value{Raw: math.Erfc(x)}, true
			}
		}
		return Value{Raw: float64(0)}, true
	}
	return Value{}, false
}

// handleUTF8Call models unicode/utf8.* functions (#75).
// For concrete string/rune arguments the real utf8 function is called; otherwise
// conservative non-zero values are returned to avoid blocking execution paths.
func (interp *Interpreter) handleUTF8Call(name string, args []Value) (Value, bool) {
	s0, s0ok := stdlibArgString(args, 0)
	r0, r0ok := stdlibArgInt(args, 0)

	switch name {
	case "RuneCountInString":
		if s0ok {
			return Value{Raw: int64(utf8.RuneCountInString(s0))}, true
		}
		return Value{Raw: int64(1)}, true

	case "RuneCount":
		// RuneCount(p []byte) int — conservative.
		return Value{Raw: int64(1)}, true

	case "ValidString":
		if s0ok {
			return Value{Raw: utf8.ValidString(s0)}, true
		}
		return Value{Raw: true}, true

	case "Valid":
		return Value{Raw: true}, true

	case "ValidRune":
		if r0ok {
			return Value{Raw: utf8.ValidRune(rune(r0))}, true
		}
		return Value{Raw: true}, true

	case "RuneLen":
		if r0ok {
			return Value{Raw: int64(utf8.RuneLen(rune(r0)))}, true
		}
		return Value{Raw: int64(1)}, true

	case "EncodeRune":
		// EncodeRune(p []byte, r rune) int — return byte size of the rune.
		r1, r1ok := stdlibArgInt(args, 1)
		if r1ok {
			return Value{Raw: int64(utf8.RuneLen(rune(r1)))}, true
		}
		return Value{Raw: int64(1)}, true

	case "DecodeRuneInString":
		// Returns (r rune, size int).
		if s0ok && len(s0) > 0 {
			r, size := utf8.DecodeRuneInString(s0)
			return Value{Raw: []Value{{Raw: int64(r)}, {Raw: int64(size)}}}, true
		}
		return Value{Raw: []Value{{Raw: int64('?')}, {Raw: int64(1)}}}, true

	case "DecodeRune":
		// Returns (r rune, size int).
		return Value{Raw: []Value{{Raw: int64('?')}, {Raw: int64(1)}}}, true

	case "DecodeLastRuneInString":
		if s0ok && len(s0) > 0 {
			r, size := utf8.DecodeLastRuneInString(s0)
			return Value{Raw: []Value{{Raw: int64(r)}, {Raw: int64(size)}}}, true
		}
		return Value{Raw: []Value{{Raw: int64('?')}, {Raw: int64(1)}}}, true

	case "DecodeLastRune":
		return Value{Raw: []Value{{Raw: int64('?')}, {Raw: int64(1)}}}, true

	case "FullRune", "FullRuneInString":
		return Value{Raw: true}, true

	case "AppendRune":
		// AppendRune(p []byte, r rune) []byte — conservative: return p unchanged.
		if len(args) > 0 {
			return args[0], true
		}
		return Value{Raw: []byte{}}, true

	case "RuneError":
		return Value{Raw: int64(utf8.RuneError)}, true

	case "MaxRune":
		return Value{Raw: int64(utf8.MaxRune)}, true

	case "UTFMax":
		return Value{Raw: int64(utf8.UTFMax)}, true
	}
	// Suppress unused variable warnings for r0/r0ok used only in some branches.
	_ = r0
	_ = r0ok
	return Value{}, false
}

// handleUnicodeCall models unicode.* functions (#75).
// Predicates return true conservatively; transforms pass through.
func (interp *Interpreter) handleUnicodeCall(name string, args []Value) (Value, bool) {
	r0, r0ok := stdlibArgInt(args, 0)

	switch name {
	case "IsLetter":
		if r0ok {
			return Value{Raw: unicode.IsLetter(rune(r0))}, true
		}
		return Value{Raw: true}, true

	case "IsDigit":
		if r0ok {
			return Value{Raw: unicode.IsDigit(rune(r0))}, true
		}
		return Value{Raw: true}, true

	case "IsSpace":
		if r0ok {
			return Value{Raw: unicode.IsSpace(rune(r0))}, true
		}
		return Value{Raw: true}, true

	case "IsUpper":
		if r0ok {
			return Value{Raw: unicode.IsUpper(rune(r0))}, true
		}
		return Value{Raw: false}, true

	case "IsLower":
		if r0ok {
			return Value{Raw: unicode.IsLower(rune(r0))}, true
		}
		return Value{Raw: true}, true

	case "IsPunct":
		if r0ok {
			return Value{Raw: unicode.IsPunct(rune(r0))}, true
		}
		return Value{Raw: false}, true

	case "IsNumber":
		if r0ok {
			return Value{Raw: unicode.IsNumber(rune(r0))}, true
		}
		return Value{Raw: true}, true

	case "IsMark":
		if r0ok {
			return Value{Raw: unicode.IsMark(rune(r0))}, true
		}
		return Value{Raw: false}, true

	case "IsControl":
		if r0ok {
			return Value{Raw: unicode.IsControl(rune(r0))}, true
		}
		return Value{Raw: false}, true

	case "IsGraphic", "IsPrint":
		if r0ok {
			return Value{Raw: unicode.IsGraphic(rune(r0))}, true
		}
		return Value{Raw: true}, true

	case "IsTitle":
		if r0ok {
			return Value{Raw: unicode.IsTitle(rune(r0))}, true
		}
		return Value{Raw: false}, true

	case "ToLower":
		if r0ok {
			return Value{Raw: int64(unicode.ToLower(rune(r0)))}, true
		}
		return Value{Raw: r0}, true

	case "ToUpper":
		if r0ok {
			return Value{Raw: int64(unicode.ToUpper(rune(r0)))}, true
		}
		return Value{Raw: r0}, true

	case "ToTitle":
		if r0ok {
			return Value{Raw: int64(unicode.ToTitle(rune(r0)))}, true
		}
		return Value{Raw: r0}, true

	case "In":
		// unicode.In(r rune, ranges ...*RangeTable) bool — conservative.
		return Value{Raw: true}, true

	case "Is":
		// unicode.Is(rangeTab *RangeTable, r rune) bool — conservative.
		return Value{Raw: true}, true

	case "SimpleFold":
		if r0ok {
			return Value{Raw: int64(unicode.SimpleFold(rune(r0)))}, true
		}
		return Value{Raw: r0}, true
	}
	_ = r0
	_ = r0ok
	return Value{}, false
}

// handleContextCall models context.* functions (#76).
// context.Background/TODO return an opaque non-nil value so downstream
// nil-checks on the context pass correctly.  WithCancel/WithTimeout/WithDeadline
// return a (ctx, cancelFunc) tuple where both values are opaque non-nil.
func (interp *Interpreter) handleContextCall(name string, args []Value) (Value, bool) {
	opaque := Value{Raw: struct{}{}}

	switch name {
	case "Background", "TODO":
		return opaque, true

	case "WithCancel":
		// Returns (Context, CancelFunc).
		return Value{Raw: []Value{opaque, opaque}}, true

	case "WithTimeout", "WithDeadline":
		// Returns (Context, CancelFunc).
		return Value{Raw: []Value{opaque, opaque}}, true

	case "WithValue":
		// Returns Context (ignores key/value pair).
		return opaque, true

	case "WithCancelCause":
		// Go 1.20+: returns (Context, CancelCauseFunc).
		return Value{Raw: []Value{opaque, opaque}}, true

	case "Cause":
		// Returns nil error.
		return Value{}, true

	case "Done":
		// ctx.Done() returns a nil channel (never fires in our model).
		return Value{}, true

	case "Err":
		// ctx.Err() returns nil (no cancellation modelled).
		return Value{}, true

	case "Value":
		// ctx.Value(key) returns nil (no value propagation modelled).
		return Value{}, true

	case "Deadline":
		// ctx.Deadline() returns (zero time, false).
		return Value{Raw: []Value{{}, {Raw: false}}}, true
	}
	return Value{}, false
}

// handleAtomicCall models sync/atomic.* functions (#77).
// Atomic operations read and write through interp.valueStore keyed by the
// pointer argument's AllocID; they do NOT call handleLoad/handleStore to
// avoid false-positive race reports on atomic accesses.
func (interp *Interpreter) handleAtomicCall(name string, args []Value) (Value, bool) {
	var allocID shadow.AllocID
	if len(args) > 0 && args[0].Provenance != nil {
		allocID = args[0].Provenance.Alloc
	}

	switch name {
	case "LoadInt32", "LoadInt64", "LoadUint32", "LoadUint64", "LoadUintptr", "LoadPointer":
		if allocID != 0 && interp.valueStore != nil {
			if v, ok := interp.valueStore[allocID]; ok {
				return v, true
			}
		}
		return Value{Raw: int64(0)}, true

	case "StoreInt32", "StoreInt64", "StoreUint32", "StoreUint64", "StoreUintptr", "StorePointer":
		if allocID != 0 && len(args) >= 2 && interp.valueStore != nil {
			interp.valueStore[allocID] = args[1]
		}
		return Value{}, true

	case "AddInt32", "AddInt64", "AddUint32", "AddUint64", "AddUintptr":
		if allocID != 0 && len(args) >= 2 {
			cur := int64(0)
			if v, ok := interp.valueStore[allocID]; ok {
				cur = toInt64(v)
			}
			newVal := Value{Raw: cur + toInt64(args[1])}
			if interp.valueStore != nil {
				interp.valueStore[allocID] = newVal
			}
			return newVal, true
		}
		return Value{Raw: int64(0)}, true

	case "AndInt32", "AndInt64", "AndUint32", "AndUint64", "AndUintptr",
		"OrInt32", "OrInt64", "OrUint32", "OrUint64", "OrUintptr":
		if allocID != 0 && len(args) >= 2 {
			cur := int64(0)
			if v, ok := interp.valueStore[allocID]; ok {
				cur = toInt64(v)
			}
			delta := toInt64(args[1])
			var newRaw int64
			if name[:2] == "An" {
				newRaw = cur & delta
			} else {
				newRaw = cur | delta
			}
			newVal := Value{Raw: newRaw}
			if interp.valueStore != nil {
				interp.valueStore[allocID] = newVal
			}
			return newVal, true
		}
		return Value{Raw: int64(0)}, true

	case "CompareAndSwapInt32", "CompareAndSwapInt64",
		"CompareAndSwapUint32", "CompareAndSwapUint64",
		"CompareAndSwapUintptr", "CompareAndSwapPointer":
		if allocID != 0 && len(args) >= 3 {
			cur := int64(0)
			if v, ok := interp.valueStore[allocID]; ok {
				cur = toInt64(v)
			}
			if cur == toInt64(args[1]) {
				if interp.valueStore != nil {
					interp.valueStore[allocID] = args[2]
				}
				return Value{Raw: true}, true
			}
			return Value{Raw: false}, true
		}
		return Value{Raw: true}, true // pessimistic: assume CAS succeeds

	case "SwapInt32", "SwapInt64", "SwapUint32", "SwapUint64", "SwapUintptr", "SwapPointer":
		if allocID != 0 && len(args) >= 2 {
			old := Value{Raw: int64(0)}
			if v, ok := interp.valueStore[allocID]; ok {
				old = v
			}
			if interp.valueStore != nil {
				interp.valueStore[allocID] = args[1]
			}
			return old, true
		}
		return Value{Raw: int64(0)}, true

	case "Value":
		// atomic.Value is a struct; Load/Store methods on it.
		// Method calls on atomic.Value have pkg path "sync/atomic".
		return Value{}, true
	}
	return Value{}, false
}

// handleIOCall models io.* functions (#78).
func (interp *Interpreter) handleIOCall(name string, args []Value) (Value, bool) {
	opaque := Value{Raw: struct{}{}}
	switch name {
	case "ReadAll":
		// io.ReadAll(r Reader) ([]byte, error)
		return Value{Raw: []Value{{Raw: []byte("data")}, {}}}, true

	case "Copy", "CopyBuffer":
		// io.Copy(dst Writer, src Reader) (int64, error)
		return Value{Raw: []Value{{Raw: int64(0)}, {}}}, true

	case "CopyN":
		// io.CopyN(dst Writer, src Reader, n int64) (int64, error)
		return Value{Raw: []Value{{Raw: int64(0)}, {}}}, true

	case "WriteString":
		// io.WriteString(w Writer, s string) (n int, err error)
		n := 0
		if s, ok := stdlibArgString(args, 1); ok {
			n = len(s)
		}
		return Value{Raw: []Value{{Raw: int64(n)}, {}}}, true

	case "Pipe":
		// io.Pipe() (*PipeReader, *PipeWriter)
		return Value{Raw: []Value{opaque, opaque}}, true

	case "NopCloser":
		// io.NopCloser(r Reader) ReadCloser
		if len(args) > 0 {
			return args[0], true
		}
		return opaque, true

	case "LimitReader":
		// io.LimitReader(r Reader, n int64) Reader
		return opaque, true

	case "MultiReader":
		// io.MultiReader(readers ...Reader) Reader
		return opaque, true

	case "MultiWriter":
		// io.MultiWriter(writers ...Writer) Writer
		return opaque, true

	case "TeeReader":
		// io.TeeReader(r Reader, w Writer) Reader
		return opaque, true

	case "NewSectionReader":
		// io.NewSectionReader(r ReaderAt, off int64, n int64) *SectionReader
		return opaque, true

	case "ReadAtLeast", "ReadFull":
		// (int, error)
		return Value{Raw: []Value{{Raw: int64(0)}, {}}}, true

	case "Discard":
		// io.Discard is a variable (iota/Writer); return opaque.
		return opaque, true
	}
	return Value{}, false
}

// handleBufioCall models bufio.* functions (#78).
func (interp *Interpreter) handleBufioCall(name string, args []Value) (Value, bool) {
	opaque := Value{Raw: struct{}{}}
	switch name {
	case "NewReader", "NewReaderSize":
		return opaque, true
	case "NewWriter", "NewWriterSize":
		return opaque, true
	case "NewScanner":
		return opaque, true
	case "NewReadWriter":
		return opaque, true

	// Scanner methods (receiver = args[0]).
	case "Scan":
		// Always return false (scanner exhausted in our model).
		return Value{Raw: false}, true
	case "Text":
		return Value{Raw: ""}, true
	case "Bytes":
		return Value{Raw: []byte(nil)}, true
	case "Err":
		return Value{}, true
	case "Split":
		return Value{}, true
	case "Buffer":
		return Value{}, true

	// Reader/Writer flush methods.
	case "Flush":
		// (error)
		return Value{}, true
	case "Available":
		return Value{Raw: int64(0)}, true
	case "Buffered":
		return Value{Raw: int64(0)}, true
	case "Reset":
		return Value{}, true
	case "Size":
		return Value{Raw: int64(0)}, true
	case "Discard":
		// (int, error)
		return Value{Raw: []Value{{Raw: int64(0)}, {}}}, true

	// bufio.Writer methods.
	case "Write":
		return Value{Raw: []Value{{Raw: int64(0)}, {}}}, true
	case "WriteString":
		n := 0
		if s, ok := stdlibArgString(args, 1); ok {
			n = len(s)
		}
		return Value{Raw: []Value{{Raw: int64(n)}, {}}}, true
	case "WriteByte":
		return Value{}, true
	case "WriteRune":
		return Value{Raw: []Value{{Raw: int64(1)}, {}}}, true

	// bufio.Reader methods.
	case "ReadString":
		return Value{Raw: []Value{{Raw: ""}, {}}}, true
	case "ReadLine":
		return Value{Raw: []Value{{Raw: []byte(nil)}, {Raw: false}, {}}}, true
	case "ReadByte":
		return Value{Raw: []Value{{Raw: int64(0)}, {}}}, true
	case "ReadRune":
		return Value{Raw: []Value{{Raw: int64(0)}, {Raw: int64(0)}, {}}}, true
	case "ReadSlice":
		return Value{Raw: []Value{{Raw: []byte(nil)}, {}}}, true
	case "Peek":
		return Value{Raw: []Value{{Raw: []byte(nil)}, {}}}, true
	case "UnreadByte":
		return Value{}, true
	case "UnreadRune":
		return Value{}, true
	}
	return Value{}, false
}

// handleLogCall models log.* functions (#80).
// log.Fatal/Fatalln/Fatalf mark all goroutines as Panicked (simulates os.Exit(1)).
// log.Panic/Panicln/Panicf mark the current goroutine as Panicked.
// log.Print/Println/Printf are noops (no output in the interpreter).
func (interp *Interpreter) handleLogCall(gid int64, name string, args []Value) (Value, bool) {
	switch name {
	case "Print", "Println", "Printf":
		return Value{}, true

	case "Fatal", "Fatalln", "Fatalf":
		// Simulates os.Exit(1): terminate all goroutines.
		for _, g := range interp.goroutines {
			g.Panicked = true
		}
		return Value{}, true

	case "Panic", "Panicln", "Panicf":
		// Mark only the current goroutine as panicked.
		if g := interp.goroutines[gid]; g != nil {
			g.Panicked = true
		}
		return Value{}, true

	case "New":
		// log.New(out, prefix, flags) *Logger — return opaque logger.
		return Value{Raw: struct{}{}}, true

	case "SetOutput", "SetFlags", "SetPrefix":
		return Value{}, true

	case "Flags":
		return Value{Raw: int64(0)}, true

	case "Prefix":
		return Value{Raw: ""}, true

	case "Writer":
		return Value{Raw: struct{}{}}, true

	case "Default":
		// log.Default() *Logger
		return Value{Raw: struct{}{}}, true
	}
	return Value{}, false
}

// handleHexCall models encoding/hex.* functions (#81).
func (interp *Interpreter) handleHexCall(name string, args []Value) (Value, bool) {
	switch name {
	case "EncodeToString":
		if len(args) > 0 {
			switch b := args[0].Raw.(type) {
			case []byte:
				return Value{Raw: hex.EncodeToString(b)}, true
			case []Value:
				bs := make([]byte, len(b))
				for i, v := range b {
					bs[i] = byte(toInt64(v))
				}
				return Value{Raw: hex.EncodeToString(bs)}, true
			}
		}
		return Value{Raw: "deadbeef"}, true // sentinel

	case "DecodeString":
		if s, ok := stdlibArgString(args, 0); ok {
			b, err := hex.DecodeString(s)
			if err != nil {
				return Value{Raw: []Value{{Raw: []byte(nil)}, {Raw: err.Error()}}}, true
			}
			return Value{Raw: []Value{{Raw: b}, {}}}, true
		}
		return Value{Raw: []Value{{Raw: []byte{0xde, 0xad}}, {}}}, true

	case "Encode":
		// hex.Encode(dst, src []byte) int
		if len(args) >= 2 {
			switch b := args[1].Raw.(type) {
			case []byte:
				return Value{Raw: int64(hex.EncodedLen(len(b)))}, true
			}
		}
		return Value{Raw: int64(0)}, true

	case "Decode":
		// hex.Decode(dst, src []byte) (int, error)
		return Value{Raw: []Value{{Raw: int64(0)}, {}}}, true

	case "EncodedLen":
		if n, ok := stdlibArgInt(args, 0); ok {
			return Value{Raw: int64(hex.EncodedLen(int(n)))}, true
		}
		return Value{Raw: int64(0)}, true

	case "DecodedLen":
		if n, ok := stdlibArgInt(args, 0); ok {
			return Value{Raw: int64(hex.DecodedLen(int(n)))}, true
		}
		return Value{Raw: int64(0)}, true

	case "NewEncoder":
		return Value{Raw: struct{}{}}, true

	case "NewDecoder":
		return Value{Raw: struct{}{}}, true

	case "Dump":
		// Returns formatted hex dump string.
		return Value{Raw: ""}, true
	}
	return Value{}, false
}

// handleBase64Call models encoding/base64.* functions (#81).
func (interp *Interpreter) handleBase64Call(name string, args []Value) (Value, bool) {
	opaque := Value{Raw: struct{}{}} // opaque *Encoding or io.ReadCloser

	switch name {
	case "StdEncoding", "URLEncoding", "RawStdEncoding", "RawURLEncoding":
		// These are package-level variables; return opaque *Encoding.
		return opaque, true

	case "NewEncoding":
		return opaque, true

	case "EncodeToString":
		// Called as method: enc.EncodeToString(src []byte) string
		// args[0] = receiver (*Encoding), args[1] = src
		if len(args) >= 2 {
			switch b := args[1].Raw.(type) {
			case []byte:
				return Value{Raw: base64.StdEncoding.EncodeToString(b)}, true
			}
		}
		return Value{Raw: "aGVsbG8="}, true // base64("hello")

	case "DecodeString":
		// enc.DecodeString(s string) ([]byte, error)
		if len(args) >= 2 {
			if s, ok := stdlibArgString(args, 1); ok {
				b, err := base64.StdEncoding.DecodeString(s)
				if err != nil {
					// Try URL encoding.
					b, err = base64.URLEncoding.DecodeString(s)
				}
				if err == nil {
					return Value{Raw: []Value{{Raw: b}, {}}}, true
				}
				return Value{Raw: []Value{{Raw: []byte(nil)}, {Raw: err.Error()}}}, true
			}
		}
		return Value{Raw: []Value{{Raw: []byte("data")}, {}}}, true

	case "Encode":
		// enc.Encode(dst, src []byte)
		return Value{}, true

	case "Decode":
		// enc.Decode(dst, src []byte) (int, error)
		return Value{Raw: []Value{{Raw: int64(0)}, {}}}, true

	case "EncodedLen":
		if len(args) >= 2 {
			if n, ok := stdlibArgInt(args, 1); ok {
				return Value{Raw: int64(base64.StdEncoding.EncodedLen(int(n)))}, true
			}
		}
		return Value{Raw: int64(0)}, true

	case "DecodedLen":
		if len(args) >= 2 {
			if n, ok := stdlibArgInt(args, 1); ok {
				return Value{Raw: int64(base64.StdEncoding.DecodedLen(int(n)))}, true
			}
		}
		return Value{Raw: int64(0)}, true

	case "NewEncoder":
		// NewEncoder(enc *Encoding, w io.Writer) io.WriteCloser
		return opaque, true

	case "NewDecoder":
		// NewDecoder(enc *Encoding, r io.Reader) io.Reader
		return opaque, true
	}
	return Value{}, false
}

// handleCryptoRandCall models crypto/rand.* functions (#82).
func (interp *Interpreter) handleCryptoRandCall(name string, args []Value) (Value, bool) {
	switch name {
	case "Read":
		// rand.Read(b []byte) (int, error) — fills b with random bytes.
		n := 0
		if len(args) > 0 {
			switch b := args[0].Raw.(type) {
			case []byte:
				n = len(b)
			case []Value:
				n = len(b)
			}
		}
		return Value{Raw: []Value{{Raw: int64(n)}, {}}}, true

	case "Int":
		// rand.Int(rand io.Reader, max *big.Int) (*big.Int, error)
		return Value{Raw: []Value{{Raw: struct{}{}}, {}}}, true

	case "Prime":
		// rand.Prime(rand io.Reader, bits int) (*big.Int, error)
		return Value{Raw: []Value{{Raw: struct{}{}}, {}}}, true

	case "Reader":
		// crypto/rand.Reader is a global; return opaque.
		return Value{Raw: struct{}{}}, true
	}
	return Value{}, false
}

// handleHashCall models crypto/md5, crypto/sha1, crypto/sha256, crypto/sha512 (#82).
// All four packages share the same API (hash.Hash interface), differing only in digest size.
func (interp *Interpreter) handleHashCall(pkgPath, name string, args []Value) (Value, bool) {
	// Digest lengths by package.
	digestLen := 16 // md5
	switch pkgPath {
	case "crypto/sha1":
		digestLen = 20
	case "crypto/sha256":
		digestLen = 32
	case "crypto/sha512":
		digestLen = 64
	}

	switch name {
	case "New", "New224", "New384":
		// Returns a hash.Hash (opaque interface value).
		return Value{Raw: struct{}{}}, true

	case "Sum":
		// Package-level sum function: md5.Sum(data []byte) [16]byte
		// Returns a fixed-size array; model as []byte sentinel.
		digest := make([]byte, digestLen)
		return Value{Raw: digest}, true

	case "Write":
		// h.Write(p []byte) (int, error)
		n := 0
		if len(args) > 1 {
			switch b := args[1].Raw.(type) {
			case []byte:
				n = len(b)
			case []Value:
				n = len(b)
			}
		}
		return Value{Raw: []Value{{Raw: int64(n)}, {}}}, true

	case "Sum32", "Sum64":
		// Sum function variants returning uint32/uint64.
		return Value{Raw: int64(0)}, true

	case "Reset":
		return Value{}, true

	case "Size":
		return Value{Raw: int64(digestLen)}, true

	case "BlockSize":
		// All common hash functions use 64-byte blocks (sha512 uses 128).
		if pkgPath == "crypto/sha512" {
			return Value{Raw: int64(128)}, true
		}
		return Value{Raw: int64(64)}, true
	}
	return Value{}, false
}

// handleFilepathCall models path/filepath.* functions (#83).
func (interp *Interpreter) handleFilepathCall(name string, args []Value) (Value, bool) {
	switch name {
	case "Join":
		var parts []string
		for _, arg := range args {
			if s, ok := arg.Raw.(string); ok {
				parts = append(parts, s)
			} else {
				parts = append(parts, "path")
			}
		}
		return Value{Raw: filepath.Join(parts...)}, true

	case "Dir":
		if s, ok := stdlibArgString(args, 0); ok {
			return Value{Raw: filepath.Dir(s)}, true
		}
		return Value{Raw: "/path"}, true

	case "Base":
		if s, ok := stdlibArgString(args, 0); ok {
			return Value{Raw: filepath.Base(s)}, true
		}
		return Value{Raw: "file"}, true

	case "Ext":
		if s, ok := stdlibArgString(args, 0); ok {
			return Value{Raw: filepath.Ext(s)}, true
		}
		return Value{Raw: ".txt"}, true

	case "Abs":
		if s, ok := stdlibArgString(args, 0); ok {
			a, err := filepath.Abs(s)
			if err != nil {
				return Value{Raw: []Value{{Raw: s}, {Raw: err.Error()}}}, true
			}
			return Value{Raw: []Value{{Raw: a}, {}}}, true
		}
		return Value{Raw: []Value{{Raw: "/tmp/path"}, {}}}, true

	case "Clean":
		if s, ok := stdlibArgString(args, 0); ok {
			return Value{Raw: filepath.Clean(s)}, true
		}
		return Value{Raw: "."}, true

	case "IsAbs":
		if s, ok := stdlibArgString(args, 0); ok {
			return Value{Raw: filepath.IsAbs(s)}, true
		}
		return Value{Raw: false}, true

	case "Split":
		if s, ok := stdlibArgString(args, 0); ok {
			dir, file := filepath.Split(s)
			return Value{Raw: []Value{{Raw: dir}, {Raw: file}}}, true
		}
		return Value{Raw: []Value{{Raw: "/"}, {Raw: "file"}}}, true

	case "Rel":
		if s0, ok0 := stdlibArgString(args, 0); ok0 {
			if s1, ok1 := stdlibArgString(args, 1); ok1 {
				rel, err := filepath.Rel(s0, s1)
				if err != nil {
					return Value{Raw: []Value{{Raw: ""}, {Raw: err.Error()}}}, true
				}
				return Value{Raw: []Value{{Raw: rel}, {}}}, true
			}
		}
		return Value{Raw: []Value{{Raw: "rel/path"}, {}}}, true

	case "Match":
		// filepath.Match(pattern, name string) (matched bool, err error)
		if s0, ok0 := stdlibArgString(args, 0); ok0 {
			if s1, ok1 := stdlibArgString(args, 1); ok1 {
				matched, err := filepath.Match(s0, s1)
				if err != nil {
					return Value{Raw: []Value{{Raw: false}, {Raw: err.Error()}}}, true
				}
				return Value{Raw: []Value{{Raw: matched}, {}}}, true
			}
		}
		return Value{Raw: []Value{{Raw: true}, {}}}, true // pessimistic

	case "Glob":
		// filepath.Glob(pattern string) (matches []string, err error)
		return Value{Raw: []Value{{Raw: []Value{{Raw: "file.txt"}}}, {}}}, true

	case "Walk", "WalkDir":
		// Noop — no filesystem access in the interpreter.
		return Value{}, true

	case "FromSlash", "ToSlash":
		if s, ok := stdlibArgString(args, 0); ok {
			return Value{Raw: s}, true
		}
		return Value{Raw: "path"}, true

	case "VolumeName":
		return Value{Raw: ""}, true

	case "EvalSymlinks":
		if s, ok := stdlibArgString(args, 0); ok {
			return Value{Raw: []Value{{Raw: s}, {}}}, true
		}
		return Value{Raw: []Value{{Raw: "/path"}, {}}}, true

	case "SplitList":
		if s, ok := stdlibArgString(args, 0); ok {
			parts := filepath.SplitList(s)
			vals := make([]Value, len(parts))
			for i, p := range parts {
				vals[i] = Value{Raw: p}
			}
			return Value{Raw: vals}, true
		}
		return Value{Raw: []Value{}}, true
	}
	return Value{}, false
}

// handlePathCall models path.* functions (non-OS, slash-only paths) (#83).
func (interp *Interpreter) handlePathCall(name string, args []Value) (Value, bool) {
	switch name {
	case "Join":
		var parts []string
		for _, arg := range args {
			if s, ok := arg.Raw.(string); ok {
				parts = append(parts, s)
			} else {
				parts = append(parts, "seg")
			}
		}
		// Use filepath.Join then replace OS separator with slash.
		result := strings.ReplaceAll(filepath.Join(parts...), string(filepath.Separator), "/")
		return Value{Raw: result}, true

	case "Dir":
		if s, ok := stdlibArgString(args, 0); ok {
			idx := strings.LastIndex(s, "/")
			if idx < 0 {
				return Value{Raw: "."}, true
			}
			return Value{Raw: s[:idx]}, true
		}
		return Value{Raw: "."}, true

	case "Base":
		if s, ok := stdlibArgString(args, 0); ok {
			idx := strings.LastIndex(s, "/")
			if idx < 0 {
				return Value{Raw: s}, true
			}
			return Value{Raw: s[idx+1:]}, true
		}
		return Value{Raw: "file"}, true

	case "Ext":
		if s, ok := stdlibArgString(args, 0); ok {
			idx := strings.LastIndex(s, ".")
			if idx < 0 {
				return Value{Raw: ""}, true
			}
			return Value{Raw: s[idx:]}, true
		}
		return Value{Raw: ""}, true

	case "Clean":
		if s, ok := stdlibArgString(args, 0); ok {
			return Value{Raw: strings.TrimRight(s, "/")}, true
		}
		return Value{Raw: "."}, true

	case "IsAbs":
		if s, ok := stdlibArgString(args, 0); ok {
			return Value{Raw: strings.HasPrefix(s, "/")}, true
		}
		return Value{Raw: false}, true

	case "Split":
		if s, ok := stdlibArgString(args, 0); ok {
			idx := strings.LastIndex(s, "/")
			if idx < 0 {
				return Value{Raw: []Value{{Raw: ""}, {Raw: s}}}, true
			}
			return Value{Raw: []Value{{Raw: s[:idx+1]}, {Raw: s[idx+1:]}}}, true
		}
		return Value{Raw: []Value{{Raw: "/"}, {Raw: "file"}}}, true

	case "Match":
		return Value{Raw: []Value{{Raw: true}, {}}}, true // pessimistic
	}
	return Value{}, false
}

// handleNetCall models net.* utility functions (#84).
// Full socket/connection model is not implemented; only pure utility functions.
func (interp *Interpreter) handleNetCall(name string, args []Value) (Value, bool) {
	switch name {
	case "SplitHostPort":
		if s, ok := stdlibArgString(args, 0); ok {
			host, port, err := net.SplitHostPort(s)
			if err != nil {
				return Value{Raw: []Value{{Raw: ""}, {Raw: ""}, {Raw: err.Error()}}}, true
			}
			return Value{Raw: []Value{{Raw: host}, {Raw: port}, {}}}, true
		}
		return Value{Raw: []Value{{Raw: "localhost"}, {Raw: "8080"}, {}}}, true

	case "JoinHostPort":
		host, hostOK := stdlibArgString(args, 0)
		port, portOK := stdlibArgString(args, 1)
		if hostOK && portOK {
			return Value{Raw: net.JoinHostPort(host, port)}, true
		}
		return Value{Raw: "localhost:8080"}, true

	case "ParseIP":
		if s, ok := stdlibArgString(args, 0); ok {
			ip := net.ParseIP(s)
			if ip == nil {
				return Value{}, true // nil IP
			}
			return Value{Raw: ip.String()}, true
		}
		return Value{Raw: "127.0.0.1"}, true // sentinel

	case "ParseCIDR":
		if s, ok := stdlibArgString(args, 0); ok {
			ip, ipnet, err := net.ParseCIDR(s)
			if err != nil {
				return Value{Raw: []Value{{}, {}, {Raw: err.Error()}}}, true
			}
			_ = ipnet
			return Value{Raw: []Value{{Raw: ip.String()}, {Raw: struct{}{}}, {}}}, true
		}
		return Value{Raw: []Value{{Raw: "127.0.0.1"}, {Raw: struct{}{}}, {}}}, true

	case "LookupHost":
		// Conservative: do not actually resolve; return sentinel.
		return Value{Raw: []Value{{Raw: []Value{{Raw: "127.0.0.1"}}}, {}}}, true

	case "LookupPort":
		s, _ := stdlibArgString(args, 1)
		port := int64(8080)
		switch s {
		case "http":
			port = 80
		case "https":
			port = 443
		case "ftp":
			port = 21
		case "ssh":
			port = 22
		}
		return Value{Raw: []Value{{Raw: port}, {}}}, true

	case "LookupIP", "LookupTXT", "LookupMX", "LookupNS", "LookupCNAME":
		return Value{Raw: []Value{{Raw: []Value{}}, {}}}, true

	case "ResolveTCPAddr", "ResolveUDPAddr", "ResolveIPAddr", "ResolveUnixAddr":
		// (network, addr string) → (*Addr, error)
		return Value{Raw: []Value{{Raw: struct{}{}}, {}}}, true

	case "Dial", "DialTimeout":
		return Value{Raw: []Value{{Raw: struct{}{}}, {}}}, true

	case "Listen", "ListenPacket":
		return Value{Raw: []Value{{Raw: struct{}{}}, {}}}, true

	case "Pipe":
		opaque := Value{Raw: struct{}{}}
		return Value{Raw: []Value{opaque, opaque}}, true

	case "IPv4", "IPv4Mask":
		return Value{Raw: "0.0.0.0"}, true

	case "CIDRMask":
		return Value{Raw: struct{}{}}, true
	}
	return Value{}, false
}

// handleTemplateCall models text/template and html/template functions (#84).
// Both packages share the same API; html/template escapes output but the
// interpreter models both identically.
func (interp *Interpreter) handleTemplateCall(name string, args []Value) (Value, bool) {
	opaque := Value{Raw: struct{}{}}
	switch name {
	case "New":
		// template.New(name string) *Template
		return opaque, true

	case "Must":
		// template.Must(t *Template, err error) *Template
		// If err is nil return t, otherwise return opaque.
		if len(args) >= 2 && args[1].Raw == nil {
			return args[0], true
		}
		return opaque, true

	case "ParseFiles", "ParseGlob":
		// template.ParseFiles/ParseGlob(...) (*Template, error)
		return Value{Raw: []Value{opaque, {}}}, true

	case "Parse":
		// t.Parse(text string) (*Template, error)
		return Value{Raw: []Value{opaque, {}}}, true

	case "ParseFS":
		return Value{Raw: []Value{opaque, {}}}, true

	case "Execute":
		// t.Execute(wr io.Writer, data interface{}) error — return nil.
		return Value{}, true

	case "ExecuteTemplate":
		// t.ExecuteTemplate(wr io.Writer, name string, data interface{}) error
		return Value{}, true

	case "Funcs":
		// t.Funcs(funcMap FuncMap) *Template — chainable.
		return opaque, true

	case "Delims":
		return opaque, true

	case "Lookup":
		// t.Lookup(name string) *Template
		return opaque, true

	case "Name":
		// t.Name() string
		if s, ok := stdlibArgString(args, 0); ok {
			return Value{Raw: s}, true
		}
		return Value{Raw: "tmpl"}, true

	case "Clone":
		return Value{Raw: []Value{opaque, {}}}, true

	case "Templates":
		// t.Templates() []*Template
		return Value{Raw: []Value{opaque}}, true

	case "Option":
		return opaque, true

	case "HTMLEscape", "HTMLEscapeString", "JSEscape", "JSEscapeString",
		"URLQueryEscaper":
		if s, ok := stdlibArgString(args, 0); ok {
			return Value{Raw: s}, true // pass-through for non-dangerous input
		}
		return Value{Raw: "escaped"}, true
	}
	return Value{}, false
}

// handleXMLCall models encoding/xml.* functions (#87).
func (interp *Interpreter) handleXMLCall(name string, args []Value) (Value, bool) {
	opaque := Value{Raw: struct{}{}}
	switch name {
	case "Marshal":
		// xml.Marshal(v interface{}) ([]byte, error)
		if len(args) > 0 && args[0].Raw != nil {
			b, err := xml.Marshal(args[0].Raw)
			if err == nil {
				return Value{Raw: []Value{{Raw: b}, {}}}, true
			}
		}
		return Value{Raw: []Value{{Raw: []byte("<sentinel/>")}, {}}}, true

	case "MarshalIndent":
		return Value{Raw: []Value{{Raw: []byte("<sentinel/>")}, {}}}, true

	case "Unmarshal":
		// xml.Unmarshal(data []byte, v interface{}) error — return nil error.
		return Value{}, true

	case "NewDecoder":
		return opaque, true

	case "NewEncoder":
		return opaque, true

	case "NewTokenDecoder":
		return opaque, true

	case "Decode":
		// d.Decode(v interface{}) error
		return Value{}, true

	case "DecodeElement":
		return Value{}, true

	case "Token":
		// d.Token() (Token, error)
		return Value{Raw: []Value{opaque, {}}}, true

	case "Encode":
		// e.Encode(v interface{}) error
		return Value{}, true

	case "EncodeElement":
		return Value{}, true

	case "EncodeToken":
		return Value{}, true

	case "Flush":
		return Value{}, true

	case "EscapeText":
		return Value{}, true

	case "Escape":
		return Value{}, true

	case "CopyToken":
		return opaque, true

	case "Name", "Attr", "CharData", "Comment", "ProcInst", "Directive",
		"StartElement", "EndElement":
		// XML token types — return opaque.
		return opaque, true
	}
	return Value{}, false
}

// handleCSVCall models encoding/csv.* functions (#87).
func (interp *Interpreter) handleCSVCall(name string, args []Value) (Value, bool) {
	opaque := Value{Raw: struct{}{}}
	switch name {
	case "NewReader":
		return opaque, true

	case "NewWriter":
		return opaque, true

	case "Read":
		// r.Read() (record []string, err error)
		return Value{Raw: []Value{{Raw: []Value{{Raw: "field1"}, {Raw: "field2"}}}, {}}}, true

	case "ReadAll":
		// r.ReadAll() (records [][]string, err error)
		row := []Value{{Raw: "f1"}, {Raw: "f2"}}
		records := []Value{{Raw: row}}
		return Value{Raw: []Value{{Raw: records}, {}}}, true

	case "Write":
		// w.Write(record []string) error
		return Value{}, true

	case "WriteAll":
		// w.WriteAll(records [][]string) error
		return Value{}, true

	case "Flush":
		return Value{}, true

	case "Error":
		// w.Error() error — return nil
		return Value{}, true
	}
	_ = opaque
	return Value{}, false
}

// handleReflectCall models reflect.* functions (#86).
// Only non-unsafe reflect functions are handled here; Pointer and UnsafeAddr
// are intercepted earlier in exec.go for Rule 5 checking.
func (interp *Interpreter) handleReflectCall(name string, args []Value) (Value, bool) {
	opaque := Value{Raw: struct{}{}} // sentinel for reflect.Type / reflect.Value
	switch name {
	case "TypeOf":
		// reflect.TypeOf(v interface{}) reflect.Type — return opaque non-nil Type.
		return opaque, true

	case "ValueOf":
		// reflect.ValueOf(v interface{}) reflect.Value — return the value as-is.
		if len(args) > 0 {
			return args[0], true
		}
		return opaque, true

	case "DeepEqual":
		// reflect.DeepEqual(x, y interface{}) bool
		if len(args) >= 2 && args[0].Raw != nil && args[1].Raw != nil {
			return Value{Raw: reflect.DeepEqual(args[0].Raw, args[1].Raw)}, true
		}
		return Value{Raw: true}, true // pessimistic: assume equal

	case "New":
		// reflect.New(t reflect.Type) reflect.Value — return opaque pointer.
		return opaque, true

	case "Zero":
		// reflect.Zero(t reflect.Type) reflect.Value
		return opaque, true

	case "MakeSlice":
		// reflect.MakeSlice(t, len, cap int) reflect.Value
		return opaque, true

	case "MakeMap", "MakeMapWithSize":
		return opaque, true

	case "MakeChan":
		return opaque, true

	case "MakeFunc":
		return opaque, true

	case "Append", "AppendSlice":
		return opaque, true

	case "Copy":
		// reflect.Copy(dst, src Value) int
		return Value{Raw: int64(0)}, true

	case "Indirect":
		// reflect.Indirect(v Value) Value
		if len(args) > 0 {
			return args[0], true
		}
		return opaque, true

	case "PtrTo", "PointerTo":
		return opaque, true

	case "SliceOf":
		return opaque, true

	case "ArrayOf":
		return opaque, true

	case "MapOf":
		return opaque, true

	case "ChanOf":
		return opaque, true

	case "FuncOf":
		return opaque, true

	case "StructOf":
		return opaque, true

	// reflect.Value method calls (receiver = args[0]):
	case "Kind":
		// v.Kind() reflect.Kind — return sentinel (Struct=25)
		return Value{Raw: int64(25)}, true

	case "Type":
		return opaque, true

	case "Interface":
		if len(args) > 0 {
			return args[0], true
		}
		return opaque, true

	case "Elem":
		if len(args) > 0 {
			return args[0], true
		}
		return opaque, true

	case "Field":
		return opaque, true

	case "Index":
		return opaque, true

	case "MapIndex":
		return opaque, true

	case "MapKeys":
		return Value{Raw: []Value{}}, true

	case "NumField":
		return Value{Raw: int64(0)}, true

	case "NumMethod":
		return Value{Raw: int64(0)}, true

	case "Method", "MethodByName":
		return opaque, true

	case "Len", "Cap":
		return Value{Raw: int64(0)}, true

	case "IsNil":
		return Value{Raw: false}, true // pessimistic: assume not nil

	case "IsValid":
		return Value{Raw: true}, true // pessimistic: assume valid

	case "IsZero":
		return Value{Raw: false}, true

	case "CanAddr", "CanSet", "CanInterface":
		return Value{Raw: true}, true

	case "Set", "SetInt", "SetUint", "SetFloat", "SetBool", "SetString",
		"SetBytes", "SetCap", "SetLen", "SetPointer", "SetIterKey", "SetIterValue":
		return Value{}, true

	case "Int":
		return Value{Raw: int64(0)}, true

	case "Uint":
		return Value{Raw: int64(0)}, true

	case "Float":
		return Value{Raw: float64(0)}, true

	case "Bool":
		return Value{Raw: false}, true

	case "String":
		return Value{Raw: ""}, true

	case "Bytes":
		return Value{Raw: []byte(nil)}, true

	case "Addr":
		return opaque, true

	case "Call", "CallSlice":
		// v.Call(in []Value) []Value — return empty slice (no actual dispatch).
		return Value{Raw: []Value{}}, true

	case "Convert":
		return opaque, true

	case "Recv":
		return Value{Raw: []Value{opaque, {Raw: false}}}, true

	case "Send":
		return Value{}, true

	case "Close":
		return Value{}, true

	case "TrySend", "TryRecv":
		return Value{Raw: []Value{opaque, {Raw: false}}}, true

	// reflect.Type method calls:
	case "Name":
		return Value{Raw: "T"}, true

	case "PkgPath":
		return Value{Raw: ""}, true

	case "Size":
		return Value{Raw: int64(8)}, true

	case "Implements":
		return Value{Raw: true}, true // pessimistic

	case "AssignableTo", "ConvertibleTo", "Comparable":
		return Value{Raw: true}, true

	case "In":
		return opaque, true

	case "Out":
		return opaque, true

	case "NumIn", "NumOut":
		return Value{Raw: int64(0)}, true

	case "Key":
		return opaque, true

	case "ChanDir":
		return Value{Raw: int64(0)}, true

	case "IsVariadic":
		return Value{Raw: false}, true

	case "Bits":
		return Value{Raw: int64(64)}, true

	case "FieldByName", "FieldByIndex", "FieldByNameFunc":
		return Value{Raw: []Value{opaque, {Raw: false}}}, true

	case "MethodByName2":
		return Value{Raw: []Value{opaque, {Raw: false}}}, true

	case "Align", "FieldAlign":
		return Value{Raw: int64(8)}, true
	}
	return Value{}, false
}

// handleFlagCall models flag.* functions (#88).
// Flag-defined values return opaque non-nil pointers so nil-checks on them pass.
// Parse is a noop; command-line arguments cannot be modelled at analysis time.
func (interp *Interpreter) handleFlagCall(name string, args []Value) (Value, bool) {
	opaque := Value{Raw: struct{}{}}
	switch name {
	// Flag definition functions — return a non-nil pointer to the flag value.
	case "String", "StringVar":
		if name == "StringVar" {
			return Value{}, true // sets *string in place, no return
		}
		return Value{Raw: new(string)}, true
	case "Int", "IntVar":
		if name == "IntVar" {
			return Value{}, true
		}
		return Value{Raw: new(int)}, true
	case "Int64", "Int64Var":
		if name == "Int64Var" {
			return Value{}, true
		}
		return Value{Raw: new(int64)}, true
	case "Uint", "UintVar":
		if name == "UintVar" {
			return Value{}, true
		}
		return Value{Raw: new(uint)}, true
	case "Uint64", "Uint64Var":
		if name == "Uint64Var" {
			return Value{}, true
		}
		return Value{Raw: new(uint64)}, true
	case "Bool", "BoolVar":
		if name == "BoolVar" {
			return Value{}, true
		}
		return Value{Raw: new(bool)}, true
	case "Float64", "Float64Var":
		if name == "Float64Var" {
			return Value{}, true
		}
		return Value{Raw: new(float64)}, true
	case "Duration", "DurationVar":
		if name == "DurationVar" {
			return Value{}, true
		}
		return Value{Raw: new(int64)}, true // time.Duration is int64

	case "Func":
		// flag.Func(name, usage string, fn func(string) error)
		return Value{}, true

	case "TextVar":
		return Value{}, true

	// Parsing
	case "Parse":
		return Value{}, true

	case "Parsed":
		return Value{Raw: true}, true // assume parsed

	// Introspection
	case "Arg":
		return Value{Raw: ""}, true

	case "Args":
		return Value{Raw: []Value{}}, true

	case "NArg", "NFlag":
		return Value{Raw: int64(0)}, true

	case "Lookup":
		// Returns *flag.Flag (nil if not found — conservative).
		return Value{}, true

	case "Set":
		// flag.Set(name, value string) error
		return Value{}, true

	case "Visit", "VisitAll":
		return Value{}, true

	case "PrintDefaults":
		return Value{}, true

	case "Usage":
		return Value{}, true

	case "CommandLine":
		return opaque, true

	case "NewFlagSet":
		return opaque, true

	case "UnquoteUsage":
		return Value{Raw: []Value{{Raw: ""}, {Raw: ""}}}, true
	}
	return Value{}, false
}

// handleRuntimeCall models runtime.* functions (#88).
// Most runtime functions are noops or return sentinel integers.
func (interp *Interpreter) handleRuntimeCall(name string, args []Value) (Value, bool) {
	switch name {
	case "NumCPU":
		return Value{Raw: int64(runtime.NumCPU())}, true

	case "GOMAXPROCS":
		prev := runtime.GOMAXPROCS(0) // query without changing
		if len(args) > 0 {
			if n, ok := stdlibArgInt(args, 0); ok && n > 0 {
				runtime.GOMAXPROCS(int(n))
			}
		}
		return Value{Raw: int64(prev)}, true

	case "NumGoroutine":
		return Value{Raw: int64(1)}, true // model: 1 (our goroutines are virtual)

	case "NumCgoCall":
		return Value{Raw: int64(0)}, true

	case "Caller":
		// runtime.Caller(skip int) (pc uintptr, file string, line int, ok bool)
		return Value{Raw: []Value{
			{Raw: int64(0)},
			{Raw: "giri.go"},
			{Raw: int64(1)},
			{Raw: false}, // conservative: stack not available
		}}, true

	case "Callers":
		// runtime.Callers(skip int, pc []uintptr) int
		return Value{Raw: int64(0)}, true

	case "FuncForPC":
		// Returns *runtime.Func (nil — no debug info available).
		return Value{}, true

	case "CallersFrames":
		// Returns *runtime.Frames (opaque).
		return Value{Raw: struct{}{}}, true

	case "GC":
		return Value{}, true

	case "Gosched":
		return Value{}, true

	case "LockOSThread", "UnlockOSThread":
		return Value{}, true

	case "Goexit":
		// Terminates the current goroutine — noop in the interpreter
		// (the goroutine naturally stops returning from execFunction).
		return Value{}, true

	case "Stack":
		// runtime.Stack(buf []byte, all bool) int
		return Value{Raw: int64(0)}, true

	case "Version":
		return Value{Raw: runtime.Version()}, true

	case "ReadMemStats":
		return Value{}, true

	case "SetFinalizer":
		return Value{}, true

	case "KeepAlive":
		return Value{}, true

	case "SetBlockProfileRate", "SetMutexProfileFraction", "SetCPUProfileRate":
		return Value{}, true

	case "Breakpoint":
		return Value{}, true

	case "SetPanicOnFault":
		return Value{Raw: false}, true

	case "GOARCH":
		return Value{Raw: runtime.GOARCH}, true

	case "GOOS":
		return Value{Raw: runtime.GOOS}, true

	case "GOROOT":
		return Value{Raw: runtime.GOROOT()}, true
	}
	return Value{}, false
}

// handleNetURLCall models net/url.* functions (#89).
func (interp *Interpreter) handleNetURLCall(name string, args []Value) (Value, bool) {
	opaque := Value{Raw: struct{}{}}

	switch name {
	case "Parse", "ParseRequestURI":
		// url.Parse(rawurl string) (*URL, error)
		if s, ok := stdlibArgString(args, 0); ok {
			u, err := url.Parse(s)
			if err != nil {
				return Value{Raw: []Value{{}, {Raw: err.Error()}}}, true
			}
			// Return the URL as an opaque value; downstream field accesses
			// (Scheme, Host, Path, etc.) are intercepted as method calls.
			return Value{Raw: []Value{{Raw: u}, {}}}, true
		}
		return Value{Raw: []Value{opaque, {}}}, true

	case "ParseQuery":
		// url.ParseQuery(query string) (Values, error)
		if s, ok := stdlibArgString(args, 0); ok {
			vals, err := url.ParseQuery(s)
			if err != nil {
				return Value{Raw: []Value{{}, {Raw: err.Error()}}}, true
			}
			// Model Values as opaque; downstream Get/Set calls are intercepted.
			return Value{Raw: []Value{{Raw: vals}, {}}}, true
		}
		return Value{Raw: []Value{opaque, {}}}, true

	case "QueryEscape":
		if s, ok := stdlibArgString(args, 0); ok {
			return Value{Raw: url.QueryEscape(s)}, true
		}
		return Value{Raw: "escaped"}, true

	case "QueryUnescape":
		if s, ok := stdlibArgString(args, 0); ok {
			u, err := url.QueryUnescape(s)
			if err != nil {
				return Value{Raw: []Value{{Raw: ""}, {Raw: err.Error()}}}, true
			}
			return Value{Raw: []Value{{Raw: u}, {}}}, true
		}
		return Value{Raw: []Value{{Raw: "unescaped"}, {}}}, true

	case "PathEscape":
		if s, ok := stdlibArgString(args, 0); ok {
			return Value{Raw: url.PathEscape(s)}, true
		}
		return Value{Raw: "escaped"}, true

	case "PathUnescape":
		if s, ok := stdlibArgString(args, 0); ok {
			u, err := url.PathUnescape(s)
			if err != nil {
				return Value{Raw: []Value{{Raw: ""}, {Raw: err.Error()}}}, true
			}
			return Value{Raw: []Value{{Raw: u}, {}}}, true
		}
		return Value{Raw: []Value{{Raw: "unescaped"}, {}}}, true

	case "User":
		// url.User(username string) *Userinfo
		return opaque, true

	case "UserPassword":
		// url.UserPassword(username, password string) *Userinfo
		return opaque, true

	case "JoinPath":
		// url.JoinPath(base string, elem ...string) (string, error)
		if s, ok := stdlibArgString(args, 0); ok {
			parts := []string{s}
			for _, a := range args[1:] {
				if p, ok2 := a.Raw.(string); ok2 {
					parts = append(parts, p)
				}
			}
			return Value{Raw: []Value{{Raw: strings.Join(parts, "/")}, {}}}, true
		}
		return Value{Raw: []Value{{Raw: "http://example.com/path"}, {}}}, true

	// *url.URL method calls (receiver = args[0]):
	case "String":
		if len(args) > 0 {
			if u, ok := args[0].Raw.(*url.URL); ok {
				return Value{Raw: u.String()}, true
			}
		}
		return Value{Raw: "http://example.com"}, true

	case "Scheme":
		if len(args) > 0 {
			if u, ok := args[0].Raw.(*url.URL); ok {
				return Value{Raw: u.Scheme}, true
			}
		}
		return Value{Raw: "https"}, true

	case "Host":
		if len(args) > 0 {
			if u, ok := args[0].Raw.(*url.URL); ok {
				return Value{Raw: u.Host}, true
			}
		}
		return Value{Raw: "example.com"}, true

	case "Path":
		if len(args) > 0 {
			if u, ok := args[0].Raw.(*url.URL); ok {
				return Value{Raw: u.Path}, true
			}
		}
		return Value{Raw: "/path"}, true

	case "RawQuery":
		if len(args) > 0 {
			if u, ok := args[0].Raw.(*url.URL); ok {
				return Value{Raw: u.RawQuery}, true
			}
		}
		return Value{Raw: "key=value"}, true

	case "Fragment":
		if len(args) > 0 {
			if u, ok := args[0].Raw.(*url.URL); ok {
				return Value{Raw: u.Fragment}, true
			}
		}
		return Value{Raw: ""}, true

	case "Query":
		// u.Query() url.Values
		if len(args) > 0 {
			if u, ok := args[0].Raw.(*url.URL); ok {
				return Value{Raw: u.Query()}, true
			}
		}
		return opaque, true

	case "Hostname":
		if len(args) > 0 {
			if u, ok := args[0].Raw.(*url.URL); ok {
				return Value{Raw: u.Hostname()}, true
			}
		}
		return Value{Raw: "example.com"}, true

	case "Port":
		if len(args) > 0 {
			if u, ok := args[0].Raw.(*url.URL); ok {
				return Value{Raw: u.Port()}, true
			}
		}
		return Value{Raw: ""}, true

	case "RequestURI":
		if len(args) > 0 {
			if u, ok := args[0].Raw.(*url.URL); ok {
				return Value{Raw: u.RequestURI()}, true
			}
		}
		return Value{Raw: "/path?key=value"}, true

	case "ResolveReference":
		return opaque, true

	case "IsAbs":
		if len(args) > 0 {
			if u, ok := args[0].Raw.(*url.URL); ok {
				return Value{Raw: u.IsAbs()}, true
			}
		}
		return Value{Raw: true}, true

	case "MarshalBinary":
		return Value{Raw: []Value{{Raw: []byte("url")}, {}}}, true

	case "UnmarshalBinary":
		return Value{}, true

	case "EscapedPath":
		return Value{Raw: "/path"}, true

	case "EscapedFragment":
		return Value{Raw: ""}, true

	// url.Values method calls (receiver = args[0]):
	case "Get":
		if len(args) >= 2 {
			if vals, ok := args[0].Raw.(url.Values); ok {
				if k, ok2 := stdlibArgString(args, 1); ok2 {
					return Value{Raw: vals.Get(k)}, true
				}
			}
		}
		return Value{Raw: "value"}, true

	case "Set":
		if len(args) >= 3 {
			if vals, ok := args[0].Raw.(url.Values); ok {
				k, _ := stdlibArgString(args, 1)
				v, _ := stdlibArgString(args, 2)
				vals.Set(k, v)
			}
		}
		return Value{}, true

	case "Add":
		return Value{}, true

	case "Del":
		return Value{}, true

	case "Has":
		return Value{Raw: true}, true // pessimistic

	case "Encode":
		if len(args) > 0 {
			if vals, ok := args[0].Raw.(url.Values); ok {
				return Value{Raw: vals.Encode()}, true
			}
		}
		return Value{Raw: "key=value"}, true
	}
	return Value{}, false
}

// handleExecCall models os/exec.* functions (#90).
func (interp *Interpreter) handleExecCall(name string, args []Value) (Value, bool) {
	opaque := Value{Raw: struct{}{}}
	switch name {
	case "Command", "CommandContext":
		// exec.Command(name string, arg ...string) *Cmd
		return opaque, true

	case "LookPath":
		// exec.LookPath(file string) (string, error)
		if s, ok := stdlibArgString(args, 0); ok {
			return Value{Raw: []Value{{Raw: "/usr/bin/" + s}, {}}}, true
		}
		return Value{Raw: []Value{{Raw: "/usr/bin/cmd"}, {}}}, true

	// *exec.Cmd method calls (receiver = args[0]):
	case "Run":
		// cmd.Run() error — return nil (assume success)
		return Value{}, true

	case "Output":
		// cmd.Output() ([]byte, error)
		return Value{Raw: []Value{{Raw: []byte("output")}, {}}}, true

	case "CombinedOutput":
		// cmd.CombinedOutput() ([]byte, error)
		return Value{Raw: []Value{{Raw: []byte("output")}, {}}}, true

	case "Start":
		// cmd.Start() error
		return Value{}, true

	case "Wait":
		// cmd.Wait() error
		return Value{}, true

	case "StdoutPipe":
		// cmd.StdoutPipe() (io.ReadCloser, error)
		return Value{Raw: []Value{opaque, {}}}, true

	case "StderrPipe":
		// cmd.StderrPipe() (io.ReadCloser, error)
		return Value{Raw: []Value{opaque, {}}}, true

	case "StdinPipe":
		// cmd.StdinPipe() (io.WriteCloser, error)
		return Value{Raw: []Value{opaque, {}}}, true

	case "String":
		// cmd.String() string
		return Value{Raw: "cmd"}, true

	case "Environ":
		// cmd.Environ() []string
		return Value{Raw: []Value{}}, true
	}
	return Value{}, false
}

// handleGzipCall models compress/gzip.* functions (#91).
func (interp *Interpreter) handleGzipCall(name string, args []Value) (Value, bool) {
	opaque := Value{Raw: struct{}{}}
	switch name {
	case "NewReader":
		// gzip.NewReader(r io.Reader) (*Reader, error)
		return Value{Raw: []Value{opaque, {}}}, true

	case "NewWriter":
		// gzip.NewWriter(w io.Writer) *Writer — single return value.
		return opaque, true

	case "NewWriterLevel":
		// gzip.NewWriterLevel(w io.Writer, level int) (*Writer, error)
		return Value{Raw: []Value{opaque, {}}}, true

	case "BestCompression", "BestSpeed", "DefaultCompression", "HuffmanOnly",
		"NoCompression":
		// gzip level constants — should not be called but handle gracefully.
		return Value{Raw: int64(-1)}, true

	// *gzip.Reader method calls:
	case "Read":
		// r.Read(p []byte) (int, error) — return EOF immediately.
		return Value{Raw: []Value{{Raw: int64(0)}, {Raw: "EOF"}}}, true

	case "Close":
		return Value{}, true

	case "Reset":
		return Value{}, true

	case "Multistream":
		return Value{}, true

	// *gzip.Writer method calls:
	case "Write":
		n := 0
		if len(args) >= 2 {
			switch b := args[1].Raw.(type) {
			case []byte:
				n = len(b)
			case []Value:
				n = len(b)
			}
		}
		return Value{Raw: []Value{{Raw: int64(n)}, {}}}, true

	case "Flush":
		return Value{}, true
	}
	_ = opaque
	return Value{}, false
}

// handleZlibCall models compress/zlib.* functions (#91).
func (interp *Interpreter) handleZlibCall(name string, args []Value) (Value, bool) {
	opaque := Value{Raw: struct{}{}}
	switch name {
	case "NewReader":
		// zlib.NewReader(r io.Reader) (io.ReadCloser, error)
		return Value{Raw: []Value{opaque, {}}}, true

	case "NewReaderDict":
		// zlib.NewReaderDict(r io.Reader, dict []byte) (io.ReadCloser, error)
		return Value{Raw: []Value{opaque, {}}}, true

	case "NewWriter":
		// zlib.NewWriter(w io.Writer) *Writer — single return value.
		return opaque, true

	case "NewWriterLevel":
		// zlib.NewWriterLevel(w io.Writer, level int) (*Writer, error)
		return Value{Raw: []Value{opaque, {}}}, true

	case "NewWriterLevelDict":
		return Value{Raw: []Value{opaque, {}}}, true

	// Shared io.ReadCloser / *Writer methods:
	case "Read":
		return Value{Raw: []Value{{Raw: int64(0)}, {Raw: "EOF"}}}, true

	case "Close":
		return Value{}, true

	case "Reset":
		return Value{}, true

	case "Write":
		n := 0
		if len(args) >= 2 {
			switch b := args[1].Raw.(type) {
			case []byte:
				n = len(b)
			case []Value:
				n = len(b)
			}
		}
		return Value{Raw: []Value{{Raw: int64(n)}, {}}}, true

	case "Flush":
		return Value{}, true
	}
	_ = opaque
	return Value{}, false
}

// valuesToInterfaces converts []Value to []interface{} for real fmt.* calls.
// Non-concrete values (Raw == nil) are rendered as "?" to avoid nil-format panics.
func valuesToInterfaces(vals []Value) []interface{} {
	result := make([]interface{}, len(vals))
	for i, v := range vals {
		if v.Raw != nil {
			result[i] = v.Raw
		} else {
			result[i] = "?"
		}
	}
	return result
}
