// Showcase: nil pointer dereference on an untested code path
//
// A port lookup returns nil when a scheme is absent; the caller dereferences
// without a nil check. In a test suite that only exercises known schemes,
// this bug is completely invisible until an unknown scheme is encountered.
//
// Giri executes all code paths including getPort("ftp") and detects the nil
// dereference statically without requiring that specific path to be covered
// by a test.
//
// What each tool reports:
//
//	go vet:              PASS — vet does not perform nil-reachability analysis
//	go test -race:       PASS — if tests only call getPort("http") or "https"
//	giri:                FAIL — nil pointer dereference
//
// The program would panic at runtime if "ftp" is passed, but Giri catches
// it without needing to reach that execution path during testing.
package main

func intPtr(n int) *int { return &n }

// getPort returns the default TCP port for a well-known scheme.
// Bug: returns nil for unknown schemes; the caller dereferences unconditionally.
func getPort(scheme string) int {
	ports := map[string]*int{
		"http":  intPtr(80),
		"https": intPtr(443),
	}
	return *ports[scheme] // nil dereference if scheme is not in the map
}

func main() {
	// These are the paths covered by tests — all pass.
	_ = getPort("http")
	_ = getPort("https")

	// This path is NOT covered by tests. The pointer is nil.
	_ = getPort("ftp") // nil dereference: "ftp" is absent from the map
}
