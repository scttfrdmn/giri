// strconv_complete exercises strconv functions added in v0.71.0:
// QuoteRune, QuoteRuneToASCII, QuoteRuneToGraphic, QuoteToASCII,
// QuoteToGraphic, QuotedPrefix, CanBackquote, IsPrint, IsGraphic,
// ParseComplex, FormatComplex, UnquoteChar, AppendQuote* family.
// Expected: 0 violations.
package main

import "strconv"

func main() {
	// QuoteRune and variants.
	_ = strconv.QuoteRune('x')
	_ = strconv.QuoteRuneToASCII('ñ')
	_ = strconv.QuoteRuneToGraphic('✓')

	// QuoteToASCII / QuoteToGraphic.
	_ = strconv.QuoteToASCII("hello\tworld")
	_ = strconv.QuoteToGraphic("hello world")

	// QuotedPrefix (Go 1.17).
	s, err := strconv.QuotedPrefix(`"hello"world`)
	_, _ = s, err

	// CanBackquote.
	_ = strconv.CanBackquote("simple string")

	// IsPrint / IsGraphic.
	_ = strconv.IsPrint('a')
	_ = strconv.IsGraphic('a')

	// ParseComplex / FormatComplex.
	c, err2 := strconv.ParseComplex("1+2i", 128)
	_, _ = c, err2
	_ = strconv.FormatComplex(complex(1, 2), 'g', -1, 128)

	// UnquoteChar.
	r, multi, tail, err3 := strconv.UnquoteChar(`\t`, '"')
	_, _, _, _ = r, multi, tail, err3

	// AppendQuote family — append to a dst slice.
	dst := []byte("prefix:")
	dst = strconv.AppendQuoteRune(dst, 'x')
	dst = strconv.AppendQuoteRuneToASCII(dst, 'ñ')
	dst = strconv.AppendQuoteRuneToGraphic(dst, '✓')
	dst = strconv.AppendQuoteToASCII(dst, "hi")
	dst = strconv.AppendQuoteToGraphic(dst, "hi")
	_ = dst
}
