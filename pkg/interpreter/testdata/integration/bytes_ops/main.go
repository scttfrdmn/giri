// bytes_ops verifies that common bytes.* functions are intercepted cleanly (#66).
//
// Without intercepts, these calls fall through to execFunction and attempt
// to interpret stdlib internals, causing interpreter crashes or wrong results.
//
// Expected: 0 violations.
package main

import "bytes"

func main() {
	b := []byte("Hello, World!")

	_ = bytes.Contains(b, []byte("World"))
	_ = bytes.HasPrefix(b, []byte("Hello"))
	_ = bytes.HasSuffix(b, []byte("!"))
	_ = bytes.Count(b, []byte("l"))
	_ = bytes.Index(b, []byte("World"))

	lower := bytes.ToLower(b)
	upper := bytes.ToUpper(b)
	_ = lower
	_ = upper

	trimmed := bytes.TrimSpace([]byte("  hello  "))
	_ = trimmed

	parts := bytes.Split(b, []byte(", "))
	_ = parts

	replaced := bytes.ReplaceAll(b, []byte("World"), []byte("Giri"))
	_ = replaced

	a, _, found := bytes.Cut(b, []byte(", "))
	_ = a
	_ = found
}
