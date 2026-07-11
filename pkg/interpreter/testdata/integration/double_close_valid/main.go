// double_close_valid verifies that the double-close detector does NOT fire on
// correct usage even when enabled (#223): each of two distinct files is closed
// exactly once. This guards against a false positive where two handles are
// conflated (e.g. both aliasing the shared opaque sentinel).
//
// Expected: 0 violations.
package main

import "os"

func main() {
	f1, err := os.Open("/etc/hostname")
	if err != nil {
		return
	}
	f2, err := os.Open("/etc/hosts")
	if err != nil {
		f1.Close()
		return
	}
	f1.Close() // distinct handle
	f2.Close() // distinct handle — not a double close
}
