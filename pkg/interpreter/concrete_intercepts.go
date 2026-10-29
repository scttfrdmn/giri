// Package interpreter - concrete_intercepts.go is the table of stdlib
// functions modeled concretely in concrete mode.
//
// Each entry wraps the real stdlib function, so a concrete call computes
// exactly what native code computes. The table is the source of truth for
// IsConcreteIntercept: a function is concrete if and only if it is listed
// here. Everything else reached in concrete mode falls through to the CLI
// intercepts in stdlib.go, which are allowed to be approximate or opaque
// (placeholder results), and is therefore reported as not concrete.
package interpreter

import (
	"fmt"
	"go/types"
	"math"
	"math/bits"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// concreteImpl is one concretely modeled function.
type concreteImpl struct {
	// pure is set when the function has no side effects (reads and writes no
	// interpreter memory); IsPure further requires scalar operands/result.
	pure bool
	fn   func(interp *Interpreter, args []Value) Value
}

// call runs the implementation, converting a native panic (e.g. bits.Div64
// overflow, strings.Repeat with a negative count) into an interpreted panic
// on the current goroutine, as the native call would.
func (c *concreteImpl) call(interp *Interpreter, gid int64, args []Value) (result Value) {
	defer func() {
		if r := recover(); r != nil {
			if g := interp.goroutines[gid]; g != nil {
				g.PanicValue = Value{Raw: fmt.Sprint(r)}
				g.Panicking = true
			}
			result = Value{}
		}
	}()
	return c.fn(interp, args)
}

// lookupConcrete returns the concrete implementation of pkg.name, or nil.
func lookupConcrete(pkg, name string) *concreteImpl {
	if fns, ok := concreteTable[pkg]; ok {
		return fns[name]
	}
	return nil
}

// IsConcreteIntercept reports whether the stdlib function pkg.name is modeled
// concretely (computes the same result as native code) in concrete mode.
func IsConcreteIntercept(pkg, name string) bool {
	return lookupConcrete(pkg, name) != nil
}

// ConcreteIntercepts lists every concretely modeled function as
// "pkg/path.Name", sorted.
func ConcreteIntercepts() []string {
	var out []string
	for pkg, fns := range concreteTable {
		for name := range fns {
			out = append(out, pkg+"."+name)
		}
	}
	sort.Strings(out)
	return out
}

// --- Scalar marshaling for native-backed adapters ---

type scalar interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64 |
		~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 | ~uintptr |
		~float32 | ~float64 | ~bool | ~string
}

func argAt(args []Value, i int) Value {
	if i < len(args) {
		return args[i]
	}
	return Value{}
}

// toNative converts an interpreter scalar to T.
func toNative[T scalar](v Value) T {
	var z T
	switch p := any(&z).(type) {
	case *int:
		*p = int(toInt64(v))
	case *int8:
		*p = int8(toInt64(v))
	case *int16:
		*p = int16(toInt64(v))
	case *int32:
		*p = int32(toInt64(v))
	case *int64:
		*p = toInt64(v)
	case *uint:
		*p = uint(toInt64(v))
	case *uint8:
		*p = uint8(toInt64(v))
	case *uint16:
		*p = uint16(toInt64(v))
	case *uint32:
		*p = uint32(toInt64(v))
	case *uint64:
		*p = uint64(toInt64(v))
	case *uintptr:
		*p = uintptr(toInt64(v))
	case *float32:
		f, _ := v.Raw.(float64)
		*p = float32(f)
	case *float64:
		f, _ := v.Raw.(float64)
		*p = f
	case *bool:
		b, _ := v.Raw.(bool)
		*p = b
	case *string:
		s, _ := v.Raw.(string)
		*p = s
	}
	return z
}

// fromNative converts a native scalar to the interpreter representation.
func fromNative[T scalar](x T) Value {
	switch v := any(x).(type) {
	case int:
		return Value{Raw: int64(v)}
	case int8:
		return Value{Raw: int64(v)}
	case int16:
		return Value{Raw: int64(v)}
	case int32:
		return Value{Raw: int64(v)}
	case int64:
		return Value{Raw: v}
	case uint:
		return Value{Raw: int64(v)}
	case uint8:
		return Value{Raw: int64(v)}
	case uint16:
		return Value{Raw: int64(v)}
	case uint32:
		return Value{Raw: int64(v)}
	case uint64:
		return Value{Raw: int64(v)}
	case uintptr:
		return Value{Raw: int64(v)}
	case float32:
		return Value{Raw: float64(v)}
	case float64:
		return Value{Raw: v}
	case bool:
		return Value{Raw: v}
	case string:
		return Value{Raw: v}
	}
	return Value{}
}

