// testing_helper verifies that testing.T method intercepts work (#104).
//
// Expected: 0 violations.
package main

import "testing"

func helperFunc(t *testing.T) {
	t.Helper()
	t.Log("helper called")
}

func main() {
	t := new(testing.T)

	// Logging — noops.
	t.Log("hello")
	t.Logf("value: %d", 42)

	// Error — noop (does not stop execution).
	t.Error("something went wrong")
	t.Errorf("code %d", 1)

	// Helper.
	t.Helper()

	// State queries.
	_ = t.Failed()
	_ = t.Skipped()
	_ = t.Name()

	// TempDir.
	dir := t.TempDir()
	_ = dir

	// Run probes the subtest function.
	t.Run("sub", func(sub *testing.T) {
		sub.Log("inside subtest")
	})

	// Helper via function call.
	helperFunc(t)
}
