// suppress_category_match verifies that a category-scoped //giri:ignore
// directive suppresses a violation of that exact category (#229).
//
// The int8(300) conversion truncates (integer-truncation). The directive names
// that category, so the violation is suppressed. Expected: 0 violations.
// (Requires TrackTruncation=true — set in the test table.)
package main

func narrow(v int) int8 {
	//giri:ignore integer-truncation
	return int8(v)
}

func main() {
	_ = narrow(300)
}
