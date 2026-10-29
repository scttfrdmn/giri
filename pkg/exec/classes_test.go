package exec

import (
	"testing"

	"golang.org/x/tools/go/ssa"

	"github.com/scttfrdmn/giri/pkg/interpreter"
)

// TestStepsByClass checks per-class counts against instruction mixes counted
// by hand from the SSA of ClassBranchy and ClassMem (kernels.go).
func TestStepsByClass(t *testing.T) {
	cases := []struct {
		fn   string
		args [][]any
		want map[Class]uint64
	}{
		{
			// (6,7): t0=a+b, t1=t0>10, if, t2=t0*2, jump, phi, return.
			// (1,2): t0, t1, if, phi, return.
			fn:   "ClassBranchy",
			args: [][]any{{6, 7}, {1, 2}},
			want: map[Class]uint64{ClassArithmetic: 3, ClassCompare: 2, ClassBranch: 3, ClassPhi: 2, ClassOther: 2},
		},
		{
			// len(xs)=3: entry (len, make, jump); loop header 4× (phi, len, <, if);
			// body 3× (&xs[i], load, <<, &ys[i], store, +, jump); exit (&ys[0],
			// load, call helper, len, convert, call OnesCount64, +, return);
			// helper (-, return).
			fn:   "ClassMem",
			args: [][]any{{[]int{1, 2, 3}}},
			want: map[Class]uint64{
				ClassBuiltin: 6, ClassAlloc: 1, ClassBranch: 8, ClassPhi: 4,
				ClassCompare: 4, ClassIndex: 7, ClassLoad: 4, ClassStore: 3,
				ClassArithmetic: 8, ClassCallInterpreted: 1, ClassCallIntercepted: 1,
				ClassOther: 3,
			},
		},
	}
	for _, c := range cases {
		t.Run(c.fn, func(t *testing.T) {
			res, err := Run(kernelFunc(t, c.fn), c.args, Options{})
			if err != nil {
				t.Fatal(err)
			}
			var sum, wantSum uint64
			for cl := Class(0); cl < NumClasses; cl++ {
				got := res.StepsByClass[cl]
				if got != c.want[cl] {
					t.Errorf("%s: got %d, want %d", cl, got, c.want[cl])
				}
				sum += got
				wantSum += c.want[cl]
			}
			if sum != res.Steps || res.Steps != wantSum {
				t.Errorf("Steps = %d, class sum = %d, want %d", res.Steps, sum, wantSum)
			}
		})
	}
}

// TestClassOf checks the static classification exported for documentation
// and calibration tooling.
func TestClassOf(t *testing.T) {
	fn := kernelFunc(t, "ClassMem")
	got := map[string]Class{}
	for _, b := range fn.Blocks {
		for _, instr := range b.Instrs {
			switch i := instr.(type) {
			case *ssa.Call:
				got[i.Call.Value.Name()] = interpreter.ClassOf(i)
			case *ssa.Store:
				got["store"] = interpreter.ClassOf(i)
			}
		}
	}
	want := map[string]Class{
		"len": ClassBuiltin, "helper": ClassCallInterpreted,
		"OnesCount64": ClassCallIntercepted, "store": ClassStore,
	}
	for k, w := range want {
		if got[k] != w {
			t.Errorf("ClassOf(%s) = %v, want %v", k, got[k], w)
		}
	}
	if s := ClassCallInterpreted.String(); s != "call-interpreted" {
		t.Errorf("String() = %q", s)
	}
}
