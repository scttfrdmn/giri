// text_encoding_charmap exercises x/text/encoding, encoding/charmap, and
// encoding/unicode intercepts (issue #199). Expected: 0 violations.
package main

import (
	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/encoding/unicode"
)

func main() {
	// encoding: HTMLEscapeUnsupported / ReplaceUnsupported wrap an encoder
	cp437Enc := charmap.CodePage437.NewEncoder()
	escaped := encoding.HTMLEscapeUnsupported(cp437Enc)
	_ = escaped

	replaced := encoding.ReplaceUnsupported(cp437Enc)
	_ = replaced

	// charmap: NewDecoder / NewEncoder
	dec := charmap.Windows1252.NewDecoder()
	_ = dec

	enc := charmap.Windows1252.NewEncoder()
	_ = enc

	// charmap: String
	_ = charmap.Windows1252.String()

	// charmap: ISO8859 variants
	dec2 := charmap.ISO8859_1.NewDecoder()
	_ = dec2

	// encoding/unicode: UTF16
	utf16enc := unicode.UTF16(unicode.LittleEndian, unicode.UseBOM)
	_ = utf16enc

	utf16dec := utf16enc.NewDecoder()
	_ = utf16dec

	// encoding/unicode: BOMOverride
	bo := unicode.BOMOverride(unicode.UTF16(unicode.LittleEndian, unicode.IgnoreBOM).NewDecoder())
	_ = bo

	// encoding/unicode: UTF8 (package-level var)
	utf8enc := unicode.UTF8
	_ = utf8enc

	// encoding/unicode: UTF8BOM
	utf8bom := unicode.UTF8BOM
	_ = utf8bom
}
