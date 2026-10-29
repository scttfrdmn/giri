package exec

import (
	"fmt"

	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"
)

// loadDir loads the single package in dir and builds SSA for all of its
// functions in one call. Dependencies are type-checked but get no SSA
// bodies: standard-library calls are served by giri's intercepts and never
// interpreted, so their bodies are not needed.
//
// Dependencies are type-checked from source (NeedDeps) rather than read from
// export data, matching internal/ssautil: x/tools v0.41's export-data reader
// cannot read the export data written by newer Go toolchains.
func loadDir(dir string) (*ssa.Package, error) {
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
		return nil, fmt.Errorf("exec: %s: %v", dir, pkgs[0].Errors[0])
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
