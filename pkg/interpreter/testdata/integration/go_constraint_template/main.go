// go_constraint_template verifies that go/build/constraint, go/doc/comment,
// and text/template/parse are correctly intercepted.
//
// Expected: 0 violations.
package main

import (
	"go/build/constraint"
	"go/doc/comment"
	"text/template/parse"
)

func main() {
	// go/build/constraint: IsGoBuild / IsPlusBuild.
	ok := constraint.IsGoBuild("//go:build linux")
	_ = ok

	ok2 := constraint.IsPlusBuild("// +build linux")
	_ = ok2

	// go/build/constraint: Parse.
	expr, err := constraint.Parse("//go:build linux && amd64")
	_ = err
	if expr == nil {
		var s []int
		_ = s[0] // canary: parsed expr must be non-nil
	}

	// go/build/constraint: GoVersion.
	ver := constraint.GoVersion(expr)
	_ = ver

	// go/doc/comment: Parser.Parse.
	var p comment.Parser
	doc := p.Parse("Package foo implements bar.")
	if doc == nil {
		var s []int
		_ = s[0] // canary: doc must be non-nil
	}

	// go/doc/comment: Printer.HTML.
	var pr comment.Printer
	html := pr.HTML(doc)
	_ = html

	// text/template/parse: New.
	t := parse.New("test")
	if t == nil {
		var s []int
		_ = s[0] // canary: tree must be non-nil
	}
}
