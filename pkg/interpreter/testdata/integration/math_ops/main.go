// math_ops verifies that math.* intercepts work cleanly (#72).
//
// Expected: 0 violations.
package main

import "math"

func main() {
	x := -3.5
	y := 2.0

	// Rounding
	a := math.Abs(x)
	b := math.Floor(x)
	c := math.Ceil(x)
	d := math.Round(x)
	e := math.Trunc(x)
	_, _, _, _, _ = a, b, c, d, e

	// Powers and roots
	sq := math.Sqrt(9.0)
	cb := math.Cbrt(8.0)
	pw := math.Pow(2.0, 10.0)
	_, _, _ = sq, cb, pw

	// Logarithms and exponentials
	lg := math.Log(math.E)
	l2 := math.Log2(8.0)
	l10 := math.Log10(100.0)
	ex := math.Exp(1.0)
	e2 := math.Exp2(3.0)
	_, _, _, _, _ = lg, l2, l10, ex, e2

	// Trigonometry
	s := math.Sin(0.0)
	co := math.Cos(0.0)
	t := math.Tan(0.0)
	_, _, _ = s, co, t

	// Min / Max
	mn := math.Min(x, y)
	mx := math.Max(x, y)
	_, _ = mn, mx

	// Modulo
	md := math.Mod(10.0, 3.0)
	_ = md

	// Hypot
	h := math.Hypot(3.0, 4.0)
	_ = h

	// Special values
	inf := math.Inf(1)
	nan := math.NaN()
	isInf := math.IsInf(inf, 1)
	isNaN := math.IsNaN(nan)
	_, _, _ = inf, isInf, isNaN

	// Frexp / Modf (multi-return)
	frac, exp := math.Frexp(8.0)
	_, _ = frac, exp
	i, f := math.Modf(3.75)
	_, _ = i, f
}
