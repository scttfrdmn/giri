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
//
//	# Run as an LSP server for editor diagnostics
//	giri lsp
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/scttfrdmn/giri/internal/ssautil"
	"github.com/scttfrdmn/giri/pkg/cache"
	"github.com/scttfrdmn/giri/pkg/interpreter"
	"github.com/scttfrdmn/giri/pkg/lsp"
	"github.com/scttfrdmn/giri/pkg/report"
	"github.com/scttfrdmn/giri/pkg/shadow"
)

// projectConfig represents the optional .giri.json project-level configuration file.
// Fields mirror CLI flags. Absent fields do not override defaults; CLI flags
// always take precedence over file values.
//
// Example .giri.json:
//
//	{
//	  "format":   "sarif",
//	  "strategy": "pct",
//	  "runs":     100,
//	  "seed":     42,
//	  "race":     true,
//	  "unsafe":   true
//	}
type projectConfig struct {
	// Detector selection
	All    *bool `json:"all,omitempty"`
	Arena  *bool `json:"arena,omitempty"`
	Unsafe *bool `json:"unsafe,omitempty"`
	Race   *bool `json:"race,omitempty"`
	Init   *bool `json:"init,omitempty"`

	// Scheduling
	Strategy *string `json:"strategy,omitempty"`
	Seed     *int64  `json:"seed,omitempty"`
	Depth    *int    `json:"depth,omitempty"`
	Runs     *int    `json:"runs,omitempty"`

	// Output
	Format  *string `json:"format,omitempty"`
	Verbose *bool   `json:"verbose,omitempty"`

	// Execution limits
	MaxSteps      *uint64 `json:"max_steps,omitempty"`
	MaxGoroutines *int    `json:"max_goroutines,omitempty"`
}

// loadProjectConfig reads .giri.json from the working directory.
// Returns nil without error if the file does not exist.
func loadProjectConfig() (*projectConfig, error) {
	data, err := os.ReadFile(".giri.json")
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading .giri.json: %w", err)
	}
	var pc projectConfig
	if err := json.Unmarshal(data, &pc); err != nil {
		return nil, fmt.Errorf("parsing .giri.json: %w", err)
	}
	return &pc, nil
}

var (
	// Detector flags
	flagAll    = flag.Bool("all", true, "Enable all detectors")
	flagArena  = flag.Bool("arena", false, "Enable arena safety detector only")
	flagUnsafe = flag.Bool("unsafe", false, "Enable unsafe.Pointer detector only")
	flagRace   = flag.Bool("race", false, "Enable data race detector only")
	flagInit   = flag.Bool("init", false, "Enable uninitialized read detector")
	flagTrunc  = flag.Bool("truncation", false, "Enable integer-overflow-on-conversion detector (opt-in; not included in -all)")
	flagDClose = flag.Bool("double-close", false, "Enable double-close detection for os.File/net.Conn (opt-in; not included in -all)")

	// Scheduling flags
	flagStrategy = flag.String("strategy", "roundrobin", "Scheduling strategy: roundrobin, random, pct")
	flagSeed     = flag.Int64("seed", 0, "Random seed for reproducible scheduling")
	flagDepth    = flag.Int("depth", 3, "Bug depth for PCT strategy")

	// Output flags
	flagFormat        = flag.String("format", "text", "Output format: text, json, sarif, html")
	flagVerbose       = flag.Bool("v", false, "Verbose output (show all SSA instructions)")
	flagMaxViolations = flag.Int("max-violations", 0, "Cap active findings shown in text/html output (0 = unlimited); JSON/SARIF always emit all")
	flagNoCache       = flag.Bool("no-cache", false, "Disable the analysis result cache (always re-interpret)")

	// Execution flags
	flagMaxSteps      = flag.Uint64("max-steps", 10_000_000, "Maximum SSA instructions to execute")
	flagMaxGoroutines = flag.Int("max-goroutines", 1000, "Maximum concurrent goroutines")
	flagRuns          = flag.Int("runs", 1, "Number of PCT scheduling runs (>1 enables RunN multi-interleaving)")

	// Mode flags
	flagDump = flag.Bool("dump-ssa", false, "Dump SSA and exit (for debugging)")
	flagTest = flag.Bool("test", false, "Analyze TestXxx(*testing.T) functions in *_test.go files instead of main")
)

