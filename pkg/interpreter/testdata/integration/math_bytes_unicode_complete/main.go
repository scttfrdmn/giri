// math_bytes_unicode_complete exercises additions from v0.78.0:
// math: FMA, RoundToEven, Erfcinv;
// bytes.Buffer: Available, AvailableBuffer, Peek (Go 1.21);
// unicode: IsSymbol, IsOneOf, To.
// Note: math.Erfinv is intentionally absent — it is the TestUnmodeledCallsReport sentinel.
// Expected: 0 violations.
package main

import (
	"bytes"
	"math"
	"unicode"
)

func main() {
	// math.FMA — fused multiply-add.
	_ = math.FMA(2.0, 3.0, 1.0)

	// math.RoundToEven — round ties to even.
	_ = math.RoundToEven(0.5)
	_ = math.RoundToEven(1.5)

	// math.Erfcinv — inverse of erfc.
	_ = math.Erfcinv(1.0)
	_ = math.Erfcinv(0.5)

	// bytes.Buffer: Available, AvailableBuffer, Peek (Go 1.21).
	var buf bytes.Buffer
	buf.WriteString("hello world")
	avail := buf.Available()
	_ = avail
	availBuf := buf.AvailableBuffer()
	_ = availBuf
	peeked, err := buf.Peek(3)
	_, _ = peeked, err

	// unicode.IsSymbol.
	_ = unicode.IsSymbol('€')
	_ = unicode.IsSymbol('A')

	// unicode.IsOneOf.
	_ = unicode.IsOneOf([]*unicode.RangeTable{unicode.Letter}, 'A')
	_ = unicode.IsOneOf([]*unicode.RangeTable{unicode.Digit}, '3')

	// unicode.To.
	_ = unicode.To(unicode.UpperCase, 'a')
	_ = unicode.To(unicode.LowerCase, 'A')
	_ = unicode.To(unicode.TitleCase, 'a')
}
