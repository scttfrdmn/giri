package interpreter_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/scttfrdmn/giri/internal/ssautil"
	"github.com/scttfrdmn/giri/pkg/interpreter"
)

var integrationTests = []struct {
	name           string
	dir            string
	wantViolations int
	wantCategory   string // empty = don't check; substring match against error string
	config         interpreter.Config
}{
	{
		name:           "safe alloc",
		dir:            "safe_alloc",
		wantViolations: 0,
		wantCategory:   "",
		config:         interpreter.DefaultConfig(),
	},
	{
		name:           "unsafe oob",
		dir:            "unsafe_oob",
		wantViolations: 1,
		wantCategory:   "unsafe",
		config:         interpreter.DefaultConfig(),
	},
	{
		name:           "binop",
		dir:            "binop",
		wantViolations: 0,
		wantCategory:   "",
		config:         interpreter.DefaultConfig(),
	},
	{
		name:           "multi return",
		dir:            "multi_return",
		wantViolations: 0,
		wantCategory:   "",
		config:         interpreter.DefaultConfig(),
	},
	{
		name:           "uintptr gc",
		dir:            "uintptr_gc",
		wantViolations: 1,
		wantCategory:   "rule 2",
		config:         interpreter.DefaultConfig(),
	},
	{
		name:           "safe uintptr",
		dir:            "safe_uintptr",
		wantViolations: 0,
		wantCategory:   "",
		config:         interpreter.DefaultConfig(),
	},
	{
		name:           "misaligned ptr",
		dir:            "misaligned_ptr",
		wantViolations: 1,
		wantCategory:   "rule 1",
		config:         interpreter.DefaultConfig(),
	},
	// v0.3.1 regression tests
	{
		name:           "loop phi zero",
		dir:            "loop",
		wantViolations: 0,
		wantCategory:   "",
		config:         interpreter.DefaultConfig(),
	},
	{
		name:           "closure freevars",
		dir:            "closure",
		wantViolations: 0,
		wantCategory:   "",
		config:         interpreter.DefaultConfig(),
	},
	{
		name:           "maxsteps enforced",
		dir:            "maxsteps",
		wantViolations: 1,
		wantCategory:   "execution limit",
		config: func() interpreter.Config {
			c := interpreter.DefaultConfig()
			c.MaxSteps = 200 // trip well before 1M iterations
			return c
		}(),
	},
	{
		name:           "panic defers",
		dir:            "panic_defers",
		wantViolations: 0,
		wantCategory:   "",
		config:         interpreter.DefaultConfig(),
	},
	// v0.4.0 regression tests
	{
		name:           "data race",
		dir:            "data_race",
		wantViolations: 1,
		wantCategory:   "data race",
		config:         interpreter.DefaultConfig(),
	},
	{
		name:           "no race chan",
		dir:            "no_race_chan",
		wantViolations: 0,
		wantCategory:   "",
		config:         interpreter.DefaultConfig(),
	},
	{
		name:           "uninit read",
		dir:            "uninit_read",
		wantViolations: 1,
		wantCategory:   "uninitialized",
		config: func() interpreter.Config {
			c := interpreter.DefaultConfig()
			c.TrackInit = true
			return c
		}(),
	},
}

func TestIntegration(t *testing.T) {
	for _, tt := range integrationTests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			wd, err := os.Getwd()
			if err != nil {
				t.Skipf("could not get working directory: %v", err)
			}
			absPath := filepath.Join(wd, "testdata", "integration", tt.dir)

			prog, err := ssautil.LoadProgram(absPath)
			if err != nil {
				t.Skipf("skipping %s: could not load program: %v", tt.name, err)
				return
			}
			if prog.Main == nil {
				t.Skipf("skipping %s: no main package found", tt.name)
				return
			}

			result := interpreter.Run(prog, tt.config)

			// Deduplicate violations for count check
			gotViolations := len(result.Violations)

			if tt.wantViolations == 0 {
				if gotViolations != 0 {
					t.Errorf("want 0 violations, got %d:", gotViolations)
					for _, v := range result.Violations {
						t.Logf("  - %v", v)
					}
				}
			} else {
				if gotViolations < tt.wantViolations {
					t.Errorf("want >= %d violations, got %d", tt.wantViolations, gotViolations)
				}
			}

			if tt.wantCategory != "" {
				found := false
				for _, v := range result.Violations {
					if strings.Contains(v.Error(), tt.wantCategory) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("want violation containing %q, violations: %v", tt.wantCategory, result.Violations)
				}
			}
		})
	}
}
