// string_index_valid verifies that valid string indexing produces no violations.
// Expected: 0 violations.
package main

func main() {
	s := "hello"
	for i := 0; i < len(s); i++ {
		_ = s[i]
	}
}
