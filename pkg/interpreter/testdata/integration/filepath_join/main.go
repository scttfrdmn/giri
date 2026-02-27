// filepath_join verifies that path/filepath intercepts work (#83).
//
// Expected: 0 violations.
package main

import "path/filepath"

func main() {
	// Join combines path elements.
	p := filepath.Join("/usr", "local", "bin")
	_ = p // "/usr/local/bin"

	// Dir returns the directory component.
	d := filepath.Dir("/usr/local/bin/giri")
	_ = d // "/usr/local/bin"

	// Base returns the last element.
	b := filepath.Base("/usr/local/bin/giri")
	_ = b // "giri"

	// Ext returns the file extension.
	e := filepath.Ext("archive.tar.gz")
	_ = e // ".gz"

	// Clean normalises the path.
	c := filepath.Clean("/usr/../usr/local")
	_ = c // "/usr/local"

	// IsAbs reports whether a path is absolute.
	abs := filepath.IsAbs("/usr/bin")
	_ = abs // true

	rel := filepath.IsAbs("relative/path")
	_ = rel // false

	// Split separates dir and file.
	dir, file := filepath.Split("/etc/hosts")
	_ = dir  // "/etc/"
	_ = file // "hosts"

	// Abs returns an absolute path.
	a, err := filepath.Abs("relative")
	_ = a
	_ = err

	// Rel returns a relative path between basepath and target.
	r, err2 := filepath.Rel("/a/b", "/a/b/c/d")
	_ = r    // "c/d"
	_ = err2 // nil

	// Match pattern matching.
	matched, err3 := filepath.Match("*.go", "main.go")
	_ = matched // true
	_ = err3    // nil
}
