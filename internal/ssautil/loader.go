// Package ssautil provides utilities for loading Go programs into SSA form
// for interpretation by Giri.
package ssautil

import (
	"fmt"
	"go/token"
	"os"
	"strings"

	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"

	"github.com/scttfrdmn/giri/pkg/interpreter"
)

// LoadProgram loads a Go package (or packages) into SSA form.
// patterns follows the same conventions as `go build` (e.g., "./...", "./cmd/foo").
//
// If the target imports the "arena" package but GOEXPERIMENT=arenas is not set,
// those packages cannot be compiled. LoadProgram prints a warning to stderr and
// continues with whatever packages did load successfully. Arena-specific checks
// will produce no findings because no arena allocations will be visible. To
// enable full arena analysis, set GOEXPERIMENT=arenas before running giri.
func LoadProgram(patterns ...string) (*interpreter.Program, error) {
	cfg := &packages.Config{
		Mode: packages.NeedName |
			packages.NeedFiles |
			packages.NeedCompiledGoFiles |
			packages.NeedImports |
			packages.NeedDeps |
			packages.NeedTypes |
			packages.NeedSyntax |
			packages.NeedTypesInfo,
	}

	initial, err := packages.Load(cfg, patterns...)
	if err != nil {
		return nil, fmt.Errorf("loading packages: %w", err)
	}

	// Check for loading errors.  Arena errors are treated as a soft warning:
	// the packages that use "arena" are simply absent from the analysis.
	// All other errors are fatal.
	var loadErrs []error
	for _, pkg := range initial {
		for _, e := range pkg.Errors {
			loadErrs = append(loadErrs, fmt.Errorf("%s: %s", pkg.PkgPath, e.Msg))
		}
	}
	if len(loadErrs) > 0 {
		if pkgsHaveArenaError(initial) {
			fmt.Fprintf(os.Stderr,
				"warning: some packages import \"arena\" but GOEXPERIMENT=arenas is not set.\n"+
					"  Arena analysis is disabled. To enable it, re-run with:\n"+
					"  GOEXPERIMENT=arenas giri %s\n",
				strings.Join(patterns, " "),
			)
			// Filter out the arena-related errors; treat the rest as fatal.
			var nonArenaErrs []error
			for _, pkg := range initial {
				for _, e := range pkg.Errors {
					if !strings.Contains(e.Msg, "arena") {
						nonArenaErrs = append(nonArenaErrs, fmt.Errorf("%s: %s", pkg.PkgPath, e.Msg))
					}
				}
			}
			if len(nonArenaErrs) > 0 {
				return nil, fmt.Errorf("package errors: %v", nonArenaErrs)
			}
			// Fall through: build SSA from whatever loaded successfully.
		} else {
			return nil, fmt.Errorf("package errors: %v", loadErrs)
		}
	}

	// Build SSA
	prog, pkgs := ssautil.AllPackages(initial, ssa.InstantiateGenerics)
	prog.Build()

	// Find main package
	var mainPkg *ssa.Package
	for _, pkg := range pkgs {
		if pkg != nil && pkg.Pkg.Name() == "main" {
			mainPkg = pkg
			break
		}
	}

	if mainPkg == nil && len(pkgs) > 0 {
		mainPkg = pkgs[0] // Use first package if no main
	}

	// Collect the file set from loaded packages
	fset := token.NewFileSet()
	if len(initial) > 0 {
		fset = initial[0].Fset
	}

	return &interpreter.Program{
		SSA:          prog,
		Main:         mainPkg,
		Fset:         fset,
		Suppressions: ParseSuppressions(fset, initial),
	}, nil
}

// LoadTest loads a package's test functions into SSA form.
// This is useful for running Giri on test suites.
func LoadTest(pattern string) (*interpreter.Program, []string, error) {
	cfg := &packages.Config{
		Mode: packages.NeedName |
			packages.NeedFiles |
			packages.NeedCompiledGoFiles |
			packages.NeedImports |
			packages.NeedDeps |
			packages.NeedTypes |
			packages.NeedSyntax |
			packages.NeedTypesInfo,
		Tests: true,
	}

	initial, err := packages.Load(cfg, pattern)
	if err != nil {
		return nil, nil, fmt.Errorf("loading test packages: %w", err)
	}

	if pkgsHaveArenaError(initial) {
		fmt.Fprintf(os.Stderr,
			"warning: this package imports \"arena\" but GOEXPERIMENT=arenas is not set.\n"+
				"  Arena analysis is disabled. To enable it, re-run with:\n"+
				"  GOEXPERIMENT=arenas giri %s\n",
			pattern,
		)
	}

	prog, pkgs := ssautil.AllPackages(initial, ssa.InstantiateGenerics)
	prog.Build()

	// Find test functions
	var testNames []string
	var testPkg *ssa.Package
	for _, pkg := range pkgs {
		if pkg == nil {
			continue
		}
		for _, mem := range pkg.Members {
			if fn, ok := mem.(*ssa.Function); ok {
				name := fn.Name()
				if len(name) > 4 && name[:4] == "Test" {
					testNames = append(testNames, name)
					if testPkg == nil {
						testPkg = pkg
					}
				}
			}
		}
	}

	fset := token.NewFileSet()
	if len(initial) > 0 {
		fset = initial[0].Fset
	}

	return &interpreter.Program{
		SSA:  prog,
		Main: testPkg,
		Fset: fset,
	}, testNames, nil
}

