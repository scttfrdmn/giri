// string_range_utf8 verifies that for i, r := range s yields byte offsets
// as the index, matching Go's actual semantics (#73).
//
// Expected: 0 violations.
package main

func main() {
	// ASCII string: each rune is 1 byte so indices are 0,1,2,3,4.
	ascii := "Hello"
	for i, r := range ascii {
		_ = i
		_ = r
	}

	// Unicode string with multibyte runes.
	// "日本語" — each kanji is 3 bytes in UTF-8.
	// Correct indices: 0, 3, 6.
	jp := "日本語"
	for i, r := range jp {
		_ = i
		_ = r
	}

	// Mixed ASCII and multibyte.
	mixed := "a£b"
	for i, r := range mixed {
		_ = i
		_ = r
	}
}
