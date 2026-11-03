package exec

import (
	"fmt"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"sync"

	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"
)

// LoadDir loads the single package in dir (which must belong to a module) and
// builds SSA for all of its functions in one call, with
// ssa.SanityCheckFunctions and ssa.BuildSerially off and generic
// instantiations built. Dependencies are type-checked but get no SSA
// bodies: standard-library calls are served by giri's intercepts and never
// interpreted, so their bodies are not needed.
//
// Dependencies are type-checked from source (NeedDeps) rather than read from
// export data, matching internal/ssautil. Export data is faster for packages
// with several imports but slower for packages with none; see
// docs/decisions.md (req 7).
func LoadDir(dir string) (*ssa.Package, error) {
	cfg := &packages.Config{
		Dir: dir,
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles |
			packages.NeedImports | packages.NeedDeps | packages.NeedTypes |
			packages.NeedSyntax | packages.NeedTypesInfo,
	}
	pkgs, err := packages.Load(cfg, ".")
	if err != nil {
		return nil, fmt.Errorf("exec: loading %s: %w", dir, err)
	}
	if len(pkgs) != 1 {
		return nil, fmt.Errorf("exec: %s: want one package, found %d", dir, len(pkgs))
	}
	if len(pkgs[0].Errors) > 0 {
		return nil, fmt.Errorf("exec: %s: %w", dir, pkgs[0].Errors[0])
	}

	// No SanityCheckFunctions, no BuildSerially: build as fast as possible.
	// ssautil.Packages gives only the initial package function bodies.
	_, ssaPkgs := ssautil.Packages(pkgs, ssa.InstantiateGenerics)
	if ssaPkgs[0] == nil {
		return nil, fmt.Errorf("exec: %s: package is ill-typed", dir)
	}
	ssaPkgs[0].Build()
	return ssaPkgs[0], nil
}

// Loader builds SSA packages from Go source in memory, type-checking
// dependency packages once and reusing them across loads (#273). It is meant
// for callers that load many small packages over a long run, such as one
// package of candidate functions per generation: LoadDir runs `go list` and
// type-checks every dependency from source on every call.
//
// A Loader keeps no reference to the packages it returns. Dropping a package
// releases its SSA; only the dependency types are retained, and those are
// shared by every load. Each load gets a fresh token.FileSet, so positions
// (Prog.Fset.Position(v.Pos()).Line) are relative to that load's source,
// whatever was loaded before, and the same source always yields the same
// functions.
//
// Load is safe to call from multiple goroutines (loads are serialized), and
// while Runs on functions from earlier loads are in flight.
//
// Dependencies are resolved with the go/build default context against the
// toolchain giri runs under (GOROOT, GOOS, GOARCH). They should be
// standard-library packages: no cgo, no module dependencies.
type Loader struct {
	mu  sync.Mutex
	imp types.Importer // caches type-checked dependencies; not concurrency-safe
}

// NewLoader returns an empty Loader.
func NewLoader() *Loader {
	return &Loader{imp: importer.ForCompiler(token.NewFileSet(), "source", nil)}
}

// Load parses src as a single-file package with import path pkgPath and
// builds SSA for all of its functions, as LoadDir does (generic
// instantiations built; no sanity checks; parallel build). The file is
// named pkgPath+".go" in positions. Syntax and type errors are returned.
func (l *Loader) Load(pkgPath string, src []byte) (*ssa.Package, error) {
	if pkgPath == "" {
		return nil, fmt.Errorf("exec: Load: empty package path")
	}
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, pkgPath+".go", src, parser.SkipObjectResolution)
	if err != nil {
		return nil, fmt.Errorf("exec: Load %s: %w", pkgPath, err)
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	pkg := types.NewPackage(pkgPath, f.Name.Name)
	ssaPkg, _, err := ssautil.BuildPackage(&types.Config{Importer: l.imp}, fset, pkg, []*ast.File{f}, ssa.InstantiateGenerics)
	if err != nil {
		return nil, fmt.Errorf("exec: Load %s: %w", pkgPath, err)
	}
	return ssaPkg, nil
}
