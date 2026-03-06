// unmodeled_demo exercises a math function that exists in Giri's "math" package
// intercept group but whose specific name is absent from handleMathCall's switch.
// math.Erfinv is NOT in the switch, so Giri falls through to execFunction and
// records it as an unmodeled cross-package call.
//
// Used by TestUnmodeledCallsReport to verify RunResult.UnmodeledCalls is populated.
// Expected: 0 violations, UnmodeledCalls contains "math.Erfinv".
package main

import "math"

func main() {
	// math.Erfinv is NOT handled in Giri's handleMathCall switch.
	// Giri will execute it via pure SSA interpretation (execFunction) and
	// record "math.Erfinv" in UnmodeledCalls because it crosses a package boundary
	// (main → math) without a dedicated model.
	_ = math.Erfinv(0.5)
}
