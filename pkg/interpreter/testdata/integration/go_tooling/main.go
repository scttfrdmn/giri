// go_tooling verifies that go/token, go/ast, go/parser, and go/format are
// correctly intercepted.
//
// Expected: 0 violations.
package main

import (
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
)

func main() {
	// go/token: NewFileSet.
	fset := token.NewFileSet()
	_ = fset

	// go/token: Lookup.
	tok := token.Lookup("func")
	_ = tok

	// go/parser: ParseFile — parse a tiny snippet.
	f, err := parser.ParseFile(fset, "x.go", `package main`, 0)
	_ = f
	_ = err

	// go/ast: IsExported.
	if !ast.IsExported("Foo") {
		var s []int
		_ = s[0] // canary: "Foo" is exported
	}
	if ast.IsExported("bar") {
		var s []int
		_ = s[0] // canary: "bar" is not exported
	}

	// go/ast: Inspect — probe callback once.
	ast.Inspect(f, func(n ast.Node) bool {
		return false
	})

	// go/format: Source — returns empty []byte, nil error.
	src, err2 := format.Source([]byte("package main\n"))
	_ = src
	_ = err2
}
