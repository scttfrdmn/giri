// syscall_testing verifies that syscall, testing/iotest, and testing/fstest
// are correctly intercepted.
//
// Expected: 0 violations.
package main

import (
	"syscall"
	"testing/iotest"
	"strings"
)

func main() {
	// syscall: Getpid — returns a valid PID.
	pid := syscall.Getpid()
	if pid <= 0 {
		var s []int
		_ = s[0] // canary: PID must be > 0
	}

	// syscall: Getuid — returns a valid UID (>= 0).
	uid := syscall.Getuid()
	if uid < 0 {
		var s []int
		_ = s[0] // canary: UID must be >= 0
	}

	// syscall: Getenv — returns (string, bool).
	val, found := syscall.Getenv("PATH")
	_ = val
	_ = found

	// syscall: Getpagesize.
	ps := syscall.Getpagesize()
	if ps <= 0 {
		var s []int
		_ = s[0] // canary: page size must be > 0
	}

	// testing/iotest: ErrReader — returns an opaque reader.
	r := iotest.ErrReader(nil)
	_ = r

	// testing/iotest: OneByteReader.
	src := strings.NewReader("hello")
	obr := iotest.OneByteReader(src)
	_ = obr

	// testing/iotest: NewReadLogger.
	rl := iotest.NewReadLogger("read", src)
	_ = rl
}
