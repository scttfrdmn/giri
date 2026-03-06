// text_unicode_norm exercises golang.org/x/text/unicode/norm, width, and runes
// intercepts (issue #194). Expected: 0 violations.
package main

import (
	"unicode"

	"golang.org/x/text/runes"
	"golang.org/x/text/unicode/norm"
	"golang.org/x/text/width"
)

func main() {
	// norm: String passthrough
	s := "café"
	nfc := norm.NFC.String(s)
	nfd := norm.NFD.String(s)
	nfkc := norm.NFKC.String(s)
	nfkd := norm.NFKD.String(s)
	_, _, _, _ = nfc, nfd, nfkc, nfkd

	// norm: Bytes
	b := norm.NFC.Bytes([]byte(s))
	_ = b

	// norm: IsNormal — conservative false
	_ = norm.NFC.IsNormalString(s)

	// width: LookupRune
	props := width.LookupRune('Ａ') // fullwidth A
	_ = props

	// width: Fold/Narrow/Widen are transformer vars — treat as opaque
	// Access them directly to exercise intercept
	_ = width.Fold
	_ = width.Narrow
	_ = width.Widen

	// runes: Map
	mapT := runes.Map(func(r rune) rune { return r })
	_ = mapT

	// runes: Remove
	removeT := runes.Remove(runes.In(unicode.Cc))
	_ = removeT

	// runes: ReplaceIllFormed
	replT := runes.ReplaceIllFormed()
	_ = replT
}
