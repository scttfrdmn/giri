package interpreter_test

import (
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/scttfrdmn/giri/internal/ssautil"
	"github.com/scttfrdmn/giri/pkg/interpreter"
	"github.com/scttfrdmn/giri/pkg/report"
)

// Programs in testdata/gosched exercise blocking and resumption (#294). Each
// header states the native outcome. Most end in a trap that indexes out of
// range only if the code after the blocking point ran and saw the expected
// state, so an out-of-bounds finding is the pass signal.
var goschedTests = []struct {
	dir  string
	want []string // exact sorted report categories
}{
	{"wg_resume", []string{"out-of-bounds"}},
	{"chan_resume", []string{"out-of-bounds"}},
	{"pingpong", []string{"out-of-bounds"}},
	{"buffered_range", []string{"out-of-bounds"}},
	{"select_wait", []string{"out-of-bounds"}},
	{"sort_block", []string{"out-of-bounds"}},
	{"once_block", []string{"out-of-bounds"}},
	{"defer_block", []string{"out-of-bounds"}},
	{"panic_block", []string{"out-of-bounds"}},
	{"init_block", []string{"out-of-bounds"}},
	{"errgroup_wait", []string{"out-of-bounds"}},
	// The leak is reported, and the leaked goroutine's deferred nil
	// dereference never runs.
	{"leak_no_defers", []string{"goroutine-leak"}},
	{"oncefunc_probe", nil},
	{"wg_deadlock", []string{"deadlock"}},
}

func runGosched(t *testing.T, dir string, cfg interpreter.Config) []string {
	t.Helper()
	abs, err := filepath.Abs(filepath.Join("testdata", "gosched", dir))
	if err != nil {
		t.Fatal(err)
	}
	prog, err := ssautil.LoadProgram(abs)
	if err != nil {
		t.Fatalf("load %s: %v", dir, err)
	}
	res := interpreter.Run(prog, cfg)
	cats := make([]string, 0, len(res.Violations))
	for _, v := range res.Violations {
		cats = append(cats, report.CategoryFor(v))
	}
	slices.Sort(cats)
	return cats
}

func TestGosched(t *testing.T) {
	strategies := map[string]interpreter.ScheduleStrategy{
		"roundrobin": interpreter.ScheduleRoundRobin,
		"random":     interpreter.ScheduleRandom,
		"pct":        interpreter.SchedulePCT,
	}
	for _, tt := range goschedTests {
		for name, strategy := range strategies {
			t.Run(tt.dir+"/"+name, func(t *testing.T) {
				cfg := interpreter.DefaultConfig()
				cfg.ScheduleStrategy = strategy
				cfg.RandomSeed = 7
				got := runGosched(t, tt.dir, cfg)
				if !slices.Equal(got, tt.want) {
					t.Errorf("findings %v, want %v", got, tt.want)
				}
			})
		}
	}
}

// TestGoschedNoHostGoroutineLeak: no host goroutine outlives Run, including
// for programs whose goroutines are aborted or deadlock.
func TestGoschedNoHostGoroutineLeak(t *testing.T) {
	base := runtime.NumGoroutine()
	for _, tt := range goschedTests {
		runGosched(t, tt.dir, interpreter.DefaultConfig())
	}
	// Exiting host goroutines may still be returning when Run does.
	deadline := time.Now().Add(5 * time.Second)
	for runtime.NumGoroutine() > base && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if n := runtime.NumGoroutine(); n > base {
		buf := make([]byte, 1<<16)
		buf = buf[:runtime.Stack(buf, true)]
		t.Fatalf("%d goroutines after Run, %d before:\n%s", n, base, strings.TrimSpace(string(buf)))
	}
}

// TestGoschedDeterministic: the same seed gives the same findings.
func TestGoschedDeterministic(t *testing.T) {
	for _, dir := range []string{"pingpong", "wg_resume", "buffered_range"} {
		var first []string
		for i := 0; i < 5; i++ {
			cfg := interpreter.DefaultConfig()
			cfg.ScheduleStrategy = interpreter.ScheduleRandom
			cfg.RandomSeed = 42
			got := runGosched(t, dir, cfg)
			if i == 0 {
				first = got
			} else if !slices.Equal(got, first) {
				t.Fatalf("%s run %d: %v, first run %v", dir, i, got, first)
			}
		}
	}
}
