package interpreter_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/scttfrdmn/giri/internal/ssautil"
	"github.com/scttfrdmn/giri/pkg/interpreter"
)

// TestUnmodeledCallsReport verifies that RunResult.UnmodeledCalls is populated
// when a program calls a function in an intercepted package that has no specific
// handler in Giri's stdlib switch.
//
// math.Erfinv exists in Go's math package but is absent from Giri's handleMathCall
// switch, so the interpreter falls through to execFunction and records it as an
// unmodeled cross-package call.
func TestUnmodeledCallsReport(t *testing.T) {
	wd, _ := os.Getwd()
	prog, err := ssautil.LoadProgram(filepath.Join(wd, "testdata", "integration", "unmodeled_demo"))
	if err != nil {
		t.Skipf("could not load program: %v", err)
	}
	result := interpreter.Run(prog, interpreter.DefaultConfig())
	t.Logf("Violations: %d, UnmodeledCalls: %d", len(result.Violations), len(result.UnmodeledCalls))
	for _, c := range result.UnmodeledCalls {
		t.Logf("  unmodeled: %s", c)
	}

	if len(result.Violations) != 0 {
		t.Errorf("expected 0 violations, got %d: %v", len(result.Violations), result.Violations)
	}
	if len(result.UnmodeledCalls) == 0 {
		t.Error("expected non-empty UnmodeledCalls: math.Erfinv has no Giri model")
	}
	var found bool
	for _, c := range result.UnmodeledCalls {
		if c == "math.Erfinv" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected math.Erfinv in UnmodeledCalls, got: %v", result.UnmodeledCalls)
	}
}
