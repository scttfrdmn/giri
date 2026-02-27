// context_basic verifies that context package intercepts work cleanly (#76).
//
// Expected: 0 violations.
package main

import "context"

func doWork(ctx context.Context) {
	_ = ctx
}

func main() {
	// Background and TODO return non-nil contexts.
	bg := context.Background()
	_ = bg

	todo := context.TODO()
	_ = todo

	// WithCancel returns (Context, CancelFunc).
	ctx, cancel := context.WithCancel(bg)
	defer cancel()
	_ = ctx

	// WithValue returns a derived context.
	type key struct{}
	vctx := context.WithValue(ctx, key{}, "hello")
	_ = vctx

	// Pass context to a function.
	doWork(vctx)

	// WithTimeout returns (Context, CancelFunc).
	tctx, tcancel := context.WithTimeout(bg, 0)
	defer tcancel()
	_ = tctx
}
