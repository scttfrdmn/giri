package exec

import (
	"fmt"
	"math"
	"reflect"
	"testing"

	"github.com/scttfrdmn/giri/pkg/exec/internal/aggk"
)

func nativeSafe(f any, args []any) (out []any, panicked bool) {
	defer func() {
		if r := recover(); r != nil {
			panicked = true
		}
	}()
	return nativeCall(f, args), false
}

// TestAggregateKernels runs the tumble session's array/struct review kernels
// (internal/aggk) natively and interpreted, with and without detectors. A
// Returned result must equal native, a native panic must be Panicked, and
// Opaque (not computable concretely) is allowed but never a wrong value.
func TestAggregateKernels(t *testing.T) {
	pkg, err := LoadDir("internal/aggk")
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name   string
		native any
		args   [][]any
	}{
		{"CopyArr", aggk.CopyArr, [][]any{{7}}},
		{"CopyStruct", aggk.CopyStruct, [][]any{{7}}},
		{"PassByValue", aggk.PassByValue, [][]any{{7}}},
		{"Returned", aggk.Returned, [][]any{{7}}},
		{"ArrOfStruct", aggk.ArrOfStruct, [][]any{{7}}},
		{"StructOfArr", aggk.StructOfArr, [][]any{{7}}},
		{"Equality", aggk.Equality, [][]any{{4}, {5}, {0}}},
		{"FloatEq", aggk.FloatEq, [][]any{{1.5}, {0.0}, {-1.0}}},
		{"Literals", aggk.Literals, [][]any{{7}}},
		{"RangeCopy", aggk.RangeCopy, [][]any{{7}}},
		{"Accum", aggk.Accum, [][]any{{0}, {5}, {13}}},
		{"VarIndex", aggk.VarIndex, [][]any{{0}, {3}, {4}, {-1}}},
		{"VarIndexAddr", aggk.VarIndexAddr, [][]any{{2}, {4}, {-1}}},
		{"ValIndexStruct", aggk.ValIndexStruct, [][]any{{2}, {3}}},
		{"PtrArr", aggk.PtrArr, [][]any{{7}}},
		{"PtrStruct", aggk.PtrStruct, [][]any{{7}}},
		{"Mixed", aggk.Mixed, [][]any{{int8(100), uint16(30000), float32(1.1)}, {int8(-5), uint16(65535), float32(3.3e38)}, {int8(27), uint16(1), float32(0.1)}}},
		{"MixedArr", aggk.MixedArr, [][]any{{int8(100)}, {int8(-128)}}},
		{"SliceOfStruct", aggk.SliceOfStruct, [][]any{{7}}},
		{"AppendStruct", aggk.AppendStruct, [][]any{{7}}},
		{"CopyStructSlice", aggk.CopyStructSlice, [][]any{{7}}},
		{"AppendArr", aggk.AppendArr, [][]any{{7}}},
		{"SliceLitStruct", aggk.SliceLitStruct, [][]any{{7}}},
		{"RangeSliceStruct", aggk.RangeSliceStruct, [][]any{{7}}},
		{"MapStructKey", aggk.MapStructKey, [][]any{{2}, {3}}},
		{"MapArrKey", aggk.MapArrKey, [][]any{{2}, {3}}},
		{"MapStructVal", aggk.MapStructVal, [][]any{{7}}},
		{"IfaceEq", aggk.IfaceEq, [][]any{{2}, {3}}},
		{"IfaceAssert", aggk.IfaceAssert, [][]any{{7}}},
		{"ArrSlice", aggk.ArrSlice, [][]any{{7}}},
		{"Closure", aggk.Closure, [][]any{{7}}},
		{"Phi", aggk.Phi, [][]any{{1}, {-1}}},
		{"StructStrings", aggk.StructStrings, [][]any{{"q"}}},
		{"Swap", aggk.Swap, [][]any{{7}}},
		{"Nested2D", aggk.Nested2D, [][]any{{7}}},
		{"ZeroSizeAndBlank", aggk.ZeroSizeAndBlank, [][]any{{7}}},
		{"PtrInStruct", aggk.PtrInStruct, [][]any{{7}}},
		{"DeferArr", aggk.DeferArr, [][]any{{7}}},
		{"MapScalar", aggk.MapScalar, [][]any{{1}, {2}}}, {"DeferScalar", aggk.DeferScalar, [][]any{{7}}}, {"DeferArrNoParam", aggk.DeferArrNoParam, [][]any{{7}}}, {"AppendScalar", aggk.AppendScalar, [][]any{{7}}},
		{"IfaceScalarEq", aggk.IfaceScalarEq, [][]any{{2}, {3}}}, {"IfaceArrOnly", aggk.IfaceArrOnly, [][]any{{2}, {3}}}, {"IfaceStructAssert", aggk.IfaceStructAssert, [][]any{{7}}}, {"GlobalsScalar", aggk.GlobalsScalar, [][]any{{7}}}, {"Globals", aggk.Globals, [][]any{{7}}}, {"Convert", aggk.Convert, [][]any{{7}}}, {"NaNStruct", aggk.NaNStruct, [][]any{{1.0}, {math.NaN()}}}, {"PtrEqStruct", aggk.PtrEqStruct, [][]any{{7}}}, {"IfaceFieldEq", aggk.IfaceFieldEq, [][]any{{7}}}, {"ClearArr", aggk.ClearArr, [][]any{{7}}}, {"AppendIntoArr", aggk.AppendIntoArr, [][]any{{7}}}, {"ConstIndexPtr", aggk.ConstIndexPtr, [][]any{{7}}}, {"ArrayOfStrings", aggk.ArrayOfStrings, [][]any{{"a"}}}, {"StructCompareInLoop", aggk.StructCompareInLoop, [][]any{{10}, {2}}}, {"ArgSliceToArr", aggk.ArgSliceToArr, [][]any{{[]int{1, 2, 3, 4}}, {[]int{5}}}}, {"SliceToArrConv", aggk.SliceToArrConv, [][]any{{[]int{1, 2}}, {[]int{1}}}}, {"ChanStruct", aggk.ChanStruct, [][]any{{7}}}, {"ChanScalar", aggk.ChanScalar, [][]any{{7}}}, {"ExtStruct", aggk.ExtStruct, [][]any{{7}}}, {"ExtStructArr", aggk.ExtStructArr, [][]any{{7}}}, {"ChanArrIdx", aggk.ChanArrIdx, [][]any{{7}}}, {"RecoverIdx", aggk.RecoverIdx, [][]any{{1}, {5}}}, {"ClearStructSlice", aggk.ClearStructSlice, [][]any{{7}}}, {"RecoverSliceIdx", aggk.RecoverSliceIdx, [][]any{{1}, {5}}},
	}
	// Kernels that recover a run-time panic giri raises: the result must match
	// native, and the panic is still reported as a violation.
	recoversPanic := map[string]bool{"RecoverIdx": true, "RecoverSliceIdx": true}
	for _, c := range cases {
		// Native results are computed once per case, in table order: some
		// kernels write package variables, which persist natively but start
		// fresh in every exec.Run.
		type nat struct {
			out      []any
			panicked bool
		}
		natives := make([]nat, len(c.args))
		for i, a := range c.args {
			natives[i].out, natives[i].panicked = nativeSafe(c.native, cloneArgs(a))
		}
		for _, opts := range []Options{{}, {SkipDetectors: true}} {
			name := fmt.Sprintf("%s/skip=%v", c.name, opts.SkipDetectors)
			t.Run(name, func(t *testing.T) {
				fn := pkg.Func(c.name)
				if fn == nil {
					t.Fatalf("no func")
				}
				for i, a := range c.args {
					res, err := Run(fn, [][]any{cloneArgs(a)}, opts)
					if err != nil {
						t.Fatalf("set %d: %v", i, err)
					}
					want, np := natives[i].out, natives[i].panicked
					if np {
						if res.Outcomes[0] != Panicked {
							t.Errorf("MISMATCH set %d args %v: native panics; interp outcome %v out %v viol %v", i, a, res.Outcomes[0], res.Outputs[0], res.Violations)
						}
						continue
					}
					if res.Outcomes[0] == Opaque {
						continue // not computable concretely; never a wrong answer
					}
					if res.Outcomes[0] != Returned {
						t.Errorf("MISMATCH set %d args %v: outcome %v viol %v; native %v", i, a, res.Outcomes[0], res.Violations, want)
						continue
					}
					// A recovered run-time panic is still reported (#259).
					if len(res.Violations) > 0 && !recoversPanic[c.name] {
						t.Errorf("set %d: violations %v", i, res.Violations)
					}
					if !reflect.DeepEqual(res.Outputs[0], want) {
						t.Errorf("MISMATCH set %d args %v:\n native %v\n interp %v", i, a, want, res.Outputs[0])
					}
				}
			})
		}
	}
}
