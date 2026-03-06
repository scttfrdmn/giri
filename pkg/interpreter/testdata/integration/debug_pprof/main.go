// debug_pprof verifies that runtime/pprof and net/http/pprof are correctly
// intercepted.
//
// Expected: 0 violations.
package main

import (
	"runtime/pprof"
)

func main() {
	// runtime/pprof: Lookup returns a non-nil *Profile.
	p := pprof.Lookup("goroutine")
	if p == nil {
		// Intercept returns opaque non-nil — should not reach here.
		return
	}

	// runtime/pprof: *Profile.Name returns string.
	name := p.Name()
	if len(name) == 0 {
		var s []int
		_ = s[0] // canary: name must be non-empty
	}

	// runtime/pprof: *Profile.Count returns int.
	count := p.Count()
	if count < 0 {
		var s []int
		_ = s[0] // canary: count must be >= 0
	}

	// runtime/pprof: Profiles returns []*Profile.
	profiles := pprof.Profiles()
	_ = profiles

	// runtime/pprof: StartCPUProfile / StopCPUProfile.
	err := pprof.StartCPUProfile(nil)
	_ = err
	pprof.StopCPUProfile()
}
