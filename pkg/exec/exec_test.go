package exec

import (
	"errors"
	"math"
	"reflect"
	"sync"
	"testing"

	"golang.org/x/tools/go/ssa"

	"github.com/scttfrdmn/giri/pkg/exec/internal/kernels"
)

var (
	kernelsOnce sync.Once
	kernelsPkg  *ssa.Package
	kernelsErr  error
)

// kernelFunc returns the SSA function name from internal/kernels.
func kernelFunc(t testing.TB, name string) *ssa.Function {
	t.Helper()
	kernelsOnce.Do(func() { kernelsPkg, kernelsErr = loadDir("internal/kernels") })
	if kernelsErr != nil {
		t.Fatal(kernelsErr)
	}
	fn := kernelsPkg.Func(name)
	if fn == nil {
		t.Fatalf("no function %s", name)
	}
	return fn
}

// nativeCall calls f with args via reflection and returns its results.
func nativeCall(f any, args []any) []any {
	in := make([]reflect.Value, len(args))
	ft := reflect.TypeOf(f)
	for i, a := range args {
		if a == nil {
			in[i] = reflect.Zero(ft.In(i))
		} else {
			in[i] = reflect.ValueOf(a)
		}
	}
	var out []any
	for _, r := range reflect.ValueOf(f).Call(in) {
		out = append(out, r.Interface())
	}
	if out == nil {
		out = []any{}
	}
	return out
}

// cloneArgs deep-copies slice arguments so a native call that mutates its
// input (Reverse) cannot affect the interpreted call.
func cloneArgs(args []any) []any {
	out := make([]any, len(args))
	for i, a := range args {
		rv := reflect.ValueOf(a)
		if a != nil && rv.Kind() == reflect.Slice && !rv.IsNil() {
			c := reflect.MakeSlice(rv.Type(), rv.Len(), rv.Len())
			reflect.Copy(c, rv)
			out[i] = c.Interface()
		} else {
			out[i] = a
		}
	}
	return out
}

