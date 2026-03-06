// sys_unix verifies that golang.org/x/sys/unix calls are correctly intercepted.
//
// Expected: 0 violations.
//go:build !windows

package main

import (
	"golang.org/x/sys/unix"
)

func main() {
	// Process identity.
	pid := unix.Getpid()
	_ = pid
	uid := unix.Getuid()
	_ = uid

	// Working directory.
	wd, err := unix.Getwd()
	_ = wd
	_ = err

	// File operations.
	fd, err2 := unix.Open("/dev/null", unix.O_RDONLY, 0)
	_ = err2
	if fd >= 0 {
		var buf [8]byte
		n, _ := unix.Read(fd, buf[:])
		_ = n
		_ = unix.Close(fd)
	}

	// Byte helpers.
	sl, _ := unix.ByteSliceFromString("hello")
	_ = sl
	_ = unix.ByteSliceToString([]byte{'h', 'i'})

	// Stat.
	var st unix.Stat_t
	_ = unix.Stat("/tmp", &st)
}
