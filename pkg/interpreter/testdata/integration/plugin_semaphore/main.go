// plugin_semaphore verifies that plugin and golang.org/x/sync/semaphore are
// correctly intercepted.
//
// Expected: 0 violations.
package main

import (
	"context"

	"golang.org/x/sync/semaphore"
)

func main() {
	// golang.org/x/sync/semaphore: NewWeighted returns *Weighted.
	sem := semaphore.NewWeighted(10)
	if sem == nil {
		// Should not reach here — intercept returns opaque non-nil.
		return
	}

	// semaphore: Acquire returns error (nil in our model).
	ctx := context.Background()
	err := sem.Acquire(ctx, 1)
	_ = err

	// semaphore: TryAcquire returns bool.
	ok := sem.TryAcquire(1)
	if !ok {
		// Intercept returns true — should not reach here.
		var s []int
		_ = s[0] // canary: TryAcquire must return true
	}

	// semaphore: Release.
	sem.Release(1)
}
