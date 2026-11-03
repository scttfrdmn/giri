package exec

import (
	"fmt"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"

	"golang.org/x/tools/go/ssa"
)

// loaderCases are kernels and argument sets compared between LoadDir and
// Loader.Load.
var loaderCases = []struct {
	fn   string
	args [][]any
}{
	{"MixU64", [][]any{{uint64(1 << 63), uint64(3)}, {uint64(5), uint64(9)}}},
	{"PrefixSum32", [][]any{{[]int32{1, 2, 3}, make([]int32, 3)}}},
	{"Strs", [][]any{{"héllo"}}},
	{"SortCopy", [][]any{{[]int{3, 1, 2}}}},
	{"StructValues", [][]any{{1.5, int32(300)}}},
	{"Swap", [][]any{{3}}},
	{"Pop", [][]any{{uint64(0xF0F0)}}},
	{"DivMod", [][]any{{uint64(7), uint64(0)}}},
	{"Accum", [][]any{{20}}},
}

// sameResult compares everything observable in two Results except violation
// messages, which name the source file.
func sameResult(a, b Result) bool {
	cats := func(r Result) []string {
		var c []string
		for _, v := range r.Violations {
			c = append(c, fmt.Sprintf("%d %s", v.Set, v.Category))
		}
		return c
	}
	return reflect.DeepEqual(a.Outputs, b.Outputs) && reflect.DeepEqual(a.ArgsAfter, b.ArgsAfter) &&
		reflect.DeepEqual(a.Outcomes, b.Outcomes) && a.Steps == b.Steps &&
		reflect.DeepEqual(a.StepsByClass, b.StepsByClass) && reflect.DeepEqual(cats(a), cats(b))
}

func loadKernelsSource(t *testing.T, l *Loader) *ssa.Package {
	t.Helper()
	src, err := os.ReadFile("internal/kernels/kernels.go")
	if err != nil {
		t.Fatal(err)
	}
	pkg, err := l.Load("kernels", src)
	if err != nil {
		t.Fatal(err)
	}
	return pkg
}

// TestLoaderMatchesLoadDir: functions loaded from source bytes run exactly as
// the same functions loaded by LoadDir, including stdlib calls and traces.
func TestLoaderMatchesLoadDir(t *testing.T) {
	pkg := loadKernelsSource(t, NewLoader())
	for _, c := range loaderCases {
		want, err := Run(kernelFunc(t, c.fn), c.args, Options{Trace: true})
		if err != nil {
			t.Fatal(err)
		}
		got, err := Run(pkg.Func(c.fn), c.args, Options{Trace: true})
		if err != nil {
			t.Fatal(err)
		}
		if !sameResult(got, want) {
			t.Errorf("%s: Loader result %+v\nLoadDir result %+v", c.fn, got, want)
		}
		if len(got.Traces) != len(want.Traces) || len(got.Traces[0]) != len(want.Traces[0]) {
			t.Errorf("%s: trace shape differs", c.fn)
		}
	}
}

// TestLoaderPositionsAndDeterminism: positions are relative to each load's
// source whatever was loaded before, and the same source yields identical
// Results (traces included) on every load.
func TestLoaderPositionsAndDeterminism(t *testing.T) {
	l := NewLoader()
	if _, err := l.Load("big", []byte(genSource(200, true))); err != nil {
		t.Fatal(err)
	}
	src := []byte("package small\n\n// F is on line 4.\nfunc F(n int) int {\n\ts := 0\n\tfor i := 0; i < n; i++ {\n\t\ts += i\n\t}\n\treturn s\n}\n")
	var first Result
	for i := 0; i < 3; i++ {
		pkg, err := l.Load("small", src)
		if err != nil {
			t.Fatal(err)
		}
		fn := pkg.Func("F")
		if line := fn.Prog.Fset.Position(fn.Pos()).Line; line != 4 {
			t.Errorf("load %d: F at line %d, want 4", i, line)
		}
		for v := range mustRun(t, fn, Options{Trace: true}).Traces[0] {
			if line := fn.Prog.Fset.Position(v.Pos()).Line; v.Pos().IsValid() && (line < 4 || line > 10) {
				t.Errorf("load %d: traced value %v at line %d, outside F", i, v, line)
			}
		}
		r := mustRun(t, fn, Options{Trace: true})
		if i == 0 {
			first = r
		} else if string(canonical(t, r)) != string(canonical(t, first)) {
			t.Errorf("load %d: Result differs from the first load", i)
		}
	}
	if _, err := l.Load("gen", []byte(genSource(5, false))); err != nil {
		t.Errorf("dotless package path: %v", err)
	}
}

func mustRun(t *testing.T, fn *ssa.Function, opts Options) Result {
	t.Helper()
	r, err := Run(fn, [][]any{{10}}, opts)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestLoaderErrors(t *testing.T) {
	l := NewLoader()
	for name, src := range map[string]string{
		"syntax": "package p\nfunc F( {",
		"type":   "package p\nfunc F() int { return \"x\" }",
		"import": "package p\nimport \"no/such/pkg\"\nvar _ = pkg.X",
		"nopath": "",
	} {
		path := "p"
		if name == "nopath" {
			path = ""
		}
		if _, err := l.Load(path, []byte(src)); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
	// A failed load doesn't break the Loader.
	if _, err := l.Load("p", []byte("package p\nfunc F() int { return 1 }")); err != nil {
		t.Errorf("load after errors: %v", err)
	}
}

// TestLoaderConcurrent: concurrent Loads, and Runs on functions from earlier
// loads while loading continues (run under -race in CI).
func TestLoaderConcurrent(t *testing.T) {
	l := NewLoader()
	base := loadKernelsSource(t, l)
	var wg sync.WaitGroup
	errs := make(chan error, 16)
	for g := 0; g < 4; g++ {
		wg.Add(2)
		go func(g int) {
			defer wg.Done()
			src := strings.ReplaceAll(genSource(20, g%2 == 0), "package gen", fmt.Sprintf("package gen%d", g))
			for i := 0; i < 3; i++ {
				pkg, err := l.Load(fmt.Sprintf("gen%d", g), []byte(src))
				if err != nil {
					errs <- err
					return
				}
				if r, err := Run(pkg.Func("F007"), [][]any{{1}}, Options{}); err != nil || r.Outputs[0][0] != 8 {
					errs <- fmt.Errorf("gen%d F007: %v %v", g, err, r.Outputs)
				}
			}
		}(g)
		go func() {
			defer wg.Done()
			for i := 0; i < 3; i++ {
				if r, err := Run(base.Func("Accum"), [][]any{{5}}, Options{}); err != nil || r.Outputs[0][0] != 55 {
					errs <- fmt.Errorf("Accum: %v %v", err, r.Outputs)
				}
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
}
