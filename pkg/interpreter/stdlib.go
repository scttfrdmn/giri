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
	"fmt"
	"math/rand"
	"strconv"
	"strings"

	"golang.org/x/tools/go/ssa"
)

// execStdlibCall intercepts standard library function calls in packages
// "strings", "strconv", "fmt", "time", "os", "math/rand", "bytes",
// "errors", and "sort". Returns (result, true) when intercepted,
// (Value{}, false) otherwise.
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