func TestRoundTripMatchesNative(t *testing.T) {
	cases := []struct {
		name   string
		native any
		args   [][]any
	}{
		{"AddInt", kernels.AddInt, [][]any{{1, 2}, {math.MaxInt, 1}, {-5, -7}}},
		{"Mix8", kernels.Mix8, [][]any{{int8(100), int8(3)}, {int8(-128), int8(-1)}, {int8(7), int8(127)}}},
		{"MixU8", kernels.MixU8, [][]any{{uint8(200), uint8(3)}, {uint8(255), uint8(255)}}},
		{"Mix16", kernels.Mix16, [][]any{{int16(30000), int16(9)}, {int16(-1), int16(-32768)}}},
		{"MixU16", kernels.MixU16, [][]any{{uint16(65535), uint16(2)}, {uint16(0), uint16(0)}}},
		{"Mix32", kernels.Mix32, [][]any{{int32(math.MaxInt32), int32(3)}, {int32(-77), int32(5)}}},
		{"MixU32", kernels.MixU32, [][]any{{uint32(0xDEADBEEF), uint32(7)}, {uint32(1), uint32(math.MaxUint32)}}},
		{"Mix64", kernels.Mix64, [][]any{{int64(math.MinInt64), int64(-1)}, {int64(123456789), int64(987654321)}}},
		{"MixU64", kernels.MixU64, [][]any{{uint64(math.MaxUint64), uint64(3)}, {uint64(5), uint64(1 << 63)}, {uint64(9), uint64(9)}}},
		{"Shifts", kernels.Shifts, [][]any{{uint32(0x80000001), uint(1)}, {uint32(0xF0F0F0F0), uint(31)}, {uint32(5), uint(40)}}},
		{"Poly32", kernels.Poly32, [][]any{{float32(1.1)}, {float32(-3.75)}, {float32(1e10)}}},
		{"Poly64", kernels.Poly64, [][]any{{1.1}, {-1e-3}}},
		{"Xor", kernels.Xor, [][]any{{true, false}, {true, true}}},
		{"Concat", kernels.Concat, [][]any{{"a", "b"}, {"", "ü"}}},
		{"SumU32", kernels.SumU32, [][]any{{[]uint32{1, 2, 3}}, {[]uint32{math.MaxUint32, 2}}, {nil}, {[]uint32{}}}},
		{"SumF64", kernels.SumF64, [][]any{{[]float64{0.1, 0.2, 0.3}}}},
		{"PrefixSums", kernels.PrefixSums, [][]any{{[]int64{5, -2, 9, 1}}, {[]int64{}}}},
		{"Evens", kernels.Evens, [][]any{{[]int{1, 2, 3, 4, 6, 7, 8, 10, 12}}, {[]int{1, 3}}}},
		{"Reverse", kernels.Reverse, [][]any{{[]int16{1, 2, 3, 4, 5}}}},
		{"CountTrue", kernels.CountTrue, [][]any{{[]bool{true, false, true}}}},
		{"Join", kernels.Join, [][]any{{[]string{"x", "yy", "z"}}, {[]string(nil)}}},
		{"Upper", kernels.Upper, [][]any{{[]byte("hello, World")}}},
		{"NilOrLen", kernels.NilOrLen, [][]any{{nil}, {[]int{}}, {[]int{4, 5}}}},
		{"DivMod", kernels.DivMod, [][]any{{uint64(math.MaxUint64), uint64(10)}}},
		{"F2I", kernels.F2I, [][]any{{-3.99}, {2147483000.5}}},
		{"I2F", kernels.I2F, [][]any{{uint64(math.MaxUint64)}, {uint64(16777217)}}},
		{"Pop", kernels.Pop, [][]any{{uint64(0xFF00FF00FF)}, {uint64(1)}}},
		{"SqrtSum", kernels.SqrtSum, [][]any{{[]float64{4, 9, 2}}}},
		{"SortCopy", kernels.SortCopy, [][]any{{[]int{5, 1, 4, 1, 3}}}},
		{"Strs", kernels.Strs, [][]any{{"héllo"}}},
		{"Bucket", kernels.Bucket, [][]any{{[]uint8{0, 1, 2, 3, 4, 5, 9, 255}}}},
		{"Fields", kernels.Fields, [][]any{{3, 4}}},
		{"Fib", kernels.Fib, [][]any{{10}}},
		{"ApplyTwice", kernels.ApplyTwice, [][]any{{2}}},
		{"MinMax", kernels.MinMax, [][]any{{uint64(1 << 63), uint64(5), uint64(9)}}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fn := kernelFunc(t, c.name)
			interpArgs := make([][]any, len(c.args))
			for i, a := range c.args {
				interpArgs[i] = cloneArgs(a)
			}
			res, err := Run(fn, interpArgs, Options{})
			if err != nil {
				t.Fatal(err)
			}
			if len(res.Violations) > 0 {
				t.Errorf("unexpected violations: %v", res.Violations)
			}
			if len(res.Outputs) != len(c.args) {
				t.Fatalf("got %d outputs, want %d", len(res.Outputs), len(c.args))
			}
			for i, a := range c.args {
				want := nativeCall(c.native, cloneArgs(a))
				if res.Outcomes[i] != Returned {
					t.Errorf("set %d: outcome %v", i, res.Outcomes[i])
					continue
				}
				if !reflect.DeepEqual(res.Outputs[i], want) {
					t.Errorf("set %d %v: got %#v, want %#v", i, a, res.Outputs[i], want)
				}
			}
			if res.Steps == 0 {
				t.Error("no steps counted")
			}
		})
	}
}

func TestPanicOutcome(t *testing.T) {
	fn := kernelFunc(t, "DivMod")
	res, err := Run(fn, [][]any{{uint64(1), uint64(0)}, {uint64(7), uint64(2)}}, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Outcomes[0] != Panicked || res.Outputs[0] != nil {
		t.Errorf("divide by zero: outcome %v outputs %v", res.Outcomes[0], res.Outputs[0])
	}
	if len(res.Violations) == 0 || res.Violations[0].Category != "division-by-zero" || res.Violations[0].Set != 0 {
		t.Errorf("violations = %+v", res.Violations)
	}
	if !reflect.DeepEqual(res.Outputs[1], []any{uint64(3), uint64(1)}) {
		t.Errorf("set 1 after a panic: %v", res.Outputs[1])
	}
}

func TestUnsupportedTypes(t *testing.T) {
	fn := kernelFunc(t, "Fields")
	// Fields(int, int) is supported; its parameter count and kinds are checked.
	var ae *ArgError
	if _, err := Run(fn, [][]any{{1}}, Options{}); !errors.As(err, &ae) || ae.Index != -1 {
		t.Errorf("wrong arity: %v", err)
	}
	if _, err := Run(fn, [][]any{{1, int64(2)}}, Options{}); !errors.As(err, &ae) || ae.Index != 1 {
		t.Errorf("wrong kind: %v", err)
	}
	if _, err := Run(fn, [][]any{{nil, 2}}, Options{}); !errors.As(err, &ae) || ae.Index != 0 {
		t.Errorf("nil scalar: %v", err)
	}

	// Unsupported parameter and result types.
	for _, name := range []string{"TakesMap", "ReturnsArray"} {
		var ute *UnsupportedTypeError
		if _, err := Run(kernelFunc(t, name), nil, Options{}); !errors.As(err, &ute) {
			t.Errorf("%s: err = %v, want UnsupportedTypeError", name, err)
		}
	}
}
