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
	// v0.5.0 regression tests
	{
		name:           "spawn hb",
		dir:            "spawn_hb",
		wantViolations: 0,
		wantCategory:   "",
		config:         interpreter.DefaultConfig(),
	},
	{
		name:           "nil deref",
		dir:            "nil_deref",
		wantViolations: 1,
		wantCategory:   "nil pointer",
		config:         interpreter.DefaultConfig(),
	},
	{
		name:           "close panic",
		dir:            "close_panic",
		wantViolations: 1,
		wantCategory:   "closed channel",
		config:         interpreter.DefaultConfig(),
	},
	{
		name:           "slice oob",
		dir:            "slice_oob",
		wantViolations: 1,
		wantCategory:   "out-of-bounds",
		config:         interpreter.DefaultConfig(),
	},
	// v0.7.0 regression tests
	{
		name:           "type assert ok",
		dir:            "type_assert_ok",
		wantViolations: 0,
		wantCategory:   "",
		config:         interpreter.DefaultConfig(),
	},
	{
		name:           "type assert fail",
		dir:            "type_assert_fail",
		wantViolations: 1,
		wantCategory:   "type-assertion",
		config:         interpreter.DefaultConfig(),
	},
	{
		name:           "iface dispatch",
		dir:            "iface_dispatch",
		wantViolations: 0,
		wantCategory:   "",
		config:         interpreter.DefaultConfig(),
	},
	// v0.8.0 regression tests
	{
		name:           "reflect uintptr gc",
		dir:            "reflect_uintptr",
		wantViolations: 1,
		wantCategory:   "rule 5",
		config:         interpreter.DefaultConfig(),
	},
	{
		name:           "slice header",
		dir:            "slice_header",
		wantViolations: 1,
		wantCategory:   "rule 6",
		config:         interpreter.DefaultConfig(),
	},
	// v0.9.0 regression tests
	{
		name:           "callstack depth",
		dir:            "callstack_depth",
		wantViolations: 1,
		wantCategory:   "rule 1",
		config:         interpreter.DefaultConfig(),
	},
	{
		name:           "goroutine leak",
		dir:            "goroutine_leak",
		wantViolations: 1,
		wantCategory:   "goroutine leak",
		config:         interpreter.DefaultConfig(),
	},
	{
		name:           "no goroutine leak",
		dir:            "no_goroutine_leak",
		wantViolations: 0,
		wantCategory:   "",
		config:         interpreter.DefaultConfig(),
	},
	// v0.10.0 regression tests
	{
		name:           "type switch dispatch",
		dir:            "type_switch_dispatch",
		wantViolations: 0,
		wantCategory:   "",
		config:         interpreter.DefaultConfig(),
	},
	{
		name:           "type switch nil",
		dir:            "type_switch_nil",
		wantViolations: 0,
		wantCategory:   "",
		config:         interpreter.DefaultConfig(),
	},
	// v0.11.0 regression tests
	{
		name:           "strings intercept",
		dir:            "strings_intercept",
		wantViolations: 1,
		wantCategory:   "rule 1",
		config:         interpreter.DefaultConfig(),
	},
	{
		name:           "strconv atoi",
		dir:            "strconv_atoi",
		wantViolations: 1,
		wantCategory:   "rule 1",
		config:         interpreter.DefaultConfig(),
	},
	{
		name:           "fmt sprintf",
		dir:            "fmt_sprintf",
		wantViolations: 1,
		wantCategory:   "rule 1",
		config:         interpreter.DefaultConfig(),
	},
	// v0.12.0 regression tests
	{
		name:           "buffered chan overflow",
		dir:            "buffered_chan_overflow",
		wantViolations: 1,
		wantCategory:   "goroutine leak",
		config:         interpreter.DefaultConfig(),
	},
	{
		name:           "select default",
		dir:            "select_default",
		wantViolations: 0,
		wantCategory:   "",
		config:         interpreter.DefaultConfig(),
	},
	{
		name:           "select timeout",
		dir:            "select_timeout",
		wantViolations: 0,
		wantCategory:   "",
		config:         interpreter.DefaultConfig(),
	},
	{
		name:           "map race",
		dir:            "map_race",
		wantViolations: 1,
		wantCategory:   "data race",
		config:         interpreter.DefaultConfig(),
	},
	{
		name:           "sync map no race",
		dir:            "sync_map_no_race",
		wantViolations: 0,
		wantCategory:   "",
		config:         interpreter.DefaultConfig(),
	},
	// v0.14.0 regression tests
	{
		name:           "double close",
		dir:            "double_close",
		wantViolations: 1,
		wantCategory:   "closed channel",
		config:         interpreter.DefaultConfig(),
	},
	{
		name:           "nil map write",
		dir:            "nil_map_write",
		wantViolations: 1,
		wantCategory:   "nil map",
		config:         interpreter.DefaultConfig(),
	},
	{
		name:           "div zero",
		dir:            "div_zero",
		wantViolations: 1,
		wantCategory:   "division by zero",
		config:         interpreter.DefaultConfig(),
	},
	// v0.13.0 regression tests
	{
		name:           "defer unlock",
		dir:            "defer_unlock",
		wantViolations: 0,
		wantCategory:   "",
		config:         interpreter.DefaultConfig(),
	},
	{
		name:           "defer user func",
		dir:            "defer_user_func",
		wantViolations: 0,
		wantCategory:   "",
		config:         interpreter.DefaultConfig(),
	},
	{
		name:           "multi recover",
		dir:            "multi_recover",
		wantViolations: 0,
		wantCategory:   "",
		config:         interpreter.DefaultConfig(),
	},
	{
		name:           "named return defer",
		dir:            "named_return_defer",
		wantViolations: 0,
		wantCategory:   "",
		config:         interpreter.DefaultConfig(),
	},
}