func main() {
	// The `lsp` subcommand runs the editor diagnostics server (#232). It has its
	// own flag set and never returns, so dispatch it before the default flag
	// parsing below.
	if len(os.Args) > 1 && os.Args[1] == "lsp" {
		os.Exit(runLSP(os.Args[2:]))
	}

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Giri - Go IR Interpreter for Undefined Behavior Detection\n\n")
		fmt.Fprintf(os.Stderr, "Usage: giri [flags] [packages]\n\n")
		fmt.Fprintf(os.Stderr, "Giri interprets Go programs via SSA and checks every memory\n")
		fmt.Fprintf(os.Stderr, "operation for safety violations that the compiler and runtime\n")
		fmt.Fprintf(os.Stderr, "would miss.\n\n")
		fmt.Fprintf(os.Stderr, "Project config: if .giri.json exists in the working directory,\n")
		fmt.Fprintf(os.Stderr, "it is loaded as baseline configuration. CLI flags override it.\n\n")
		fmt.Fprintf(os.Stderr, "Flags:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  giri ./...                          Check all packages\n")
		fmt.Fprintf(os.Stderr, "  giri -test ./...                    Analyze TestXxx functions in _test.go files\n")
		fmt.Fprintf(os.Stderr, "  giri -arena ./pkg/allocator         Arena safety only\n")
		fmt.Fprintf(os.Stderr, "  giri -format json ./... > r.json    CI integration\n")
		fmt.Fprintf(os.Stderr, "  giri -format sarif ./... > r.sarif  GitHub code scanning\n")
		fmt.Fprintf(os.Stderr, "  giri -strategy pct -runs 100 ./...  PCT multi-run concurrency testing\n")
		fmt.Fprintf(os.Stderr, "  giri -strategy pct -seed 42 ./...   Reproduce a specific PCT run\n")
		fmt.Fprintf(os.Stderr, "  giri lsp                            Run the LSP diagnostics server (editor integration)\n")
	}

	flag.Parse()

	patterns := flag.Args()
	if len(patterns) == 0 {
		patterns = []string{"./..."}
	}

	// Load optional project-level config file.
	pc, err := loadProjectConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: %v\n", err)
	}

	// Build configuration (file values < defaults; CLI flags override both)
	config := buildConfig(pc)

	// -test mode: analyze TestXxx functions from *_test.go files.
	if *flagTest {
		fmt.Fprintf(os.Stderr, "Loading test packages...\n")
		testProgs, tErr := ssautil.LoadTestPrograms(patterns)
		if tErr != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", tErr)
			os.Exit(2)
		}
		anyFail := false
		var allViolations []error
		for _, prog := range testProgs {
			results := interpreter.RunTests(prog, config)
			for _, tr := range results {
				if tr.Passed() {
					fmt.Fprintf(os.Stderr, "--- PASS: %s\n", tr.Name)
				} else {
					fmt.Fprintf(os.Stderr, "--- FAIL: %s (%d violation(s))\n", tr.Name, len(tr.Violations))
					anyFail = true
					allViolations = append(allViolations, tr.Violations...)
				}
			}
		}
		rpt := report.Build(allViolations, nil)
		rpt.MaxViolations = *flagMaxViolations
		format := report.FormatText
		switch *flagFormat {
		case "json":
			format = report.FormatJSON
		case "sarif":
			format = report.FormatSARIF
		case "html":
			format = report.FormatHTML
		}
		if err := rpt.Write(os.Stdout, format); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing report: %v\n", err)
			os.Exit(2)
		}
		if anyFail {
			os.Exit(1)
		}
		os.Exit(0)
	}

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

	// Resolve the analysis cache (#231). Caching is only sound for deterministic
	// single-run analysis: PCT/random scheduling yields a seed-dependent union of
	// violations that legitimately varies run to run.
	deterministic := *flagRuns <= 1 && *flagStrategy == "roundrobin"
	cacheDir, cacheOK := "", false
	if !*flagNoCache && deterministic {
		cacheDir, cacheOK = cache.Dir()
	}
	cfgFingerprint := cache.Fingerprint(config)

	// Build the report incrementally so cached and freshly-interpreted programs
	// contribute identically. Memory stats reflect only live (non-cached) runs;
	// they stay nil on an all-cache-hit run rather than reporting misleading zeros.
	var lastMemStats shadow.MemoryStats
	haveMemStats := false
	rpt := report.Build(nil, nil)
	rpt.MaxViolations = *flagMaxViolations

	for _, prog := range progs {
		goVer := ""
		if prog.GoVersion != "" {
			goVer = " (" + prog.GoVersion + ")"
		}
		name := prog.Main.Pkg.Name()

		// Cache lookup (deterministic single-run only).
		var key string
		if cacheOK && prog.SourceHash != "" {
			key = cache.Key(prog.SourceHash, cfgFingerprint, report.Version, prog.GoVersion)
			if entry, hit := cache.Load(cacheDir, key); hit {
				rpt.AddFindings(entry.Active, entry.Suppressed)
				lastMemStats = entry.MemStats
				haveMemStats = true // cached mem stats stand in for a live run
				if *flagVerbose {
					fmt.Fprintf(os.Stderr, "Interpreting %s%s... cache: hit\n", name, goVer)
				}
				continue
			}
		}

		var result *interpreter.RunResult
		if *flagRuns > 1 {
			fmt.Fprintf(os.Stderr, "Interpreting %s%s with PCT scheduler (%d runs, seed=%d)...\n",
				name, goVer, *flagRuns, *flagSeed)
			result = interpreter.RunN(prog, config, *flagRuns, *flagSeed)
		} else {
			cacheNote := ""
			if *flagVerbose {
				switch {
				case *flagNoCache:
					cacheNote = " (cache: disabled)"
				case !deterministic:
					cacheNote = " (cache: disabled — non-deterministic scheduling)"
				case !cacheOK:
					cacheNote = " (cache: unavailable)"
				default:
					cacheNote = " (cache: miss)"
				}
			}
			fmt.Fprintf(os.Stderr, "Interpreting %s%s with %s scheduler (seed=%d)...%s\n",
				name, goVer, *flagStrategy, *flagSeed, cacheNote)
			result = interpreter.Run(prog, config)
		}

		active := report.FindingsFrom(result.Violations)
		suppressed := report.FindingsFrom(result.SuppressedViolations)
		rpt.AddFindings(active, suppressed)
		lastMemStats = result.MemStats
		haveMemStats = true

		// Populate the cache for a future run (best-effort).
		if cacheOK && key != "" {
			entry := &cache.Entry{
				Active:          active,
				Suppressed:      suppressed,
				SuppressedCount: result.SuppressedCount,
				MemStats:        result.MemStats,
			}
			if err := cache.Store(cacheDir, key, entry); err != nil && *flagVerbose {
				fmt.Fprintf(os.Stderr, "  cache: store failed: %v\n", err)
			}
		}

		if *flagVerbose && len(result.UnmodeledCalls) > 0 {
			fmt.Fprintf(os.Stderr, "  Unmodeled external calls (%d, returned opaque zero value):\n",
				len(result.UnmodeledCalls))
			for _, c := range result.UnmodeledCalls {
				fmt.Fprintf(os.Stderr, "    %s\n", c)
			}
		}
		if *flagVerbose && result.SuppressedCount > 0 {
			fmt.Fprintf(os.Stderr, "  %d violation(s) suppressed by //giri:ignore\n",
				result.SuppressedCount)
		}
	}

	// Attach memory stats only when at least one program was interpreted live.
	if haveMemStats {
		rpt.Stats = &lastMemStats
	}

	// Output
	format := report.FormatText
	switch *flagFormat {
	case "json":
		format = report.FormatJSON
	case "sarif":
		format = report.FormatSARIF
	case "html":
		format = report.FormatHTML
	}

	if err := rpt.Write(os.Stdout, format); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing report: %v\n", err)
		os.Exit(2)
	}

	os.Exit(rpt.ExitCode())
}

