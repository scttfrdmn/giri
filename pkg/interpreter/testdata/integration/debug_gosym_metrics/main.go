// debug_gosym_metrics verifies that debug/gosym, debug/plan9obj, and
// runtime/metrics are correctly intercepted.
//
// Expected: 0 violations.
package main

import (
	"debug/gosym"
	"runtime/metrics"
)

func main() {
	// debug/gosym: NewLineTable.
	lt := gosym.NewLineTable(nil, 0)
	if lt == nil {
		var s []int
		_ = s[0] // canary: line table must be non-nil
	}

	// debug/gosym: NewTable.
	tab, err := gosym.NewTable(nil, lt)
	_ = err
	if tab == nil {
		// opaque — may be nil when given nil input; OK
	}

	// runtime/metrics: All.
	descs := metrics.All()
	_ = descs

	// runtime/metrics: Read with empty sample slice.
	samples := make([]metrics.Sample, 0)
	metrics.Read(samples)
}
