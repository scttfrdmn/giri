// Package testmode contains a simple library for test_mode integration testing.
// It exercises the giri -test flag: TestSafeAdd expects 0 violations,
// TestCounterRace expects 1 data-race violation.
package testmode

// Counter is a shared package-level variable used by TestCounterRace.
var Counter int

// SafeAdd returns a + b without side effects.
func SafeAdd(a, b int) int { return a + b }
