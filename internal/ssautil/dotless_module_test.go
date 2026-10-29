package ssautil_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/scttfrdmn/giri/internal/ssautil"
	"github.com/scttfrdmn/giri/pkg/interpreter"
)

// TestDotlessModuleCallInterpreted is the #249 reproduction: in a module whose
// path has no dot ("module probe"), a same-module call must be interpreted,
// not stubbed as an unmodeled stdlib call returning zero.
func TestDotlessModuleCallInterpreted(t *testing.T) {
	dir := t.TempDir()
	write := func(name, src string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("go.mod", "module probe\n\ngo 1.26\n")
	write("main.go", `package main

func g() int { return 7 }

func main() {
	if g() != 7 {
		var t []int
		_ = t[0]
	}
}
`)
	t.Chdir(dir)
	prog, err := ssautil.LoadProgram(".")
	if err != nil {
		t.Fatal(err)
	}
	res := interpreter.Run(prog, interpreter.DefaultConfig())
	if len(res.Violations) != 0 {
		t.Errorf("violations = %v (g() was stubbed instead of interpreted)", res.Violations)
	}
	for _, u := range res.UnmodeledCalls {
		if u == "probe.g" {
			t.Errorf("probe.g reported as unmodeled")
		}
	}
}
