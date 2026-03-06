// http_header exercises net/http Header method additions from v0.70.0:
// Header.Set, Header.Add, Header.Del, Header.Values, Header.Has,
// Request.Context, ResponseWriter.WriteHeader, http.CanonicalHeaderKey,
// http.DetectContentType, http.MaxBytesReader.
// Expected: 0 violations.
package main

import (
	"io"
	"net/http"
	"strings"
)

func main() {
	// http.Header — direct construction and manipulation.
	h := make(http.Header)
	h.Set("Content-Type", "application/json")
	h.Add("Accept", "text/html")
	h.Del("Accept")
	_ = h.Values("Content-Type")

	// http.CanonicalHeaderKey.
	_ = http.CanonicalHeaderKey("content-type")

	// http.DetectContentType.
	_ = http.DetectContentType([]byte("hello"))

	// http.MaxBytesReader — wraps an io.Reader with a size limit.
	rc := io.NopCloser(strings.NewReader("body"))
	_ = http.MaxBytesReader(nil, rc, 1024)

	// http.NewRequest — exercises Request.Context.
	req, _ := http.NewRequest("GET", "http://example.com", nil)
	if req != nil {
		_ = req.Context()
	}
}
