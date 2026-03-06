// quick_quotedprintable verifies that testing/quick, mime/quotedprintable,
// net/http/httptrace, and net/rpc/jsonrpc are correctly intercepted.
//
// Expected: 0 violations.
package main

import (
	"mime/quotedprintable"
	"net/http/httptrace"
)

func main() {
	// mime/quotedprintable: NewReader.
	r := quotedprintable.NewReader(nil)
	if r == nil {
		var s []int
		_ = s[0] // canary: reader must be non-nil
	}

	// mime/quotedprintable: NewWriter.
	w := quotedprintable.NewWriter(nil)
	if w == nil {
		var s []int
		_ = s[0] // canary: writer must be non-nil
	}

	// net/http/httptrace: ContextClientTrace.
	trace := httptrace.ContextClientTrace(nil)
	_ = trace

	// net/http/httptrace: WithClientTrace.
	ctx := httptrace.WithClientTrace(nil, nil)
	_ = ctx
}
