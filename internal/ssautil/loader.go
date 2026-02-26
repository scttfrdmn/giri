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
// If the initial load fails because the target imports the "arena" package but
// GOEXPERIMENT=arenas is not set in the environment, LoadProgram automatically
// retries with GOEXPERIMENT=arenas enabled so that arena programs can be
// analysed without requiring the caller to set the environment variable.
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

	// Check for loading errors.  If any mention "arena", retry with
	// GOEXPERIMENT=arenas so that arena programs load transparently.
	var loadErrs []error
	for _, pkg := range initial {
		for _, e := range pkg.Errors {
			loadErrs = append(loadErrs, fmt.Errorf("%s: %s", pkg.PkgPath, e.Msg))
		}
	}
	if len(loadErrs) > 0 && pkgsHaveArenaError(initial) {
		cfg.Env = withArenaExperiment(os.Environ())
		initial, err = packages.Load(cfg, patterns...)
		if err != nil {
			return nil, fmt.Errorf("loading packages: %w", err)
		}
		loadErrs = nil
		for _, pkg := range initial {
			for _, e := range pkg.Errors {
				loadErrs = append(loadErrs, fmt.Errorf("%s: %s", pkg.PkgPath, e.Msg))
			}
		}
	}
	if len(loadErrs) > 0 {
		return nil, fmt.Errorf("package errors: %v", loadErrs)
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
		SSA:  prog,
		Main: mainPkg,
		Fset: fset,
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
		cfg.Env = withArenaExperiment(os.Environ())
		initial, err = packages.Load(cfg, pattern)
		if err != nil {
			return nil, nil, fmt.Errorf("loading test packages: %w", err)
		}
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

// pkgsHaveArenaError reports whether any loaded package has an error whose
// message mentions "arena", which typically means the package imports
// "arena" but GOEXPERIMENT=arenas is not set.
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

// withArenaExperiment returns a copy of env with "arenas" merged into the
// GOEXPERIMENT variable.  If GOEXPERIMENT is absent it is added; if present
// but "arenas" is not listed, "arenas" is appended to the existing value.
func withArenaExperiment(env []string) []string {
	result := make([]string, len(env))
	copy(result, env)
	for i, e := range result {
		if strings.HasPrefix(e, "GOEXPERIMENT=") {
			if !strings.Contains(e, "arenas") {
				val := strings.TrimPrefix(e, "GOEXPERIMENT=")
				if val == "" {
					result[i] = "GOEXPERIMENT=arenas"
				} else {
					result[i] = "GOEXPERIMENT=" + val + ",arenas"
				}
			}
			return result
		}
	}
	return append(result, "GOEXPERIMENT=arenas")
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
