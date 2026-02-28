// Unit tests for internal/ssautil (issue #108).
// ParseSuppressions is tested by constructing a minimal fset and
// packages.Package slice parsed from source strings, without loading real
// packages from disk.
package ssautil_test

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"testing"

	"golang.org/x/tools/go/packages"

	"github.com/scttfrdmn/giri/internal/ssautil"
)

// TestParseSuppressions_Empty verifies that an empty package list returns no suppressions.
func TestParseSuppressions_Empty(t *testing.T) {
	fset := token.NewFileSet()
	result := ssautil.ParseSuppressions(fset, nil)
	if len(result) != 0 {
		t.Errorf("expected empty map for nil packages, got %v", result)
	}
}

// TestParseSuppressions_WithDirective checks that //giri:ignore is recognized
// and both the comment line and the following line are suppressed.
func TestParseSuppressions_WithDirective(t *testing.T) {
	const src = `package main

//giri:ignore
func ignored() {}

func normal() {}
`
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "test.go", src, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	result := ssautil.ParseSuppressions(fset, []*packages.Package{
		{Syntax: []*ast.File{f}},
	})

	// The //giri:ignore comment is on line 3 of the source above.
	if len(f.Comments) == 0 {
		t.Fatal("expected parsed comments in test file")
	}
	pos := fset.Position(f.Comments[0].List[0].Pos())
	commentLine := pos.Line
	filename := pos.Filename

	key := fmt.Sprintf("%s:%d", filename, commentLine)
	keyNext := fmt.Sprintf("%s:%d", filename, commentLine+1)

	if !result[key] {
		t.Errorf("expected suppression for comment line %d (%q); map=%v", commentLine, key, result)
	}
	if !result[keyNext] {
		t.Errorf("expected suppression for line after directive %d (%q); map=%v", commentLine+1, keyNext, result)
	}
}

// TestParseSuppressions_NoDirective verifies that regular comments are not suppressed.
func TestParseSuppressions_NoDirective(t *testing.T) {
	const src = `package main

// This is a regular comment.
func foo() {}
`
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "test.go", src, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	result := ssautil.ParseSuppressions(fset, []*packages.Package{
		{Syntax: []*ast.File{f}},
	})
	if len(result) != 0 {
		t.Errorf("expected no suppressions for regular comments, got %v", result)
	}
}
