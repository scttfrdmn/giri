// fmt_strings_slices_complete exercises additions from v0.79.0:
// fmt: Append, Appendf, Appendln (Go 1.19);
// strings: SplitSeq, FieldsSeq, Lines (Go 1.24 iterators);
// slices: Sorted, SortedFunc, SortedStableFunc (Go 1.23/1.24 iterator collectors).
// Expected: 0 violations.
package main

import (
	"cmp"
	"fmt"
	"slices"
	"strings"
)

func main() {
	// fmt.Append / Appendf / Appendln (Go 1.19).
	b := []byte("prefix: ")
	b = fmt.Append(b, "hello", " ", "world")
	b = fmt.Appendf(b, " %d", 42)
	b = fmt.Appendln(b, "done")
	_ = b

	// strings.SplitSeq (Go 1.24).
	seq := strings.SplitSeq("a,b,c", ",")
	_ = seq

	// strings.FieldsSeq (Go 1.24).
	fseq := strings.FieldsSeq("hello world  foo")
	_ = fseq

	// strings.Lines (Go 1.24).
	lseq := strings.Lines("line1\nline2\nline3")
	_ = lseq

	// slices.Sorted (Go 1.23).
	nums := strings.SplitSeq("3,1,2", ",")
	_ = slices.Sorted(nums)

	// slices.SortedFunc (Go 1.24).
	nums2 := strings.SplitSeq("c,a,b", ",")
	_ = slices.SortedFunc(nums2, func(a, b string) int {
		return cmp.Compare(a, b)
	})

	// slices.SortedStableFunc (Go 1.24).
	nums3 := strings.SplitSeq("3,1,2", ",")
	_ = slices.SortedStableFunc(nums3, func(a, b string) int {
		return cmp.Compare(a, b)
	})
}
