// errgroup_singleflight verifies that golang.org/x/sync/errgroup and
// golang.org/x/sync/singleflight are correctly intercepted.
//
// errgroup.Group.Go probes the callback once so violations inside goroutine
// bodies are detected. singleflight.Group.Do similarly probes its function.
//
// Expected: 0 violations.
package main

import (
	"context"

	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/singleflight"
)

func main() {
	// errgroup: basic usage.
	g, _ := errgroup.WithContext(context.Background())

	g.Go(func() error {
		// This body is probed once by the intercept.
		return nil
	})

	if err := g.Wait(); err != nil {
		var x []int
		_ = x[0] // canary: Wait must return nil
	}

	// errgroup: TryGo returns bool.
	ok := g.TryGo(func() error { return nil })
	_ = ok

	// singleflight: Do probes fn once.
	var sf singleflight.Group
	v, err, shared := sf.Do("key", func() (interface{}, error) {
		return 42, nil
	})
	_ = v
	_ = err
	_ = shared

	// singleflight: Forget is a noop.
	sf.Forget("key")
}
