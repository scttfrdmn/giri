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
