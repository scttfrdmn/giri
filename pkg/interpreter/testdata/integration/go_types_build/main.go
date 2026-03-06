// go_types_build verifies that go/types, go/importer, go/build, and go/doc
// are correctly intercepted.
//
// Expected: 0 violations.
package main

import (
	"go/build"
	"go/doc"
	"go/importer"
	"go/types"
)

func main() {
	// go/types: basic constructor calls.
	pkg := types.NewPackage("pkg/path", "mypkg")
	_ = pkg

	scope := types.NewScope(nil, 0, 0, "test")
	_ = scope

	// go/types: predicate functions.
	ok := types.Implements(nil, nil)
	_ = ok

	ok2 := types.AssignableTo(nil, nil)
	_ = ok2

	s := types.TypeString(nil, nil)
	if len(s) == 0 {
		var sl []int
		_ = sl[0] // canary: TypeString must be non-empty
	}

	// go/importer: Default.
	imp := importer.Default()
	if imp == nil {
		var sl []int
		_ = sl[0] // canary: importer must be non-nil
	}

	// go/build: Default.Context is a var; package-level funcs.
	isLocal := build.IsLocalImport("./foo")
	_ = isLocal

	// go/doc: Synopsis.
	syn := doc.Synopsis("Package foo implements the foo interface. More text here.")
	_ = syn
}
