// Package ssautil provides utilities for loading Go programs into SSA form
// for interpretation by Giri.
package ssautil

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"go/token"
	"go/types"
	"io"
	"os"
	"sort"
	"strings"

	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"

	"github.com/scttfrdmn/giri/pkg/interpreter"
	"github.com/scttfrdmn/giri/pkg/shadow"
)

// buildSSAFrom builds an SSA program from a set of already-loaded packages.
// This is the single canonical call-site for AllPackages+Build.
func buildSSAFrom(initial []*packages.Package) (*ssa.Program, []*ssa.Package) {
	prog, pkgs := ssautil.AllPackages(initial, ssa.InstantiateGenerics)
	prog.Build()
	return prog, pkgs
}

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
			packages.NeedTypesInfo |
			packages.NeedModule,
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
	prog, pkgs := buildSSAFrom(initial)

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
		GoVersion:    extractGoVersion(initial),
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
			packages.NeedTypesInfo |
			packages.NeedModule,
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

	prog, pkgs := buildSSAFrom(initial)

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
		SSA:       prog,
		Main:      testPkg,
		Fset:      fset,
		GoVersion: extractGoVersion(initial),
	}, testNames, nil
}

// ParseSuppressions scans source files for //giri:ignore comments and returns a
// map from "file:line" position to the list of violation categories that
// directive suppresses (#58, #229).
//
// The category list semantics (consumed by Interpreter.recordViolation):
//   - empty slice  → suppress ANY violation on that line (bare //giri:ignore,
//     or a free-text directive like "//giri:ignore rule 1" whose tokens are not
//     recognized category slugs — preserves backward compatibility)
//   - non-empty    → suppress only violations whose category is in the list
//     (e.g. "//giri:ignore integer-truncation")
//
// Suppression applies to:
//   - the line of the //giri:ignore comment itself (for inline use)
//   - the immediately following line (for preceding-line use)
//
// Example — inline, category-scoped:
//
//	_ = int8(big) //giri:ignore integer-truncation
//
// Example — preceding line, suppress-all:
//
//	//giri:ignore
//	_ = *(*uint32)(unsafe.Pointer(&b[1]))
func ParseSuppressions(fset *token.FileSet, pkgs []*packages.Package) map[string][]string {
	seen := make(map[string][]string)
	for _, pkg := range pkgs {
		for _, file := range pkg.Syntax {
			for _, cg := range file.Comments {
				for _, c := range cg.List {
					text := strings.TrimSpace(strings.TrimPrefix(c.Text, "//"))
					if !strings.HasPrefix(text, "giri:ignore") {
						continue
					}
					// Collect any recognized category slugs following the
					// directive. Unrecognized tokens (e.g. "rule 1", prose) are
					// ignored, leaving the list empty → suppress-all.
					var cats []string
					for _, tok := range strings.Fields(strings.TrimPrefix(text, "giri:ignore")) {
						if shadow.IsKnownCategory(tok) {
							cats = append(cats, tok)
						}
					}
					pos := fset.Position(c.Pos())
					filename := pos.Filename
					line := pos.Line
					// Suppress the comment line (inline) and the next line
					// (preceding-line directive).
					seen[fmt.Sprintf("%s:%d", filename, line)] = cats
					seen[fmt.Sprintf("%s:%d", filename, line+1)] = cats
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
			packages.NeedTypesInfo |
			packages.NeedModule,
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

	prog, pkgs := buildSSAFrom(initial)

	fset := token.NewFileSet()
	if len(initial) > 0 {
		fset = initial[0].Fset
	}
	suppressions := ParseSuppressions(fset, initial)

	goVer := extractGoVersion(initial)
	var programs []*interpreter.Program
	for _, pkg := range pkgs {
		if pkg != nil && pkg.Pkg.Name() == "main" {
			programs = append(programs, &interpreter.Program{
				SSA:          prog,
				Main:         pkg,
				Fset:         fset,
				Suppressions: suppressions,
				GoVersion:    goVer,
				SourceHash:   sourceHashForMain(pkg, initial),
			})
		}
	}

	if len(programs) == 0 {
		return nil, fmt.Errorf("no main packages found in %v", patterns)
	}
	return programs, nil
}

// LoadTestPrograms loads packages in test mode and returns one Program per
// package that contains TestXxx(*testing.T) functions. Each program's
// TestFuncs field lists the discovered test functions in member-iteration order.
//
// This is the ssautil counterpart to RunTests in the interpreter package.
// Use it with giri -test to analyze existing test suites without writing
// standalone main programs.
func LoadTestPrograms(patterns []string) ([]*interpreter.Program, error) {
	cfg := &packages.Config{
		Mode: packages.NeedName |
			packages.NeedFiles |
			packages.NeedCompiledGoFiles |
			packages.NeedImports |
			packages.NeedDeps |
			packages.NeedTypes |
			packages.NeedSyntax |
			packages.NeedTypesInfo |
			packages.NeedModule,
		Tests: true,
	}

	initial, err := packages.Load(cfg, patterns...)
	if err != nil {
		return nil, fmt.Errorf("loading test packages: %w", err)
	}

	if pkgsHaveArenaError(initial) {
		fmt.Fprintf(os.Stderr,
			"warning: some packages import \"arena\" but GOEXPERIMENT=arenas is not set.\n"+
				"  Arena analysis is disabled. Re-run with GOEXPERIMENT=arenas.\n")
	}

	var loadErrs []error
	for _, pkg := range initial {
		for _, e := range pkg.Errors {
			if !strings.Contains(e.Msg, "arena") {
				loadErrs = append(loadErrs, fmt.Errorf("%s: %s", pkg.PkgPath, e.Msg))
			}
		}
	}
	if len(loadErrs) > 0 {
		return nil, fmt.Errorf("package errors: %v", loadErrs)
	}

	prog, pkgs := buildSSAFrom(initial)

	fset := token.NewFileSet()
	if len(initial) > 0 {
		fset = initial[0].Fset
	}
	suppressions := ParseSuppressions(fset, initial)

	// Collect TestXxx functions grouped by their SSA package.
	type pkgEntry struct {
		ssaPkg *ssa.Package
		tests  []interpreter.TestFunc
	}
	// Use a slice to preserve package order deterministically.
	var pkgOrder []*ssa.Package
	pkgTests := make(map[*ssa.Package]*pkgEntry)

	for _, pkg := range pkgs {
		if pkg == nil {
			continue
		}
		for _, mem := range pkg.Members {
			fn, ok := mem.(*ssa.Function)
			if !ok {
				continue
			}
			if isTestFunc(fn) {
				if pkgTests[pkg] == nil {
					pkgTests[pkg] = &pkgEntry{ssaPkg: pkg}
					pkgOrder = append(pkgOrder, pkg)
				}
				pkgTests[pkg].tests = append(pkgTests[pkg].tests, interpreter.TestFunc{
					Name: fn.Name(),
					Fn:   fn,
				})
			}
		}
	}

	goVer := extractGoVersion(initial)
	var programs []*interpreter.Program
	for _, ssaPkg := range pkgOrder {
		entry := pkgTests[ssaPkg]
		programs = append(programs, &interpreter.Program{
			SSA:          prog,
			Main:         ssaPkg,
			Fset:         fset,
			Suppressions: suppressions,
			TestFuncs:    entry.tests,
			GoVersion:    goVer,
		})
	}

	if len(programs) == 0 {
		return nil, fmt.Errorf("no TestXxx(*testing.T) functions found in %v", patterns)
	}
	return programs, nil
}

// isTestFunc reports whether fn is a TestXxx(*testing.T) function eligible
// for Giri analysis. It matches the naming convention (Test followed by a
// capital letter) and verifies the parameter type is exactly *testing.T.
func isTestFunc(fn *ssa.Function) bool {
	name := fn.Name()
	if len(name) < 5 || !strings.HasPrefix(name, "Test") {
		return false
	}
	// "Test" alone or "Testlower..." does not match.
	if name[4] < 'A' || name[4] > 'Z' {
		return false
	}
	// Verify the signature is func(*testing.T).
	sig := fn.Signature
	if sig.Params().Len() != 1 {
		return false
	}
	ptr, ok := sig.Params().At(0).Type().(*types.Pointer)
	if !ok {
		return false
	}
	named, ok := ptr.Elem().(*types.Named)
	if !ok {
		return false
	}
	obj := named.Obj()
	return obj.Name() == "T" && obj.Pkg() != nil && obj.Pkg().Path() == "testing"
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

// extractGoVersion returns the "goX.Y" version string from the first
// package that has module information. Returns "" for non-module programs
// or when NeedModule was not requested.
func extractGoVersion(pkgs []*packages.Package) string {
	for _, pkg := range pkgs {
		if pkg.Module != nil && pkg.Module.GoVersion != "" {
			return "go" + pkg.Module.GoVersion
		}
	}
	return ""
}

// transitiveSourceHash computes a content hash over the Go source files of root
// and every package reachable through its import graph, excluding the standard
// library (#231). Stdlib is pinned by the Go version, which is a separate cache
// key component, so hashing thousands of unchanging stdlib files is both wasteful
// and unnecessary. The hash covers each file's path and byte content, in sorted
// order, so it is stable across runs and changes when any reachable file changes.
// Returns "" if root is nil or no files could be hashed.
func transitiveSourceHash(root *packages.Package) string {
	if root == nil {
		return ""
	}
	// Collect the deduplicated set of source files across the import closure.
	// The root package is always hashed; the stdlib skip applies only to
	// imported dependencies (a local module path like "myapp" has no dot and
	// would otherwise be misclassified as stdlib and dropped).
	files := make(map[string]struct{})
	visited := make(map[string]bool)
	var walk func(p *packages.Package, isRoot bool)
	walk = func(p *packages.Package, isRoot bool) {
		if p == nil || visited[p.PkgPath] {
			return
		}
		visited[p.PkgPath] = true
		if !isRoot && isStdlibPkgPath(p.PkgPath) {
			return // skip stdlib and its subtree (pinned by Go version)
		}
		for _, f := range p.CompiledGoFiles {
			files[f] = struct{}{}
		}
		for _, imp := range p.Imports {
			walk(imp, false)
		}
	}
	walk(root, true)

	if len(files) == 0 {
		return ""
	}
	sorted := make([]string, 0, len(files))
	for f := range files {
		sorted = append(sorted, f)
	}
	sort.Strings(sorted)

	h := sha256.New()
	for _, path := range sorted {
		f, err := os.Open(path)
		if err != nil {
			// A file we can't read makes the hash unreliable; fold the error into
			// the hash so a subsequent successful read produces a different key.
			// (hash.Hash.Write never returns an error.)
			_, _ = io.WriteString(h, "ERR:"+path+"\n")
			continue
		}
		_, _ = io.WriteString(h, "F:"+path+"\n")
		_, _ = io.Copy(h, f)
		_ = f.Close()
	}
	return hex.EncodeToString(h.Sum(nil))
}

// isStdlibPkgPath reports whether importPath belongs to the Go standard library.
// Stdlib import paths have no dot in their first path segment (e.g. "fmt",
// "net/http"), whereas module paths do ("github.com/...", "golang.org/x/...").
func isStdlibPkgPath(importPath string) bool {
	if importPath == "" {
		return false
	}
	first := importPath
	if i := strings.IndexByte(importPath, '/'); i >= 0 {
		first = importPath[:i]
	}
	return !strings.Contains(first, ".")
}

// sourceHashForMain finds the *packages.Package matching the SSA main package by
// import path and returns its transitive source hash (#231). initial holds the
// root packages; their .Imports closure is walked to reach dependencies.
func sourceHashForMain(mainPkg *ssa.Package, initial []*packages.Package) string {
	if mainPkg == nil || mainPkg.Pkg == nil {
		return ""
	}
	want := mainPkg.Pkg.Path()
	var match *packages.Package
	var find func(p *packages.Package)
	seen := make(map[string]bool)
	find = func(p *packages.Package) {
		if p == nil || match != nil || seen[p.PkgPath] {
			return
		}
		seen[p.PkgPath] = true
		if p.PkgPath == want {
			match = p
			return
		}
		for _, imp := range p.Imports {
			find(imp)
		}
	}
	for _, p := range initial {
		find(p)
	}
	return transitiveSourceHash(match)
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
			_, _ = fn.WriteTo(nil) // Writes to stdout; errors are non-actionable here
			fmt.Println()
		}
	}
}
