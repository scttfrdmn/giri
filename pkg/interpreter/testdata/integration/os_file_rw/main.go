// os_file_rw verifies that os.File method intercepts work (#94).
//
// Expected: 0 violations.
package main

import "os"

func main() {
	// os.Create returns (*File, nil).
	f, err := os.Create("/tmp/giri_test.txt")
	_ = err

	// Write returns (n, nil).
	n, err2 := f.Write([]byte("hello, giri"))
	_ = n
	_ = err2

	// WriteString returns (n, nil).
	n2, err3 := f.WriteString("world")
	_ = n2
	_ = err3

	// Seek returns (offset, nil).
	off, err4 := f.Seek(0, 0)
	_ = off
	_ = err4

	// Sync is a noop.
	_ = f.Sync()

	// Name returns a string.
	_ = f.Name()

	// Close returns nil.
	_ = f.Close()

	// os.Open returns (*File, nil).
	f2, err5 := os.Open("/tmp/giri_test.txt")
	_ = err5

	// Read returns (n, nil).
	buf := make([]byte, 64)
	n3, err6 := f2.Read(buf)
	_ = n3
	_ = err6

	// Close the reader.
	_ = f2.Close()

	// os.OpenFile with flags.
	f3, err7 := os.OpenFile("/tmp/giri_test.txt", os.O_RDWR, 0644)
	_ = f3
	_ = err7
}
