// text_bidi_runenames exercises x/text/unicode/bidi, x/text/unicode/runenames,
// and x/text/secure/bidirule intercepts (issue #203). Expected: 0 violations.
package main

import (
	"golang.org/x/text/secure/bidirule"
	"golang.org/x/text/unicode/bidi"
	"golang.org/x/text/unicode/runenames"
)

func main() {
	// bidi: AppendReverse
	in := []byte("hello")
	out := bidi.AppendReverse(nil, in)
	_ = out

	// bidi: ReverseString
	rev := bidi.ReverseString("world")
	_ = rev

	// bidi: Lookup
	props, size := bidi.LookupRune('A')
	_, _ = props, size

	// bidi: Lookup for byte slice
	props2, size2 := bidi.Lookup([]byte("A"))
	_, _ = props2, size2

	// bidi: New Paragraph
	para := bidi.Paragraph{}
	n, err := para.SetString("hello world")
	_, _ = n, err

	// bidi: Direction
	dir := para.Direction()
	_ = dir

	// runenames: Name
	name := runenames.Name('A')
	_ = name
	name2 := runenames.Name('\u0041')
	_ = name2

	// bidirule: Direction
	d := bidirule.Direction([]byte("test"))
	_ = d
	d2 := bidirule.DirectionString("test")
	_ = d2

	// bidirule: Valid/ValidString
	v := bidirule.Valid([]byte("test"))
	_ = v
	v2 := bidirule.ValidString("test")
	_ = v2
}
