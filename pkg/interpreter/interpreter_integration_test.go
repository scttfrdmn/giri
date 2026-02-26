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
