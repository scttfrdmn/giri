// double_close_file verifies that closing the same os.File twice is detected
// when the double-close detector is enabled (#223).
//
// A second Close returns os.ErrClosed in Go — defined behavior, not a panic —
// but is a reliable bug smell. Expected: 1 violation (double-close).
package main

import "os"

func main() {
	f, err := os.Open("/etc/hostname")
	if err != nil {
		return
	}
	f.Close()
	f.Close() // second close of the same handle
}
