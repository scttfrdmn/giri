// html_escape verifies that html.* and unicode/utf16.* functions are
// correctly intercepted.
//
// Expected: 0 violations.
package main

import (
	"html"
	"unicode/utf16"
)

func main() {
	// html.EscapeString: & → &amp;
	escaped := html.EscapeString("<b>hello & world</b>")
	if len(escaped) == 0 {
		var x []int
		_ = x[0] // canary: escaped must be non-empty
	}

	// html.UnescapeString round-trip.
	original := html.UnescapeString(escaped)
	_ = original

	// unicode/utf16.IsSurrogate: 0xD800 is a surrogate.
	if !utf16.IsSurrogate(0xD800) {
		var x []int
		_ = x[0]
	}

	// utf16.EncodeRune / DecodeRune round-trip for a supplementary character.
	r1, r2 := utf16.EncodeRune(0x1F600) // U+1F600 GRINNING FACE
	decoded := utf16.DecodeRune(r1, r2)
	if decoded != 0x1F600 {
		var x []int
		_ = x[0]
	}

	// utf16.Encode/Decode — just ensure they don't crash.
	_ = utf16.Encode([]rune("hello"))
}
