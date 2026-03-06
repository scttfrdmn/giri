// math_cmplx verifies that math/cmplx.* functions are correctly intercepted.
//
// Exercises Abs, Phase, Conj, Sqrt, Exp, Log, and trig functions.
//
// Expected: 0 violations.
package main

import (
	"math"
	"math/cmplx"
)

func main() {
	// Abs: |3+4i| = 5.
	a := cmplx.Abs(3 + 4i)
	if math.Abs(a-5.0) > 1e-9 {
		var x []int
		_ = x[0] // canary: fires only if Abs(3+4i) != 5
	}

	// Conj: conj(1+2i) = 1-2i.
	c := cmplx.Conj(1 + 2i)
	if real(c) != 1 || imag(c) != -2 {
		var x []int
		_ = x[0]
	}

	// Sqrt: sqrt(-1) = i.
	s := cmplx.Sqrt(-1 + 0i)
	if math.Abs(real(s)) > 1e-9 || math.Abs(imag(s)-1.0) > 1e-9 {
		var x []int
		_ = x[0]
	}

	// Exp and Log are inverses: exp(log(z)) ≈ z.
	z := complex(2.0, 1.0)
	roundtrip := cmplx.Exp(cmplx.Log(z))
	if math.Abs(real(roundtrip)-real(z)) > 1e-9 {
		var x []int
		_ = x[0]
	}

	// Phase: phase(1+0i) = 0.
	p := cmplx.Phase(1 + 0i)
	if math.Abs(p) > 1e-9 {
		var x []int
		_ = x[0]
	}

	// IsNaN / IsInf / NaN / Inf — just exercise them.
	_ = cmplx.IsNaN(cmplx.NaN())
	_ = cmplx.IsInf(cmplx.Inf())
	_ = cmplx.Sin(1 + 1i)
	_ = cmplx.Cos(1 + 1i)
}
