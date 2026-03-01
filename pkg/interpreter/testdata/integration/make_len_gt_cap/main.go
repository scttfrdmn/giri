// make_len_gt_cap verifies that make([]T, len, cap) where len > cap is reported
// as a "make-invalid" violation.
//
// At runtime Go panics: "makeslice: len larger than cap".
// The compiler rejects constant len>cap, so we compute both values via helper
// functions to bypass compile-time checks.
// Expected: 1 violation, category "make-invalid".
package main

func makeLen() int { return 10 }
func makeCap() int { return 3 }

func main() {
	_ = make([]int, makeLen(), makeCap()) // len=10 > cap=3: runtime panic
}
