// Package unsafek holds kernels that misuse unsafe.Pointer on purpose, for
// tests that the value cache does not hide what the detectors report.
package unsafek

import (
	"math"
	"unsafe"
)

// GCPoint holds a uintptr across a call, a GC safepoint, when hold is true:
// an unsafe.Pointer rule 2 violation. math.Sqrt is a pure intercept, so its
// call is cacheable.
func GCPoint(x float64, hold bool) float64 {
	v := 7
	var u uintptr
	if hold {
		u = uintptr(unsafe.Pointer(&v))
	}
	r := math.Sqrt(x)
	if hold {
		p := (*int)(unsafe.Pointer(u))
		_ = p
	}
	return r
}

// PtrArith reads a[n] through uintptr arithmetic (#255). For n outside 0..3
// the pointer leaves the array: an unsafe.Pointer rule 3 violation.
func PtrArith(n int) int {
	a := [4]int{1, 2, 3, 4}
	u := uintptr(unsafe.Pointer(&a[0])) + uintptr(n)*8
	q := (*int)(unsafe.Pointer(u))
	return *q
}

// PtrDiff returns the distance in bytes between two elements of an array,
// computed on uintptrs.
func PtrDiff(i, j int) int {
	var a [8]int64
	return int(uintptr(unsafe.Pointer(&a[j])) - uintptr(unsafe.Pointer(&a[i])))
}
