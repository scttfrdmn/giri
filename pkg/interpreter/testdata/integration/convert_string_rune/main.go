// convert_string_rune verifies that integer-to-string conversion works
// correctly: string(r) produces a string containing that Unicode code point (#74).
//
// Expected: 0 violations.
package main

func main() {
	// int → string conversion: string(65) = "A"
	r1 := rune(65)
	s1 := string(r1)
	_ = s1 // "A"

	// Another rune.
	r2 := rune('€') // U+20AC
	s2 := string(r2)
	_ = s2 // "€"

	// int32 (rune alias) → string
	var r3 int32 = 72
	s3 := string(r3)
	_ = s3 // "H"

	// float → int (truncation)
	f := 3.99
	i := int(f)
	_ = i // 3

	// int → float (promotion)
	n := 42
	g := float64(n)
	_ = g // 42.0
}