// runLSP runs the `giri lsp` editor-diagnostics server (#232). It parses the
// subcommand's own flags, builds the default analysis configuration (equivalent
// to `giri` with -all), and serves the LSP protocol over stdio until the client
// disconnects. It returns the process exit code.
func runLSP(argv []string) int {
	fs := flag.NewFlagSet("giri lsp", flag.ExitOnError)
	noCache := fs.Bool("no-cache", false, "Disable the analysis result cache (always re-interpret)")
	verbose := fs.Bool("v", false, "Log server activity to stderr")
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: giri lsp [flags]\n\n")
		fmt.Fprintf(os.Stderr, "Run Giri as a Language Server Protocol server over stdio, publishing\n")
		fmt.Fprintf(os.Stderr, "undefined-behavior findings as editor diagnostics on file open and save.\n\n")
		fmt.Fprintf(os.Stderr, "Launch this from your editor's LSP client. Arena programs require the\n")
		fmt.Fprintf(os.Stderr, "server to be started with GOEXPERIMENT=arenas in its environment.\n\n")
		fmt.Fprintf(os.Stderr, "Flags:\n")
		fs.PrintDefaults()
	}
	_ = fs.Parse(argv)

	// The LSP server always runs deterministic single-run analysis (roundrobin),
	// so its results are cacheable and shared with the CLI's cache. Detector
	// selection matches `giri` with -all (unsafe/arena/race on, init off).
	config := interpreter.DefaultConfig()

	var logf func(string, ...interface{})
	if !*verbose {
		logf = func(string, ...interface{}) {} // quiet unless -v
	}

	srv := lsp.NewServer(config, *noCache, logf)
	return srv.Serve(os.Stdin, os.Stdout)
}

