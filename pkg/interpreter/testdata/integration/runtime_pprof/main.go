// runtime_pprof verifies that runtime/pprof and runtime/trace functions are
// correctly intercepted.
//
// Expected: 0 violations.
package main

import (
	"os"
	"runtime/pprof"
	"runtime/trace"
)

func main() {
	// runtime/pprof: Lookup returns a non-nil *Profile.
	p := pprof.Lookup("heap")
	_ = p

	// runtime/pprof: StartCPUProfile / StopCPUProfile.
	err := pprof.StartCPUProfile(os.Stdout)
	_ = err
	pprof.StopCPUProfile()

	// runtime/pprof: WriteHeapProfile.
	_ = pprof.WriteHeapProfile(os.Stdout)

	// runtime/trace: Start / Stop.
	err2 := trace.Start(os.Stdout)
	_ = err2
	trace.Stop()

	// runtime/trace: IsEnabled.
	_ = trace.IsEnabled()
}
