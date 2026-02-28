// context_cancel_leak verifies that a cancel function that is never called
// triggers a context-cancel-leak violation.
// Expected: 1 violation (context-cancel-leak).
package main

import "context"

func doWork(ctx context.Context) {
	_ = ctx
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	// Intentionally NOT calling cancel to trigger the leak detection.
	_ = cancel
	doWork(ctx)
}
