// custom_intercept demonstrates Giri's Config.Intercepts API (#113, #114).
//
// locallib.Compute and locallib.MustAlloc are intercepted by the test; the
// interpreter never executes their bodies. Expected: 0 violations.
package main

import (
	"github.com/scttfrdmn/giri/pkg/interpreter/testdata/integration/custom_intercept/locallib"
)

func main() {
	// locallib.Compute(1000) would run a tight loop without an intercept.
	// With Config.Intercepts registered, it returns the sentinel 0 instantly.
	result := locallib.Compute(1000)
	_ = result

	// locallib.MustAlloc(64) would allocate heap memory; the intercept returns
	// an opaque non-nil value so the assignment succeeds without UB.
	buf := locallib.MustAlloc(64)
	_ = buf
}