func f0[R scalar](f func() R) *concreteImpl {
	return &concreteImpl{pure: true, fn: func(_ *Interpreter, _ []Value) Value { return fromNative(f()) }}
}

func f1[A, R scalar](f func(A) R) *concreteImpl {
	return &concreteImpl{pure: true, fn: func(_ *Interpreter, a []Value) Value {
		return fromNative(f(toNative[A](argAt(a, 0))))
	}}
}

func f2[A, B, R scalar](f func(A, B) R) *concreteImpl {
	return &concreteImpl{pure: true, fn: func(_ *Interpreter, a []Value) Value {
		return fromNative(f(toNative[A](argAt(a, 0)), toNative[B](argAt(a, 1))))
	}}
}

func f3[A, B, C, R scalar](f func(A, B, C) R) *concreteImpl {
	return &concreteImpl{pure: true, fn: func(_ *Interpreter, a []Value) Value {
		return fromNative(f(toNative[A](argAt(a, 0)), toNative[B](argAt(a, 1)), toNative[C](argAt(a, 2))))
	}}
}

func f4[A, B, C, D, R scalar](f func(A, B, C, D) R) *concreteImpl {
	return &concreteImpl{pure: true, fn: func(_ *Interpreter, a []Value) Value {
		return fromNative(f(toNative[A](argAt(a, 0)), toNative[B](argAt(a, 1)), toNative[C](argAt(a, 2)), toNative[D](argAt(a, 3))))
	}}
}

func tuple2[R1, R2 scalar](r1 R1, r2 R2) Value {
	return Value{Raw: []Value{fromNative(r1), fromNative(r2)}}
}

func f1r2[A, R1, R2 scalar](f func(A) (R1, R2)) *concreteImpl {
	return &concreteImpl{pure: true, fn: func(_ *Interpreter, a []Value) Value {
		return tuple2(f(toNative[A](argAt(a, 0))))
	}}
}

func f2r2[A, B, R1, R2 scalar](f func(A, B) (R1, R2)) *concreteImpl {
	return &concreteImpl{pure: true, fn: func(_ *Interpreter, a []Value) Value {
		return tuple2(f(toNative[A](argAt(a, 0)), toNative[B](argAt(a, 1))))
	}}
}

func f3r2[A, B, C, R1, R2 scalar](f func(A, B, C) (R1, R2)) *concreteImpl {
	return &concreteImpl{pure: true, fn: func(_ *Interpreter, a []Value) Value {
		return tuple2(f(toNative[A](argAt(a, 0)), toNative[B](argAt(a, 1)), toNative[C](argAt(a, 2))))
	}}
}

// --- Slice-backed (impure) adapters for package sort ---

var (
	tInt     = types.Typ[types.Int]
	tFloat64 = types.Typ[types.Float64]
	tString  = types.Typ[types.String]
)

// sliceOf reads args[0] as a []T held in interpreter memory.
func sliceOf[T scalar](interp *Interpreter, v Value, elem types.Type) ([]T, *SliceValue) {
	sv, _ := v.Raw.(*SliceValue)
	elems, _, _ := interp.sliceElems(v, elem)
	out := make([]T, len(elems))
	for i, e := range elems {
		out[i] = toNative[T](e)
	}
	return out, sv
}

func sortIn[T scalar](elem types.Type, sortFn func([]T)) *concreteImpl {
	return &concreteImpl{fn: func(interp *Interpreter, a []Value) Value {
		xs, sv := sliceOf[T](interp, argAt(a, 0), elem)
		if sv == nil {
			return Value{}
		}
		sortFn(xs)
		size := interp.typeSizeOf(elem)
		for i, x := range xs {
			interp.setElem(sv, i, size, fromNative(x))
		}
		return Value{}
	}}
}

func sortedQ[T scalar](elem types.Type, q func([]T) bool) *concreteImpl {
	return &concreteImpl{fn: func(interp *Interpreter, a []Value) Value {
		xs, _ := sliceOf[T](interp, argAt(a, 0), elem)
		return Value{Raw: q(xs)}
	}}
}

func searchIn[T scalar](elem types.Type, s func([]T, T) int) *concreteImpl {
	return &concreteImpl{fn: func(interp *Interpreter, a []Value) Value {
		xs, _ := sliceOf[T](interp, argAt(a, 0), elem)
		return Value{Raw: int64(s(xs, toNative[T](argAt(a, 1))))}
	}}
}

