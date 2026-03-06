// text_cases_language exercises golang.org/x/text/cases, language, and transform
// intercepts (issue #193). Expected: 0 violations.
package main

import (
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"golang.org/x/text/transform"
)

func main() {
	// language: parse tags
	tag, err := language.Parse("en-US")
	_ = err
	_ = tag

	// language: well-known constants
	eng := language.English
	_ = eng

	// language: NewMatcher
	matcher := language.NewMatcher([]language.Tag{language.English, language.French})
	_ = matcher

	// cases: constructors return a Caser
	lower := cases.Lower(language.English)
	upper := cases.Upper(language.English)
	title := cases.Title(language.English)
	fold := cases.Fold()
	_ = fold

	// Caser.String: no-op passthrough
	_ = lower.String("Hello World")
	_ = upper.String("hello world")
	_ = title.String("hello world")

	// transform: String with a nop transformer
	result, _, err2 := transform.String(transform.Discard, "hello")
	_ = result
	_ = err2

	// transform: Bytes
	b, _, err3 := transform.Bytes(transform.Discard, []byte("world"))
	_ = b
	_ = err3

	// transform: NewReader / NewWriter — opaque
	// (not exercised further to keep test self-contained)
}
