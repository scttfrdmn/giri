package exec

import (
	"reflect"
	"testing"

	"github.com/scttfrdmn/giri/pkg/exec/internal/kernels"
)

// nativeArgsAfter runs f natively on a copy of args and returns the copy's
// slice contents afterwards, in ArgsAfter's shape (nil for non-slices; a nil
// slice stays a typed nil).
func nativeArgsAfter(f any, args []any) []any {
	c := cloneArgs(args)
	// An untyped nil argument becomes the parameter's typed nil natively.
	ft := reflect.TypeOf(f)
	for i, a := range c {
		if a == nil && ft.In(i).Kind() == reflect.Slice {
			c[i] = reflect.Zero(ft.In(i)).Interface()
		}
	}
	nativeCall(f, c)
	out := make([]any, len(c))
	for i, a := range c {
		if rv := reflect.ValueOf(a); a != nil && rv.Kind() == reflect.Slice {
			out[i] = a
		}
	}
	return out
}

// TestArgsAfterMatchesNative: slice arguments read back after the call equal
// the native function's view of them, whether or not the function wrote into
// them, and the caller's slices are left untouched (#266).
func TestArgsAfterMatchesNative(t *testing.T) {
	cases := []struct {
		name   string
		native any
		args   [][]any
	}{
		{"PopClobber", kernels.PopClobber, [][]any{{[]uint64{1, 3, 7}}, {[]uint64{0xff}}, {[]uint64{}}, {nil}}},
		{"PrefixSum32", kernels.PrefixSum32, [][]any{
			{[]int32{1, 2, 3, 4}, make([]int32, 4)},
			{[]int32{5, -5}, make([]int32, 1)},
			{[]int32{1}, []int32(nil)},
		}},
		{"MixedArgs", kernels.MixedArgs, [][]any{{3, []string{"a"}, []bool{true, false}}}},
		{"SumU32", kernels.SumU32, [][]any{{[]uint32{1, 2, 3}}}},        // reads only
		{"Reverse", kernels.Reverse, [][]any{{[]int16{1, 2, 3, 4, 5}}}}, // writes in place
		{"AddInt", kernels.AddInt, [][]any{{1, 2}}},                     // no slices
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Keep the originals to check Run doesn't modify them.
			inputs := make([][]any, len(c.args))
			pristine := make([][]any, len(c.args))
			for i, a := range c.args {
				inputs[i] = cloneArgs(a)
				pristine[i] = cloneArgs(a)
			}
			res, err := Run(kernelFunc(t, c.name), inputs, Options{})
			if err != nil {
				t.Fatal(err)
			}
			if len(res.ArgsAfter) != len(c.args) {
				t.Fatalf("ArgsAfter has %d entries, want %d", len(res.ArgsAfter), len(c.args))
			}
			for i, a := range c.args {
				want := nativeArgsAfter(c.native, a)
				if !reflect.DeepEqual(res.ArgsAfter[i], want) {
					t.Errorf("set %d: ArgsAfter %#v, native %#v", i, res.ArgsAfter[i], want)
				}
				if !reflect.DeepEqual(inputs[i], pristine[i]) {
					t.Errorf("set %d: Run modified the caller's arguments: %#v", i, inputs[i])
				}
			}
		})
	}
}

// TestArgsAfterOnlyForReturnedSets: sets that panic or run out of budget
// report nil; a set after a panic still reports its own contents.
func TestArgsAfterOnlyForReturnedSets(t *testing.T) {
	res, err := Run(kernelFunc(t, "OOB"), [][]any{{[]int{1, 2}, 5}, {[]int{1, 2}, 1}}, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if res.ArgsAfter[0] != nil || !reflect.DeepEqual(res.ArgsAfter[1], []any{[]int{1, 2}, nil}) {
		t.Errorf("ArgsAfter = %#v", res.ArgsAfter)
	}
	fn := kernelFunc(t, "PrefixSum32")
	full, _ := Run(fn, [][]any{{[]int32{1, 2, 3}, make([]int32, 3)}}, Options{})
	res, _ = Run(fn, [][]any{{[]int32{1, 2, 3}, make([]int32, 3)}, {[]int32{1}, make([]int32, 1)}},
		Options{MaxSteps: full.Steps + 1})
	if !res.Exhausted || len(res.ArgsAfter) != 2 || res.ArgsAfter[1] != nil || res.ArgsAfter[0] == nil {
		t.Errorf("exhausted run: outcomes %v ArgsAfter %#v", res.Outcomes, res.ArgsAfter)
	}
}

// TestArgsAfterOpaque: a slice argument left holding a value with no concrete
// representation makes the set Opaque, like an unrepresentable result.
func TestArgsAfterOpaque(t *testing.T) {
	res, err := Run(kernelFunc(t, "OpaqueIntoArg"), [][]any{{[]int{1, 2}}}, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Outcomes[0] != Opaque || res.Outputs[0] != nil || res.ArgsAfter[0] != nil {
		t.Errorf("outcome %v outputs %v ArgsAfter %v", res.Outcomes[0], res.Outputs[0], res.ArgsAfter[0])
	}
}

// TestArgsAfterNilSliceIsTyped: a nil slice argument comes back as the
// parameter's typed nil, which is what native code sees.
func TestArgsAfterNilSliceIsTyped(t *testing.T) {
	res, err := Run(kernelFunc(t, "PopClobber"), [][]any{{nil}, {[]uint64(nil)}}, Options{})
	if err != nil {
		t.Fatal(err)
	}
	for i := range res.ArgsAfter {
		got, ok := res.ArgsAfter[i][0].([]uint64)
		if !ok || got != nil {
			t.Errorf("set %d: ArgsAfter %#v, want []uint64(nil)", i, res.ArgsAfter[i][0])
		}
	}
}