// ParseSuppressions scans source files for //giri:ignore comments and returns
// a set of "file:line" positions that should suppress violations (#58).
//
// Suppression applies to:
//   - the line of the //giri:ignore comment itself (for inline use)
//   - the immediately following line (for preceding-line use)
//
// Example — inline:
//
//	_ = *(*uint32)(unsafe.Pointer(&b[1])) //giri:ignore rule 1
//
// Example — preceding line:
//
//	//giri:ignore rule 1
//	_ = *(*uint32)(unsafe.Pointer(&b[1]))
func ParseSuppressions(fset *token.FileSet, pkgs []*packages.Package) map[string]bool {
	seen := make(map[string]bool)
	for _, pkg := range pkgs {
		for _, file := range pkg.Syntax {
			for _, cg := range file.Comments {
				for _, c := range cg.List {
					text := strings.TrimSpace(strings.TrimPrefix(c.Text, "//"))
					if strings.HasPrefix(text, "giri:ignore") {
						pos := fset.Position(c.Pos())
						filename := pos.Filename
						line := pos.Line
						// Suppress the comment line (inline) and the next line
						// (preceding-line directive).
						seen[fmt.Sprintf("%s:%d", filename, line)] = true
						seen[fmt.Sprintf("%s:%d", filename, line+1)] = true
					}
				}
			}
		}
	}
	return seen
}

// LoadAllPrograms loads all main packages matching the given patterns and
// returns one Program per main package found (#53). This supports `giri ./...`
// and other multi-package invocations.
func LoadAllPrograms(patterns []string) ([]*interpreter.Program, error) {
	cfg := &packages.Config{
		Mode: packages.NeedName |
			packages.NeedFiles |
			packages.NeedCompiledGoFiles |
			packages.NeedImports |
			packages.NeedDeps |
			packages.NeedTypes |
			packages.NeedSyntax |
			packages.NeedTypesInfo,
	}

	initial, err := packages.Load(cfg, patterns...)
	if err != nil {
		return nil, fmt.Errorf("loading packages: %w", err)
	}

	var loadErrs []error
	for _, pkg := range initial {
		for _, e := range pkg.Errors {
			loadErrs = append(loadErrs, fmt.Errorf("%s: %s", pkg.PkgPath, e.Msg))
		}
	}
	if len(loadErrs) > 0 {
		if pkgsHaveArenaError(initial) {
			fmt.Fprintf(os.Stderr,
				"warning: some packages import \"arena\" but GOEXPERIMENT=arenas is not set.\n")
		} else {
			return nil, fmt.Errorf("package errors: %v", loadErrs)
		}
	}

	prog, pkgs := ssautil.AllPackages(initial, ssa.InstantiateGenerics)
	prog.Build()

	suppressions := ParseSuppressions(initial[0].Fset, initial)

	fset := token.NewFileSet()
	if len(initial) > 0 {
		fset = initial[0].Fset
	}

	var programs []*interpreter.Program
	for _, pkg := range pkgs {
		if pkg != nil && pkg.Pkg.Name() == "main" {
			programs = append(programs, &interpreter.Program{
				SSA:          prog,
				Main:         pkg,
				Fset:         fset,
				Suppressions: suppressions,
			})
		}
	}

	if len(programs) == 0 {
		return nil, fmt.Errorf("no main packages found in %v", patterns)
	}
	return programs, nil
}

// pkgsHaveArenaError reports whether any loaded package has an error whose
// message mentions "arena", indicating the package imports "arena" but
// GOEXPERIMENT=arenas is not set.
func pkgsHaveArenaError(pkgs []*packages.Package) bool {
	for _, pkg := range pkgs {
		for _, e := range pkg.Errors {
			if strings.Contains(e.Msg, "arena") {
				return true
			}
		}
	}
	return false
}

// DumpSSA prints the SSA for a package (useful for debugging).
func DumpSSA(prog *interpreter.Program) {
	if prog.Main == nil {
		fmt.Println("No main package")
		return
	}

	for _, mem := range prog.Main.Members {
		if fn, ok := mem.(*ssa.Function); ok {
			fmt.Printf("=== %s ===\n", fn.Name())
			fn.WriteTo(nil) // Writes to stdout
			fmt.Println()
		}
	}
}
