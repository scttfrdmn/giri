// http_client verifies that net/http client intercepts work (#95).
//
// Field accesses on *http.Response go through SSA FieldAddr on an opaque
// value and are avoided; only function-call patterns are tested here.
//
// Expected: 0 violations.
package main

import "net/http"

func main() {
	// NewRequest returns (*Request, nil).
	req, err := http.NewRequest("GET", "http://example.com", nil)
	_ = req
	_ = err

	// http.Get returns (*Response, nil).
	resp, err2 := http.Get("http://example.com")
	_ = resp
	_ = err2

	// http.Post returns (*Response, nil).
	resp2, err3 := http.Post("http://example.com", "application/json", nil)
	_ = resp2
	_ = err3

	// http.Head returns (*Response, nil).
	resp3, err4 := http.Head("http://example.com")
	_ = resp3
	_ = err4

	// http.NewServeMux returns an opaque *ServeMux.
	mux := http.NewServeMux()
	_ = mux

	// http.StatusText returns a string.
	_ = http.StatusText(http.StatusOK)
}
