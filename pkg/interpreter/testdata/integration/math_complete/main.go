// math_complete exercises math package functions added in v0.68.0:
// Sincos, Asinh/Acosh/Atanh, Float64bits/Float64frombits,
// Float32bits/Float32frombits, Nextafter/Nextafter32, Jn/Y0/Y1/Yn.
// Expected: 0 violations.
package main

import "math"

func main() {
	// Sincos: returns (sin, cos) simultaneously.
	sin1, cos1 := math.Sincos(1.0)
	_, _ = sin1, cos1

	// Inverse hyperbolic functions.
	_ = math.Asinh(0.5)
	_ = math.Acosh(1.5)
	_ = math.Atanh(0.5)

	// Float64 bit conversions.
	u64 := math.Float64bits(1.5)
	_ = math.Float64frombits(u64)

	// Float32 bit conversions.
	var f32 float32 = 1.5
	u32 := math.Float32bits(f32)
	_ = math.Float32frombits(u32)

	// Next representable floating-point value.
	_ = math.Nextafter(1.0, 2.0)
	_ = math.Nextafter32(float32(1.0), float32(2.0))

	// Bessel functions.
	_ = math.Jn(2, 1.0)
	_ = math.Y0(1.0)
	_ = math.Y1(1.0)
	_ = math.Yn(2, 1.0)
}
