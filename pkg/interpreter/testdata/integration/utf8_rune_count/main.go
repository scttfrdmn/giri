// utf8_rune_count verifies that unicode/utf8 and unicode intercepts work (#75).
//
// Expected: 0 violations.
package main

import (
	"unicode"
	"unicode/utf8"
)

func main() {
	// utf8.RuneCountInString
	n := utf8.RuneCountInString("Hello")
	_ = n // 5

	n2 := utf8.RuneCountInString("日本語")
	_ = n2 // 3

	// utf8.ValidString
	ok := utf8.ValidString("valid UTF-8")
	_ = ok // true

	// utf8.ValidRune
	ok2 := utf8.ValidRune('A')
	_ = ok2 // true

	// utf8.RuneLen
	size := utf8.RuneLen('A')
	_ = size // 1

	size2 := utf8.RuneLen('日')
	_ = size2 // 3

	// utf8.DecodeRuneInString
	r, sz := utf8.DecodeRuneInString("Hello")
	_ = r  // 'H'
	_ = sz // 1

	// unicode predicates
	isL := unicode.IsLetter('A')
	_ = isL // true

	isD := unicode.IsDigit('5')
	_ = isD // true

	isS := unicode.IsSpace(' ')
	_ = isS // true

	// unicode transforms
	lo := unicode.ToLower('A')
	_ = lo // 'a'

	hi := unicode.ToUpper('a')
	_ = hi // 'A'
}
