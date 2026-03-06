// text_collate_search exercises golang.org/x/text/collate and x/text/search
// intercepts (issue #200). Expected: 0 violations.
package main

import (
	"golang.org/x/text/collate"
	"golang.org/x/text/language"
	"golang.org/x/text/search"
)

func main() {
	// collate: New
	c := collate.New(language.English)

	// collate: CompareString
	_ = c.CompareString("apple", "banana")

	// collate: Compare
	var buf collate.Buffer
	_ = c.Key(&buf, []byte("hello"))
	_ = c.KeyFromString(&buf, "hello")

	// collate: SortStrings
	strs := []string{"banana", "apple", "cherry"}
	c.SortStrings(strs)

	// collate: Supported
	tags := collate.Supported()
	_ = tags

	// search: New
	m := search.New(language.English)

	// search: IndexString — returns (-1, -1) when no match (conservative)
	start, end := m.IndexString("hello world", "xyz")
	_, _ = start, end

	// search: EqualString
	eq := m.EqualString("hello", "HELLO")
	_ = eq

	// search: Compile
	pat := m.CompileString("hello")
	_ = pat
}
