// regexp_match verifies that regexp intercepts work cleanly (#71).
//
// Expected: 0 violations.
package main

import "regexp"

func main() {
	// MustCompile returns an opaque *Regexp value.
	re := regexp.MustCompile(`\d+`)
	_ = re

	// Compile returns (*Regexp, error).
	re2, err := regexp.Compile(`[a-z]+`)
	if err != nil {
		return
	}
	_ = re2

	// Package-level MatchString returns (bool, error).
	ok, err2 := regexp.MatchString(`\d+`, "hello123")
	if err2 != nil {
		return
	}
	_ = ok

	// *Regexp method calls.
	matched := re.MatchString("abc123")
	_ = matched

	found := re.FindString("price: 42 dollars")
	_ = found

	all := re.FindAllString("1 and 2 and 3", -1)
	_ = all

	replaced := re.ReplaceAllString("hello 42 world", "NUM")
	_ = replaced

	parts := re.Split("a1b2c3", -1)
	_ = parts

	// QuoteMeta
	quoted := regexp.QuoteMeta("a.b+c")
	_ = quoted
}
