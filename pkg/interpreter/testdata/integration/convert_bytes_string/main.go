// convert_bytes_string verifies that string↔[]byte conversions work
// correctly (#74).
//
// Expected: 0 violations.
package main

func main() {
	// string → []byte
	s := "Hello"
	b := []byte(s)
	_ = b // {72, 101, 108, 108, 111}

	// []byte → string
	b2 := []byte{87, 111, 114, 108, 100}
	s2 := string(b2)
	_ = s2 // "World"

	// Round-trip: string → []byte → string
	original := "Giri"
	encoded := []byte(original)
	decoded := string(encoded)
	_ = decoded

	// Empty conversions.
	emptyStr := ""
	emptyBytes := []byte(emptyStr)
	backToStr := string(emptyBytes)
	_ = backToStr
}
