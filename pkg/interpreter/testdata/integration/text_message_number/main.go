// text_message_number exercises x/text/message, x/text/number, and
// x/text/currency intercepts (issue #202). Expected: 0 violations.
package main

import (
	"golang.org/x/text/currency"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
	"golang.org/x/text/number"
)

func main() {
	// message: NewPrinter and Printf/Sprintf
	p := message.NewPrinter(language.English)
	p.Printf("value: %d\n", 42)
	s := p.Sprintf("hello %s", "world")
	_ = s

	// message: Print/Println
	p.Print("test\n")
	p.Println("line")

	// message: Fprint/Fprintln via Sprint
	s2 := p.Sprint("sprint")
	_ = s2
	s3 := p.Sprintln("sprintln")
	_ = s3

	// message: MatchLanguage
	tag := message.MatchLanguage("en", "fr")
	_ = tag

	// number: Decimal/Scientific/Engineering/Percent
	d := number.Decimal(3.14)
	_ = d
	sci := number.Scientific(2.718)
	_ = sci
	eng := number.Engineering(1234.5)
	_ = eng
	pct := number.Percent(0.75)
	_ = pct

	// currency: MustParseISO
	usd := currency.MustParseISO("USD")
	_ = usd

	// currency: ParseISO
	eur, err := currency.ParseISO("EUR")
	_, _ = eur, err

	// currency: NarrowSymbol/Symbol/ISO
	sym := currency.Symbol(usd)
	_ = sym
	iso := currency.ISO(usd)
	_ = iso
}
