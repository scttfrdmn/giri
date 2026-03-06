// strings_complete exercises strings package functions added in v0.71.0:
// Clone, CutPrefix, CutSuffix, ContainsFunc, FieldsFunc, IndexFunc,
// LastIndexAny, LastIndexByte, LastIndexFunc, SplitAfterN, Title,
// ToValidUTF8, TrimFunc, TrimLeftFunc, TrimRightFunc.
// Expected: 0 violations.
package main

import (
	"strings"
	"unicode"
)

func main() {
	// Clone (Go 1.20).
	_ = strings.Clone("hello")

	// CutPrefix / CutSuffix (Go 1.20).
	after, found := strings.CutPrefix("foobar", "foo")
	_, _ = after, found
	before, found2 := strings.CutSuffix("foobar", "bar")
	_, _ = before, found2

	// ContainsFunc (Go 1.21).
	_ = strings.ContainsFunc("hello", unicode.IsUpper)

	// FieldsFunc.
	parts := strings.FieldsFunc("foo bar baz", unicode.IsSpace)
	_ = parts

	// IndexFunc / LastIndexFunc.
	_ = strings.IndexFunc("hello", unicode.IsUpper)
	_ = strings.LastIndexFunc("hello world", unicode.IsSpace)

	// LastIndexAny / LastIndexByte.
	_ = strings.LastIndexAny("hello", "lo")
	_ = strings.LastIndexByte("hello", 'l')

	// SplitAfterN.
	chunks := strings.SplitAfterN("a,b,c", ",", 2)
	_ = chunks

	// Title (deprecated).
	_ = strings.Title("hello world") //nolint:staticcheck

	// ToValidUTF8.
	_ = strings.ToValidUTF8("hello\xffworld", "?")

	// TrimFunc / TrimLeftFunc / TrimRightFunc.
	_ = strings.TrimFunc("  hello  ", unicode.IsSpace)
	_ = strings.TrimLeftFunc("  hello  ", unicode.IsSpace)
	_ = strings.TrimRightFunc("  hello  ", unicode.IsSpace)
}
