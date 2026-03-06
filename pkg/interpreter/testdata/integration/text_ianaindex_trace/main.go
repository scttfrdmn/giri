// text_ianaindex_trace exercises x/text/encoding/ianaindex and x/net/trace
// intercepts (issue #206). Expected: 0 violations.
package main

import (
	"context"
	"net/http"

	"golang.org/x/net/trace"
	"golang.org/x/text/encoding/ianaindex"
)

func main() {
	// ianaindex: IANA.Encoding
	enc, err := ianaindex.IANA.Encoding("utf-8")
	_, _ = enc, err

	// ianaindex: MIME.Encoding
	enc2, err2 := ianaindex.MIME.Encoding("UTF-8")
	_, _ = enc2, err2

	// ianaindex: IANA.Name
	name, err3 := ianaindex.IANA.Name(enc)
	_, _ = name, err3

	// ianaindex: MIME.Name
	name2, err4 := ianaindex.MIME.Name(enc2)
	_, _ = name2, err4

	// trace: New
	tr := trace.New("test", "request")
	_ = tr

	// trace: NewContext
	ctx := trace.NewContext(context.Background(), tr)
	_ = ctx

	// trace: FromContext
	tr2, ok := trace.FromContext(ctx)
	_, _ = tr2, ok

	// trace: NewEventLog
	el := trace.NewEventLog("test", "event")
	_ = el

	// trace: HTTP handler (noop)
	var w http.ResponseWriter
	var r *http.Request
	trace.Traces(w, r)
}
