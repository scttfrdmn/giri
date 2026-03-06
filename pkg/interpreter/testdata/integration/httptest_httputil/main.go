// httptest_httputil verifies that net/http/httptest and net/http/httputil are
// correctly intercepted.
//
// Expected: 0 violations.
package main

import (
	"net/http/httptest"
	"net/http/httputil"
)

func main() {
	// httptest: NewRecorder.
	rec := httptest.NewRecorder()
	_ = rec

	// httptest: NewServer (no actual server started — intercepted).
	srv := httptest.NewServer(nil)
	_ = srv

	// httputil: NewSingleHostReverseProxy.
	rp := httputil.NewSingleHostReverseProxy(nil)
	_ = rp

	// httputil: DumpRequest returns ([]byte, error).
	data, err := httputil.DumpRequest(nil, false)
	_ = data
	_ = err

	// httputil: DumpResponse returns ([]byte, error).
	data2, err2 := httputil.DumpResponse(nil, false)
	_ = data2
	_ = err2
}
