// suppress_category_mismatch verifies that a category-scoped //giri:ignore
// directive does NOT suppress a violation of a different category (#229).
//
// The int8(300) conversion truncates (integer-truncation), but the directive
// names an unrelated category (double-close), so the violation is still
// reported. Expected: 1 violation (integer-truncation).
// (Requires TrackTruncation=true — set in the test table.)
package main

func narrow(v int) int8 {
	//giri:ignore double-close
	return int8(v)
}

func main() {
	_ = narrow(300)
}
