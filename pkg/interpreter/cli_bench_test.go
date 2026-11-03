package interpreter_test

import (
	"path/filepath"
	"testing"

	"github.com/scttfrdmn/giri/internal/ssautil"
	"github.com/scttfrdmn/giri/pkg/interpreter"
)

// BenchmarkCLISynth measures a whole-program CLI run with the default
// detectors on a mixed concurrent workload (testdata/bench/synth). Loading is
// outside the timed loop (#270).
func BenchmarkCLISynth(b *testing.B) {
	abs, err := filepath.Abs("testdata/bench/synth")
	if err != nil {
		b.Fatal(err)
	}
	prog, err := ssautil.LoadProgram(abs)
	if err != nil {
		b.Fatal(err)
	}
	cfg := interpreter.DefaultConfig()
	cfg.MaxSteps = 1 << 40
	var viol int
	for b.Loop() {
		viol = len(interpreter.Run(prog, cfg).Violations)
	}
	b.ReportMetric(float64(viol), "violations")
}
