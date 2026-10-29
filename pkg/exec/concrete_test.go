package exec

import (
	"go/token"
	"go/types"
	"strings"
	"testing"
)

// stdFunc returns the *types.Func pkgPath.name from the type-checked imports
// of internal/stdimports.
func stdFunc(t *testing.T, imports map[string]*types.Package, pkgPath, name string) *types.Func {
	t.Helper()
	p := imports[pkgPath]
	if p == nil {
		t.Fatalf("package %s not imported by internal/stdimports", pkgPath)
	}
	f, ok := p.Scope().Lookup(name).(*types.Func)
	if !ok {
		t.Fatalf("%s.%s is not a function", pkgPath, name)
	}
	return f
}

func stdImports(t *testing.T) map[string]*types.Package {
	t.Helper()
	pkg, err := LoadDir("internal/stdimports")
	if err != nil {
		t.Fatal(err)
	}
	m := map[string]*types.Package{}
	for _, p := range pkg.Pkg.Imports() {
		m[p.Path()] = p
	}
	return m
}

func TestIsConcrete(t *testing.T) {
	imports := stdImports(t)
	if !IsConcrete(stdFunc(t, imports, "math/bits", "OnesCount64")) {
		t.Error("math/bits.OnesCount64 should be concrete")
	}
	if !IsConcrete(stdFunc(t, imports, "sort", "Ints")) {
		t.Error("sort.Ints should be concrete")
	}
	// strconv.Atoi returns an error value: modeled by the CLI intercepts, not
	// concretely.
	if IsConcrete(stdFunc(t, imports, "strconv", "Atoi")) {
		t.Error("strconv.Atoi should not be concrete")
	}
	// giri's crypto/ecdsa.GenerateKey intercept returns an opaque placeholder.
	ecdsa := types.NewPackage("crypto/ecdsa", "ecdsa")
	genKey := types.NewFunc(token.NoPos, ecdsa, "GenerateKey", types.NewSignatureType(nil, nil, nil, nil, nil, false))
	if IsConcrete(genKey) {
		t.Error("crypto/ecdsa.GenerateKey should not be concrete")
	}
	// Methods are never concrete, even when the name matches a table entry.
	recv := types.NewVar(token.NoPos, nil, "r", types.Typ[types.Int])
	method := types.NewFunc(token.NoPos, types.NewPackage("strings", "strings"), "ToUpper",
		types.NewSignatureType(recv, nil, nil, nil, nil, false))
	if IsConcrete(method) || IsConcrete(nil) {
		t.Error("methods and nil are not concrete")
	}
}

// TestConcreteFuncsExist type-checks every entry of the concrete table: each
// name must be a package-level function of its package.
func TestConcreteFuncsExist(t *testing.T) {
	imports := stdImports(t)
	funcs := ConcreteFuncs()
	if len(funcs) < 100 {
		t.Fatalf("only %d concrete functions", len(funcs))
	}
	for _, qn := range funcs {
		i := strings.LastIndex(qn, ".")
		f := stdFunc(t, imports, qn[:i], qn[i+1:])
		if !IsConcrete(f) {
			t.Errorf("%s listed but IsConcrete false", qn)
		}
	}
}
