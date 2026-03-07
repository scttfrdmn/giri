// This program compiles cleanly, passes go vet, and passes go test -race.
// Giri detects the context cancel function that is never called.
package main

import "context"

// processRequest creates a child context for a request but forgets to
// call the cancel function before returning. In production, this leaks
// resources associated with the context (timers, goroutines waiting on
// ctx.Done(), child context chains) for the lifetime of the parent context.
//
// go vet cannot detect this: it does not track cancel function liveness.
// go test -race passes: the single-goroutine access pattern has no races.
// Giri detects it: context.WithCancel registers the cancel func; at program
// exit, any cancel func that was never called is reported as a leak.
func processRequest(parent context.Context) string {
	// BUG: requestCtx's cancel function is never called.
	// The context's internal resources are leaked until parent is cancelled.
	requestCtx, cancel := context.WithCancel(parent)
	_ = cancel // assigned but intentionally never invoked

	select {
	case <-requestCtx.Done():
		return "cancelled"
	default:
		return "ok"
	}
}

func main() {
	result := processRequest(context.Background())
	_ = result
}
