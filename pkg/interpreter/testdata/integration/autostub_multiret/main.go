// autostub_multiret exercises the signature-aware smart fallback (#225).
//
// math/big.ParseFloat has no explicit Giri intercept, so it hits the fallback.
// It returns (*big.Float, int, error) — a three-value tuple. The auto-generated
// stub shapes the return from the callee's result signature as
// []Value{opaque, int64(0), nil}, so the SSA Extract at each result slot unpacks
// correctly: slot 0 is a non-nil opaque *Float, slot 1 is int(0), slot 2 is a
// nil error.
//
// Before #225 the fallback returned a single opaque scalar regardless of arity,
// so the tuple unpack (tuple.Raw.([]Value)) failed and every slot came back as a
// nil Value — reintroducing the very nil-value hazards #222 set out to avoid for
// the multi-return case.
//
// Expected: 0 violations; UnmodeledCalls contains "math/big.ParseFloat".
package main

import "math/big"

func main() {
	f, prec, err := big.ParseFloat("3.14", 10, 53, big.ToNearestEven)
	if err != nil {
		return
	}
	// slot 1 (int) participates in arithmetic.
	total := prec + 1
	println(total)
	// slot 0 (*Float) — method dispatch on the opaque value.
	_ = f.String()
}
