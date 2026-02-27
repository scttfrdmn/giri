// Command giri is an undefined behavior detector for Go programs.
//
// Giri interprets Go programs via SSA and validates every memory operation
// against a shadow memory system to detect:
//
//   - Use-after-free (including arena-allocated memory)
//   - Double-free
//   - Out-of-bounds access via unsafe pointers
//   - Violations of Go's unsafe.Pointer rules
//   - Arena pointer escapes (returns, globals, channels)
//   - Data races beyond what -race detects
//   - Uninitialized memory reads
//   - Arena and allocation leaks
//
// Usage:
//
//	giri [flags] [packages]
//
// Examples:
//
//	# Check a package
//	giri ./...
//
//	# Check with all detectors
//	giri -all ./cmd/server
//
//	# Check arena safety only
//	giri -arena ./pkg/allocator
//
//	# Reproduce a specific scheduling
//	giri -seed 42 -strategy random ./...
//
//	# JSON output for CI integration
//	giri -format json ./... > giri-report.json
//
//	# Run on test suite
//	giri -test ./pkg/mypackage
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/scttfrdmn/giri/internal/ssautil"
	"github.com/scttfrdmn/giri/pkg/interpreter"
	"github.com/scttfrdmn/giri/pkg/report"
	"github.com/scttfrdmn/giri/pkg/shadow"
)

var (
	// Detector flags
	flagAll    = flag.Bool("all", true, "Enable all detectors")
	flagArena  = flag.Bool("arena", false, "Enable arena safety detector only")
	flagUnsafe = flag.Bool("unsafe", false, "Enable unsafe.Pointer detector only")
	flagRace   = flag.Bool("race", false, "Enable data race detector only")
	flagInit   = flag.Bool("init", false, "Enable uninitialized read detector")

	// Scheduling flags
	flagStrategy = flag.String("strategy", "roundrobin", "Scheduling strategy: roundrobin, random, pct")
	flagSeed     = flag.Int64("seed", 0, "Random seed for reproducible scheduling")
	flagDepth    = flag.Int("depth", 3, "Bug depth for PCT strategy")

	// Output flags
	flagFormat  = flag.String("format", "text", "Output format: text, json, sarif")
	flagVerbose = flag.Bool("v", false, "Verbose output (show all SSA instructions)")

	// Execution flags
	flagMaxSteps      = flag.Uint64("max-steps", 10_000_000, "Maximum SSA instructions to execute")
	flagMaxGoroutines = flag.Int("max-goroutines", 1000, "Maximum concurrent goroutines")
	flagRuns          = flag.Int("runs", 1, "Number of PCT scheduling runs (>1 enables RunN multi-interleaving)")

	// Mode flags
	flagDump = flag.Bool("dump-ssa", false, "Dump SSA and exit (for debugging)")
)

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Giri - Go IR Interpreter for Undefined Behavior Detection\n\n")
		fmt.Fprintf(os.Stderr, "Usage: giri [flags] [packages]\n\n")
		fmt.Fprintf(os.Stderr, "Giri interprets Go programs via SSA and checks every memory\n")
		fmt.Fprintf(os.Stderr, "operation for safety violations that the compiler and runtime\n")
		fmt.Fprintf(os.Stderr, "would miss.\n\n")
		fmt.Fprintf(os.Stderr, "Flags:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  giri ./...                     Check all packages\n")
		fmt.Fprintf(os.Stderr, "  giri -arena ./pkg/allocator    Arena safety only\n")
		fmt.Fprintf(os.Stderr, "  giri -format json ./... > r.json    CI integration\n")
		fmt.Fprintf(os.Stderr, "  giri -format sarif ./... > r.sarif  GitHub code scanning\n")
		fmt.Fprintf(os.Stderr, "  giri -seed 42 -strategy pct ./...  Reproducible concurrency testing\n")
	}

	flag.Parse()

	patterns := flag.Args()
	if len(patterns) == 0 {
		patterns = []string{"./..."}
	}

	// Build configuration
	config := buildConfig()

	// Load all matching programs (supports ./... and multiple patterns)
	fmt.Fprintf(os.Stderr, "Loading packages...\n")
	progs, err := ssautil.LoadAllPrograms(patterns)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(2)
	}
	fmt.Fprintf(os.Stderr, "Found %d main package(s).\n", len(progs))

	// Dump SSA mode (first program only)
	if *flagDump {
		ssautil.DumpSSA(progs[0])
		os.Exit(0)
	}

	// Run interpretation across all programs, collecting violations.
	var allViolations []error
	var lastMemStats shadow.MemoryStats
	for _, prog := range progs {
		var result *interpreter.RunResult
		if *flagRuns > 1 {
			fmt.Fprintf(os.Stderr, "Interpreting %s with PCT scheduler (%d runs, seed=%d)...\n",
				prog.Main.Pkg.Name(), *flagRuns, *flagSeed)
			result = interpreter.RunN(prog, config, *flagRuns, *flagSeed)
		} else {
			fmt.Fprintf(os.Stderr, "Interpreting %s with %s scheduler (seed=%d)...\n",
				prog.Main.Pkg.Name(), *flagStrategy, *flagSeed)
			result = interpreter.Run(prog, config)
		}
		allViolations = append(allViolations, result.Violations...)
		lastMemStats = result.MemStats
	}

	// Build report
	rpt := report.Build(allViolations, &lastMemStats)

	// Output
	format := report.FormatText
	switch *flagFormat {
	case "json":
		format = report.FormatJSON
	case "sarif":
		format = report.FormatSARIF
	}

	if err := rpt.Write(os.Stdout, format); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing report: %v\n", err)
		os.Exit(2)
	}

	os.Exit(rpt.ExitCode())
}

func buildConfig() interpreter.Config {
	config := interpreter.DefaultConfig()

	config.MaxSteps = *flagMaxSteps
	config.MaxGoroutines = *flagMaxGoroutines
	config.Verbose = *flagVerbose

	// Detector selection
	if !*flagAll {
		config.TrackArenas = *flagArena
		config.TrackUnsafe = *flagUnsafe
		config.TrackRaces = *flagRace
		config.TrackInit = *flagInit
	}
	if *flagInit {
		config.TrackInit = true
	}

	// Scheduling
	switch *flagStrategy {
	case "roundrobin":
		config.ScheduleStrategy = interpreter.ScheduleRoundRobin
	case "random":
		config.ScheduleStrategy = interpreter.ScheduleRandom
	case "pct":
		config.ScheduleStrategy = interpreter.SchedulePCT
	}
	config.RandomSeed = *flagSeed
	config.BugDepth = *flagDepth

	return config
}