var showcaseTests = []struct {
	name           string
	dir            string
	wantViolations int
	wantCategory   string
	config         interpreter.Config
}{
	{
		// unsafe.Add moves pointer past end of [4]byte allocation.
		// go vet: pass, go test -race: pass.
		name:           "unsafe oob",
		dir:            "unsafe_oob",
		wantViolations: 1,
		wantCategory:   "out-of-bounds",
		config:         interpreter.DefaultConfig(),
	},
	{
		// Converts *byte at offset 1 to *uint32 (requires 4-byte alignment).
		// go vet: pass, go test -race: pass.
		name:           "unsafe alignment",
		dir:            "unsafe_alignment",
		wantViolations: 1,
		wantCategory:   "rule 1",
		config:         interpreter.DefaultConfig(),
	},
	{
		// uintptr held across doWork() GC safepoint.
		// go vet: pass, go test -race: pass.
		name:           "uintptr gc hazard",
		dir:            "uintptr_gc_hazard",
		wantViolations: 1,
		wantCategory:   "rule 2",
		config:         interpreter.DefaultConfig(),
	},
	{
		// Reads new(AuthToken).value[0] before any write (TrackInit mode).
		// go vet: pass, go test -race: pass.
		name: "uninit read",
		dir:  "uninit_read",
		config: func() interpreter.Config {
			c := interpreter.DefaultConfig()
			c.TrackInit = true
			return c
		}(),
		wantViolations: 1,
		wantCategory:   "uninitialized",
	},
	{
		// getPort("ftp") dereferences nil from map miss.
		// go vet: pass, go test -race: pass if "ftp" path not covered.
		name:           "nil deref",
		dir:            "nil_deref",
		wantViolations: 1,
		wantCategory:   "nil pointer",
		config:         interpreter.DefaultConfig(),
	},
	{
		// makeAnimal("cat") returns *Cat; a.(*Dog) panics at runtime.
		// go vet: pass (can't statically trace makeAnimal's return type).
		// go test -race: pass (single goroutine, no concurrent access).
		name:           "type assert panic",
		dir:            "type_assert",
		wantViolations: 1,
		wantCategory:   "type-assertion",
		config:         interpreter.DefaultConfig(),
	},
	{
		// processValue() calls v.Pointer() then doWork() before converting back.
		// go vet: pass (types are correct).
		// go test -race: pass (no concurrent access).
		// Giri: reflect.Value.Pointer() uintptr escapes past a GC safepoint (Rule 5).
		name:           "reflect unsafe",
		dir:            "reflect_unsafe",
		wantViolations: 1,
		wantCategory:   "rule 5",
		config:         interpreter.DefaultConfig(),
	},
	{
		// worker() reads from results channel that main never sends on.
		// go vet: pass (channel operations are type-correct).
		// go test -race: pass (no concurrent data access).
		// Giri: goroutine is permanently blocked — goroutine leak.
		name:           "goroutine leak",
		dir:            "goroutine_leak",
		wantViolations: 1,
		wantCategory:   "goroutine leak",
		config:         interpreter.DefaultConfig(),
	},
}

// TestShowcase validates that each showcase program produces the expected
// violation. These programs compile and pass go vet and go test -race, but
// Giri detects a bug via static SSA interpretation.
func TestShowcase(t *testing.T) {
	for _, tt := range showcaseTests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			wd, err := os.Getwd()
			if err != nil {
				t.Skipf("could not get working directory: %v", err)
			}
			// Showcase programs live at project_root/testdata/showcase/
			absPath := filepath.Join(wd, "..", "..", "testdata", "showcase", tt.dir)

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
					t.Logf("  violations: %v", result.Violations)
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
					t.Errorf("want violation containing %q, got: %v", tt.wantCategory, result.Violations)
				}
			}
		})
	}
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
