// text_precis_encoding exercises x/text/secure/precis, x/text/encoding/japanese,
// and x/text/encoding/htmlindex intercepts (issue #204). Expected: 0 violations.
package main

import (
	"golang.org/x/text/encoding/htmlindex"
	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/language"
	"golang.org/x/text/secure/precis"
)

func main() {
	// precis: UsernameCaseMapped (predefined profile)
	p := precis.UsernameCaseMapped

	// precis: String method
	s, err := p.String("user")
	_, _ = s, err

	// precis: Bytes
	b, err2 := p.Bytes([]byte("user"))
	_, _ = b, err2

	// precis: NewFreeform
	fp := precis.NewFreeform()
	_ = fp

	// precis: NewIdentifier
	ip := precis.NewIdentifier()
	_ = ip

	// precis: NewTransformer
	t := p.NewTransformer()
	_ = t

	// japanese: EUCJP
	enc := japanese.EUCJP
	dec := enc.NewDecoder()
	_ = dec
	encw := enc.NewEncoder()
	_ = encw

	// japanese: ShiftJIS
	sj := japanese.ShiftJIS
	dec2 := sj.NewDecoder()
	_ = dec2

	// japanese: ISO2022JP
	iso := japanese.ISO2022JP
	dec3 := iso.NewDecoder()
	_ = dec3

	// htmlindex: Get
	enc2, err3 := htmlindex.Get("utf-8")
	_, _ = enc2, err3

	// htmlindex: Name
	name, err4 := htmlindex.Name(japanese.ShiftJIS)
	_, _ = name, err4

	// htmlindex: LanguageDefault
	langDef := htmlindex.LanguageDefault(language.Chinese)
	_ = langDef
}
