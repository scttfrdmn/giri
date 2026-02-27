// fmt_print_return verifies that fmt.Printf/Println/Fprintf return (n int, err error)
// and callers that check the return values take correct branches (#65).
//
// Without the fix, fmt.Println returns Value{} causing n=0 and err=nil.
// A check like `if n == 0 { panic(...) }` would incorrectly panic.
//
// Expected: 0 violations.
package main

import (
	"fmt"
	"os"
)

func main() {
	n, err := fmt.Println("hello")
	if err != nil {
		// Should not be reached: err is nil.
		panic("unexpected error from Println")
	}
	_ = n

	m, err2 := fmt.Fprintf(os.Stderr, "msg")
	if err2 != nil {
		panic("unexpected error from Fprintf")
	}
	_ = m
}
