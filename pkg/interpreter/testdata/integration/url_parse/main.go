// url_parse verifies that net/url.Parse and URL method intercepts work (#89).
//
// Expected: 0 violations.
package main

import "net/url"

func main() {
	// url.Parse returns a non-nil *URL and nil error.
	u, err := url.Parse("https://example.com/path?key=value#frag")
	_ = err // nil

	// URL methods — field accesses go through SSA FieldAddr on an opaque value,
	// so we use method calls only.
	_ = u.Hostname()   // "example.com"
	_ = u.Port()       // "" (no port in URL above)
	_ = u.RequestURI() // "/path?key=value"
	_ = u.IsAbs()      // true
	_ = u.String()     // reconstructed URL

	// QueryEscape / QueryUnescape.
	escaped := url.QueryEscape("hello world")
	_ = escaped // "hello+world"
	unescaped, err2 := url.QueryUnescape(escaped)
	_ = unescaped // "hello world"
	_ = err2      // nil

	// PathEscape / PathUnescape.
	pe := url.PathEscape("hello/world")
	_ = pe
	pu, err3 := url.PathUnescape(pe)
	_ = pu
	_ = err3

	// url.ParseRequestURI (stricter parse).
	u2, err4 := url.ParseRequestURI("/path?q=1")
	_ = u2
	_ = err4
}
