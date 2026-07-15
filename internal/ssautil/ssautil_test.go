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
	"os"
	"path/filepath"
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

	cats, ok := result[key]
	if !ok {
		t.Errorf("expected suppression for comment line %d (%q); map=%v", commentLine, key, result)
	}
	if len(cats) != 0 {
		t.Errorf("bare //giri:ignore should suppress all (empty category list), got %v", cats)
	}
	if _, ok := result[keyNext]; !ok {
		t.Errorf("expected suppression for line after directive %d (%q); map=%v", commentLine+1, keyNext, result)
	}
}

// TestParseSuppressions_Category checks that a recognized category slug after
// //giri:ignore is captured, scoping suppression to that category (#229).
func TestParseSuppressions_Category(t *testing.T) {
	const src = `package main

//giri:ignore integer-truncation
func ignored() {}
`
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "test.go", src, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	result := ssautil.ParseSuppressions(fset, []*packages.Package{
		{Syntax: []*ast.File{f}},
	})

	pos := fset.Position(f.Comments[0].List[0].Pos())
	key := fmt.Sprintf("%s:%d", pos.Filename, pos.Line)
	cats, ok := result[key]
	if !ok {
		t.Fatalf("expected suppression entry for %q; map=%v", key, result)
	}
	if len(cats) != 1 || cats[0] != "integer-truncation" {
		t.Errorf("expected [integer-truncation], got %v", cats)
	}
}

// TestParseSuppressions_UnknownToken verifies that free-text tokens that are
// not recognized categories (e.g. the legacy "rule 1" form) leave the category
// list empty → suppress-all (backward compatibility with #58).
func TestParseSuppressions_UnknownToken(t *testing.T) {
	const src = `package main

//giri:ignore rule 1
func ignored() {}
`
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "test.go", src, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	result := ssautil.ParseSuppressions(fset, []*packages.Package{
		{Syntax: []*ast.File{f}},
	})

	pos := fset.Position(f.Comments[0].List[0].Pos())
	key := fmt.Sprintf("%s:%d", pos.Filename, pos.Line)
	cats, ok := result[key]
	if !ok {
		t.Fatalf("expected suppression entry for %q; map=%v", key, result)
	}
	if len(cats) != 0 {
		t.Errorf("legacy free-text directive should suppress all (empty list), got %v", cats)
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

// TestSourceHash_ChangesWithContent verifies the transitive source hash on
// LoadAllPrograms is stable across loads and changes when a source file changes (#231).
func TestSourceHash_ChangesWithContent(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module giritest\n\ngo 1.23\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	mainGo := filepath.Join(dir, "main.go")
	if err := os.WriteFile(mainGo, []byte("package main\n\nfunc main() {\n\tx := 300\n\t_ = int8(x)\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	hash1 := loadMainHash(t, dir)
	if hash1 == "" {
		t.Fatal("expected non-empty SourceHash for a normal module")
	}
	// Reload unchanged → same hash.
	if got := loadMainHash(t, dir); got != hash1 {
		t.Errorf("hash must be stable across loads: %q vs %q", hash1, got)
	}
	// Change content → different hash.
	if err := os.WriteFile(mainGo, []byte("package main\n\nfunc main() {\n\tx := 400\n\t_ = int8(x)\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := loadMainHash(t, dir); got == hash1 {
		t.Error("hash must change when a source file changes")
	}
}

func loadMainHash(t *testing.T, dir string) string {
	t.Helper()
	// LoadAllPrograms resolves patterns relative to the process CWD, so chdir.
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() { _ = os.Chdir(old) }()

	progs, err := ssautil.LoadAllPrograms([]string{"."})
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(progs) == 0 {
		t.Fatal("no programs loaded")
	}
	return progs[0].SourceHash
}
