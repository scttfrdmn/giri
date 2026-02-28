// signal_notify verifies that os/signal intercepts work (#96).
//
// signal.Notify pre-populates the channel so no goroutine-leak violation
// fires when a goroutine waits on it.
//
// Expected: 0 violations.
package main

import (
	"os"
	"os/signal"
)

func main() {
	ch := make(chan os.Signal, 1)

	// Notify registers the channel; intercept pre-populates it.
	signal.Notify(ch, os.Interrupt)

	// Stop unregisters; noop in the interpreter.
	signal.Stop(ch)

	// Ignore and Reset are noops.
	signal.Ignore(os.Interrupt)
	signal.Reset(os.Interrupt)
}