func buildConfig(pc *projectConfig) interpreter.Config {
	config := interpreter.DefaultConfig()

	// Apply project-level config first (lowest precedence).
	if pc != nil {
		applyProjectConfig(&config, pc)
	}

	// Apply CLI flags (highest precedence — override file values).
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
	// Integer truncation is opt-in and independent of -all (noisy by design).
	if *flagTrunc {
		config.TrackTruncation = true
	}
	// Double-close is opt-in and independent of -all (defined behavior, not UB).
	if *flagDClose {
		config.TrackDoubleClose = true
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

// applyProjectConfig applies non-nil fields from pc to config.
// CLI flags applied afterwards will override these values.
func applyProjectConfig(config *interpreter.Config, pc *projectConfig) {
	if pc.All != nil && !*pc.All {
		config.TrackArenas = false
		config.TrackUnsafe = false
		config.TrackRaces = false
		config.TrackInit = false
	}
	if pc.Arena != nil {
		config.TrackArenas = *pc.Arena
	}
	if pc.Unsafe != nil {
		config.TrackUnsafe = *pc.Unsafe
	}
	if pc.Race != nil {
		config.TrackRaces = *pc.Race
	}
	if pc.Init != nil {
		config.TrackInit = *pc.Init
	}
	if pc.Strategy != nil {
		switch *pc.Strategy {
		case "roundrobin":
			config.ScheduleStrategy = interpreter.ScheduleRoundRobin
		case "random":
			config.ScheduleStrategy = interpreter.ScheduleRandom
		case "pct":
			config.ScheduleStrategy = interpreter.SchedulePCT
		}
	}
	if pc.Seed != nil {
		config.RandomSeed = *pc.Seed
	}
	if pc.Depth != nil {
		config.BugDepth = *pc.Depth
	}
	if pc.MaxSteps != nil {
		config.MaxSteps = *pc.MaxSteps
	}
	if pc.MaxGoroutines != nil {
		config.MaxGoroutines = *pc.MaxGoroutines
	}
	if pc.Verbose != nil {
		config.Verbose = *pc.Verbose
	}
}
