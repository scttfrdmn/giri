// context_cancel_ok verifies that cancel functions called via defer produce
// no violations. This is the canonical correct usage pattern.
// Expected: 0 violations.
package main

import "context"

func doWork(ctx context.Context) {
	// Simulate using the context.
	_ = ctx
}

func withCancel() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	doWork(ctx)
}

func withTimeout() {
	ctx, cancel := context.WithTimeout(context.Background(), 0)
	defer cancel()
	doWork(ctx)
}

func directCall() {
	ctx, cancel := context.WithCancel(context.Background())
	doWork(ctx)
	cancel() // Called directly (not via defer)
}

func main() {
	withCancel()
	withTimeout()
	directCall()
}
