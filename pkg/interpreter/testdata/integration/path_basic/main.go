// path_basic verifies that path (slash-only) intercepts work (#83).
//
// Expected: 0 violations.
package main

import "path"

func main() {
	// Join combines URL-style path segments.
	p := path.Join("api", "v1", "users")
	_ = p // "api/v1/users"

	// Dir returns the directory component.
	d := path.Dir("/api/v1/users")
	_ = d // "/api/v1"

	// Base returns the last segment.
	b := path.Base("/api/v1/users")
	_ = b // "users"

	// Ext returns the extension.
	e := path.Ext("config.yaml")
	_ = e // ".yaml"

	// Clean normalises slashes.
	c := path.Clean("/api//v1/./users")
	_ = c // "/api/v1/users"

	// IsAbs reports whether path starts with /.
	abs := path.IsAbs("/api/v1")
	_ = abs // true

	rel := path.IsAbs("api/v1")
	_ = rel // false

	// Split separates dir and file.
	dir, file := path.Split("/api/v1/users")
	_ = dir  // "/api/v1/"
	_ = file // "users"
}
