// math_big verifies that math/big intercepts work (#100).
//
// Expected: 0 violations.
package main

import "math/big"

func main() {
	// *big.Int operations.
	a := big.NewInt(100)
	b := big.NewInt(37)
	c := new(big.Int).Add(a, b)
	_ = c
	d := new(big.Int).Mul(a, b)
	_ = d
	e := new(big.Int).Sub(a, b)
	_ = e

	_ = a.Int64()
	_ = a.Sign()
	_ = a.BitLen()
	_ = a.String()
	_ = a.Cmp(b)

	// *big.Float operations.
	f := big.NewFloat(3.14)
	g := big.NewFloat(2.71)
	h := new(big.Float).Add(f, g)
	_ = h
	fv, _ := f.Float64()
	_ = fv
	_ = f.Sign()

	// *big.Rat operations.
	r := big.NewRat(1, 3)
	s := big.NewRat(2, 5)
	t := new(big.Rat).Add(r, s)
	_ = t
	_ = r.RatString()
	_ = r.Sign()
}
