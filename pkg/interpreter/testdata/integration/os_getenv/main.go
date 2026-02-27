// os_getenv verifies that os.Getenv and os.LookupEnv are intercepted cleanly (#62).
//
// Programs that read environment variables should not cause interpreter crashes
// or spurious violations. The intercept returns "" / ("", false) respectively.
//
// Expected: 0 violations.
package main

import "os"

func main() {
	home := os.Getenv("HOME")
	path := os.Getenv("PATH")
	val, ok := os.LookupEnv("GIRI_TEST")
	_ = home
	_ = path
	_ = val
	_ = ok
}
