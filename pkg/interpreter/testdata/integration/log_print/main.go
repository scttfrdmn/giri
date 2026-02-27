// log_print verifies that log package Print/Println/Printf intercepts work (#80).
//
// Expected: 0 violations.
package main

import "log"

func main() {
	// Print functions are noops in the interpreter.
	log.Print("hello")
	log.Println("world")
	log.Printf("value: %d", 42)

	// log.New returns an opaque logger.
	logger := log.New(nil, "prefix: ", 0)
	_ = logger

	// Default returns the default logger.
	_ = log.Default()
}
