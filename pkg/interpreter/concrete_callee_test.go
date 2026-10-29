package interpreter

import (
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"testing"

	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"
)

// TestConcreteCalleeRejectsMethods: the concrete table is keyed by package
// function name, so a same-named method must not match (#257).
func TestConcreteCalleeRejectsMethods(t *testing.T) {
	const src = `package p

import "strings"

func F(s string) (string, string) {
	return strings.NewReplacer("a", "b").Replace(s), strings.Replace(s, "a", "b", -1)
}
`
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "p.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	pkg, _, err := ssautil.BuildPackage(&types.Config{Importer: importer.ForCompiler(fset, "source", nil)},
		fset, types.NewPackage("p", "p"), []*ast.File{f}, ssa.SanityCheckFunctions)
	if err != nil {
		t.Fatal(err)
	}
	var method, function *ssa.Call
	for _, b := range pkg.Func("F").Blocks {
		for _, instr := range b.Instrs {
			c, ok := instr.(*ssa.Call)
			if !ok || c.Call.StaticCallee() == nil || c.Call.StaticCallee().Name() != "Replace" {
				continue
			}
			if c.Call.StaticCallee().Signature.Recv() != nil {
				method = c
			} else {
				function = c
			}
		}
	}
	if method == nil || function == nil {
		t.Fatal("Replace calls not found")
	}
	if concreteCallee(method.Call.StaticCallee()) != nil {
		t.Error("(*strings.Replacer).Replace matched the concrete strings.Replace")
	}
	if IsPure(method) {
		t.Error("method call classified pure")
	}
	if concreteCallee(function.Call.StaticCallee()) == nil {
		t.Error("strings.Replace not matched")
	}
	if !IsPure(function) {
		t.Error("strings.Replace on scalars not pure")
	}
}
