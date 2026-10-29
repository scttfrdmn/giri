package exec

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestLoadDirManyFunctions loads a generated package of 300 functions once
// and calls every one. The module path is dotless ("gen"), like tumble's
// per-generation packages, and every tenth function calls its predecessor,
// so same-package calls must be interpreted (#249).
func TestLoadDirManyFunctions(t *testing.T) {
	const n = 300
	dir := t.TempDir()
	var src strings.Builder
	src.WriteString("package gen\n")
	want := make([]int, n)
	for i := 0; i < n; i++ {
		if i%10 == 9 {
			fmt.Fprintf(&src, "\nfunc F%03d(x int) int { return F%03d(x) * 2 }\n", i, i-1)
			want[i] = want[i-1] * 2
		} else {
			fmt.Fprintf(&src, "\nfunc F%03d(x int) int { return x + %d }\n", i, i)
			want[i] = 5 + i
		}
	}
	for name, content := range map[string]string{
		"go.mod": "module gen\n\ngo 1.26\n",
		"gen.go": src.String(),
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	start := time.Now()
	pkg, err := LoadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("LoadDir: %d functions in %v", n, time.Since(start))

	for i := 0; i < n; i++ {
		fn := pkg.Func(fmt.Sprintf("F%03d", i))
		if fn == nil || fn.Blocks == nil {
			t.Fatalf("F%03d missing or not built", i)
		}
		res, err := Run(fn, [][]any{{5}}, Options{})
		if err != nil {
			t.Fatal(err)
		}
		if res.Outcomes[0] != Returned || res.Outputs[0][0] != want[i] {
			t.Errorf("F%03d(5) = %v (%v), want %d", i, res.Outputs[0], res.Outcomes[0], want[i])
		}
	}
}

func TestLoadDirErrors(t *testing.T) {
	if _, err := LoadDir(filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Error("missing directory: want error")
	}
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module bad\n\ngo 1.26\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "bad.go"), []byte("package bad\n\nfunc F() int { return \"x\" }\n"), 0o644)
	if _, err := LoadDir(dir); err == nil {
		t.Error("type error: want error")
	}
}