// concreteTable maps package path → function name → implementation.
var concreteTable = map[string]map[string]*concreteImpl{
	"math": {
		"Abs": f1(math.Abs), "Acos": f1(math.Acos), "Acosh": f1(math.Acosh),
		"Asin": f1(math.Asin), "Asinh": f1(math.Asinh), "Atan": f1(math.Atan),
		"Atan2": f2(math.Atan2), "Atanh": f1(math.Atanh), "Cbrt": f1(math.Cbrt),
		"Ceil": f1(math.Ceil), "Copysign": f2(math.Copysign), "Cos": f1(math.Cos),
		"Cosh": f1(math.Cosh), "Dim": f2(math.Dim), "Erf": f1(math.Erf),
		"Erfc": f1(math.Erfc), "Erfcinv": f1(math.Erfcinv), "Erfinv": f1(math.Erfinv),
		"Exp": f1(math.Exp), "Exp2": f1(math.Exp2), "Expm1": f1(math.Expm1),
		"FMA": f3(math.FMA), "Float32bits": f1(math.Float32bits),
		"Float32frombits": f1(math.Float32frombits), "Float64bits": f1(math.Float64bits),
		"Float64frombits": f1(math.Float64frombits), "Floor": f1(math.Floor),
		"Frexp": f1r2(math.Frexp), "Gamma": f1(math.Gamma), "Hypot": f2(math.Hypot),
		"Ilogb": f1(math.Ilogb), "Inf": f1(math.Inf), "IsInf": f2(math.IsInf),
		"IsNaN": f1(math.IsNaN), "J0": f1(math.J0), "J1": f1(math.J1), "Jn": f2(math.Jn),
		"Ldexp": f2(math.Ldexp), "Lgamma": f1r2(math.Lgamma), "Log": f1(math.Log),
		"Log10": f1(math.Log10), "Log1p": f1(math.Log1p), "Log2": f1(math.Log2),
		"Logb": f1(math.Logb), "Max": f2(math.Max), "Min": f2(math.Min),
		"Mod": f2(math.Mod), "Modf": f1r2(math.Modf), "NaN": f0(math.NaN),
		"Nextafter": f2(math.Nextafter), "Nextafter32": f2(math.Nextafter32),
		"Pow": f2(math.Pow), "Pow10": f1(math.Pow10), "Remainder": f2(math.Remainder),
		"Round": f1(math.Round), "RoundToEven": f1(math.RoundToEven),
		"Signbit": f1(math.Signbit), "Sin": f1(math.Sin), "Sincos": f1r2(math.Sincos),
		"Sinh": f1(math.Sinh), "Sqrt": f1(math.Sqrt), "Tan": f1(math.Tan),
		"Tanh": f1(math.Tanh), "Trunc": f1(math.Trunc), "Y0": f1(math.Y0),
		"Y1": f1(math.Y1), "Yn": f2(math.Yn),
	},
	"math/bits": {
		"Add": f3r2(bits.Add), "Add32": f3r2(bits.Add32), "Add64": f3r2(bits.Add64),
		"Div": f3r2(bits.Div), "Div32": f3r2(bits.Div32), "Div64": f3r2(bits.Div64),
		"LeadingZeros": f1(bits.LeadingZeros), "LeadingZeros8": f1(bits.LeadingZeros8),
		"LeadingZeros16": f1(bits.LeadingZeros16), "LeadingZeros32": f1(bits.LeadingZeros32),
		"LeadingZeros64": f1(bits.LeadingZeros64), "Len": f1(bits.Len), "Len8": f1(bits.Len8),
		"Len16": f1(bits.Len16), "Len32": f1(bits.Len32), "Len64": f1(bits.Len64),
		"Mul": f2r2(bits.Mul), "Mul32": f2r2(bits.Mul32), "Mul64": f2r2(bits.Mul64),
		"OnesCount": f1(bits.OnesCount), "OnesCount8": f1(bits.OnesCount8),
		"OnesCount16": f1(bits.OnesCount16), "OnesCount32": f1(bits.OnesCount32),
		"OnesCount64": f1(bits.OnesCount64), "Rem": f3(bits.Rem), "Rem32": f3(bits.Rem32),
		"Rem64": f3(bits.Rem64), "Reverse": f1(bits.Reverse), "Reverse8": f1(bits.Reverse8),
		"Reverse16": f1(bits.Reverse16), "Reverse32": f1(bits.Reverse32),
		"Reverse64": f1(bits.Reverse64), "ReverseBytes": f1(bits.ReverseBytes),
		"ReverseBytes16": f1(bits.ReverseBytes16), "ReverseBytes32": f1(bits.ReverseBytes32),
		"ReverseBytes64": f1(bits.ReverseBytes64), "RotateLeft": f2(bits.RotateLeft),
		"RotateLeft8": f2(bits.RotateLeft8), "RotateLeft16": f2(bits.RotateLeft16),
		"RotateLeft32": f2(bits.RotateLeft32), "RotateLeft64": f2(bits.RotateLeft64),
		"Sub": f3r2(bits.Sub), "Sub32": f3r2(bits.Sub32), "Sub64": f3r2(bits.Sub64),
		"TrailingZeros": f1(bits.TrailingZeros), "TrailingZeros8": f1(bits.TrailingZeros8),
		"TrailingZeros16": f1(bits.TrailingZeros16), "TrailingZeros32": f1(bits.TrailingZeros32),
		"TrailingZeros64": f1(bits.TrailingZeros64),
	},
	"strings": {
		"Compare": f2(strings.Compare), "Contains": f2(strings.Contains),
		"ContainsAny": f2(strings.ContainsAny), "ContainsRune": f2(strings.ContainsRune),
		"Count": f2(strings.Count), "CutPrefix": f2r2(strings.CutPrefix),
		"CutSuffix": f2r2(strings.CutSuffix), "EqualFold": f2(strings.EqualFold),
		"HasPrefix": f2(strings.HasPrefix), "HasSuffix": f2(strings.HasSuffix),
		"Index": f2(strings.Index), "IndexAny": f2(strings.IndexAny),
		"IndexByte": f2(strings.IndexByte), "IndexRune": f2(strings.IndexRune),
		"LastIndex": f2(strings.LastIndex), "LastIndexAny": f2(strings.LastIndexAny),
		"LastIndexByte": f2(strings.LastIndexByte), "Repeat": f2(strings.Repeat),
		"Replace": f4(strings.Replace), "ReplaceAll": f3(strings.ReplaceAll),
		"ToLower": f1(strings.ToLower), "ToTitle": f1(strings.ToTitle),
		"ToUpper": f1(strings.ToUpper), "ToValidUTF8": f2(strings.ToValidUTF8),
		"Trim": f2(strings.Trim), "TrimLeft": f2(strings.TrimLeft),
		"TrimPrefix": f2(strings.TrimPrefix), "TrimRight": f2(strings.TrimRight),
		"TrimSpace": f1(strings.TrimSpace), "TrimSuffix": f2(strings.TrimSuffix),
	},
	"strconv": {
		"FormatBool": f1(strconv.FormatBool), "FormatFloat": f4(strconv.FormatFloat),
		"FormatInt": f2(strconv.FormatInt), "FormatUint": f2(strconv.FormatUint),
		"IsGraphic": f1(strconv.IsGraphic), "IsPrint": f1(strconv.IsPrint),
		"Itoa": f1(strconv.Itoa), "Quote": f1(strconv.Quote),
		"QuoteRune": f1(strconv.QuoteRune), "QuoteRuneToASCII": f1(strconv.QuoteRuneToASCII),
		"QuoteToASCII": f1(strconv.QuoteToASCII),
	},
	"unicode": {
		"IsControl": f1(unicode.IsControl), "IsDigit": f1(unicode.IsDigit),
		"IsGraphic": f1(unicode.IsGraphic), "IsLetter": f1(unicode.IsLetter),
		"IsLower": f1(unicode.IsLower), "IsMark": f1(unicode.IsMark),
		"IsNumber": f1(unicode.IsNumber), "IsPrint": f1(unicode.IsPrint),
		"IsPunct": f1(unicode.IsPunct), "IsSpace": f1(unicode.IsSpace),
		"IsSymbol": f1(unicode.IsSymbol), "IsTitle": f1(unicode.IsTitle),
		"IsUpper": f1(unicode.IsUpper), "SimpleFold": f1(unicode.SimpleFold),
		"To": f2(unicode.To), "ToLower": f1(unicode.ToLower),
		"ToTitle": f1(unicode.ToTitle), "ToUpper": f1(unicode.ToUpper),
	},
	"unicode/utf8": {
		"DecodeLastRuneInString": f1r2(utf8.DecodeLastRuneInString),
		"DecodeRuneInString":     f1r2(utf8.DecodeRuneInString),
		"FullRuneInString":       f1(utf8.FullRuneInString),
		"RuneCountInString":      f1(utf8.RuneCountInString),
		"RuneLen":                f1(utf8.RuneLen),
		"ValidRune":              f1(utf8.ValidRune),
		"ValidString":            f1(utf8.ValidString),
	},
	"sort": {
		"Float64s":          sortIn[float64](tFloat64, sort.Float64s),
		"Float64sAreSorted": sortedQ[float64](tFloat64, sort.Float64sAreSorted),
		"Ints":              sortIn[int](tInt, sort.Ints),
		"IntsAreSorted":     sortedQ[int](tInt, sort.IntsAreSorted),
		"SearchFloat64s":    searchIn[float64](tFloat64, sort.SearchFloat64s),
		"SearchInts":        searchIn[int](tInt, sort.SearchInts),
		"SearchStrings":     searchIn[string](tString, sort.SearchStrings),
		"Strings":           sortIn[string](tString, sort.Strings),
		"StringsAreSorted":  sortedQ[string](tString, sort.StringsAreSorted),
	},
}
