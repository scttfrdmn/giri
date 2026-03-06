// os_complete exercises os package functions added in v0.69.0:
// Lstat, TempDir, Hostname, Getpid/Getuid/Getgid/Geteuid/Getegid, Getgroups,
// IsNotExist/IsExist/IsPermission/IsTimeout, ExpandEnv, Environ, Clearenv,
// Executable, UserHomeDir/UserCacheDir/UserConfigDir, Pipe, Readlink,
// Link, Symlink, SameFile, Chtimes.
// Expected: 0 violations.
package main

import (
	"errors"
	"os"
)

func main() {
	// Lstat — like Stat but for symlinks.
	_, _ = os.Lstat("/tmp")

	// TempDir — OS temporary directory.
	_ = os.TempDir()

	// Process identity.
	_ = os.Getpid()
	_ = os.Getuid()
	_ = os.Getgid()
	_ = os.Geteuid()
	_ = os.Getegid()
	_, _ = os.Getgroups()

	// Hostname.
	_, _ = os.Hostname()

	// Error predicates.
	err := errors.New("test error")
	_ = os.IsNotExist(err)
	_ = os.IsExist(err)
	_ = os.IsPermission(err)
	_ = os.IsTimeout(err)

	// Environment.
	_ = os.ExpandEnv("$HOME/bin")
	_ = os.Environ()
	os.Clearenv()

	// Executable path.
	_, _ = os.Executable()

	// User directories.
	_, _ = os.UserHomeDir()
	_, _ = os.UserCacheDir()
	_, _ = os.UserConfigDir()

	// Pipe — returns two *os.File values.
	r, w, _ := os.Pipe()
	_ = r
	_ = w

	// Symlink operations.
	_, _ = os.Readlink("/tmp/link")
	_ = os.Link("/tmp/a", "/tmp/b")
	_ = os.Symlink("/tmp/a", "/tmp/c")

	// SameFile requires two FileInfo values.
	fi1, _ := os.Stat("/tmp")
	fi2, _ := os.Stat("/tmp")
	_ = os.SameFile(fi1, fi2)
}
