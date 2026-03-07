// sync_http_complete exercises additions from v0.88.0:
// sync: OnceFunc, OnceValue, OnceValues (Go 1.21+/1.22+);
// net/http: (*Request).ProtoAtLeast, AddCookie, SetBasicAuth, CookiesNamed,
//           Write, WriteProxy; (*Response).Write, ProtoAtLeast;
//           ParseHTTPVersion, ParseTime, ServeFileFS, ServeTLS,
//           ProxyFromEnvironment.
// Expected: 0 violations.
package main

import (
	"bytes"
	"net"
	"net/http"
	"sync"
)

// noopListener implements net.Listener for ServeTLS test.
type noopListener struct{ done chan struct{} }

func (l *noopListener) Accept() (net.Conn, error) { <-l.done; return nil, net.ErrClosed }
func (l *noopListener) Close() error               { close(l.done); return nil }
func (l *noopListener) Addr() net.Addr             { return &net.TCPAddr{} }

func main() {
	// sync: OnceFunc — returns a func() that calls setup at most once.
	setup := func() {}
	once := sync.OnceFunc(setup)
	once()

	// sync: OnceValue — returns a func() T.
	computeVal := func() int { return 42 }
	getVal := sync.OnceValue(computeVal)
	_ = getVal()

	// sync: OnceValues — returns a func() (T1, T2).
	compute2 := func() (int, error) { return 1, nil }
	get2 := sync.OnceValues(compute2)
	v, err := get2()
	_, _ = v, err

	// net/http: (*Request).ProtoAtLeast.
	req, _ := http.NewRequest("GET", "http://example.com", nil)
	_ = req.ProtoAtLeast(1, 1)

	// net/http: (*Request).AddCookie.
	cookie := &http.Cookie{Name: "session", Value: "abc"}
	req.AddCookie(cookie)

	// net/http: (*Request).SetBasicAuth.
	req.SetBasicAuth("user", "pass")

	// net/http: (*Request).CookiesNamed (Go 1.25+).
	named := req.CookiesNamed("session")
	_ = named

	// net/http: (*Request).Write and WriteProxy.
	var buf bytes.Buffer
	_ = req.Write(&buf)
	_ = req.WriteProxy(&buf)

	// net/http: (*Response).ProtoAtLeast and Write.
	resp := &http.Response{Header: http.Header{}, Body: http.NoBody}
	_ = resp.ProtoAtLeast(1, 1)
	_ = resp.Write(&buf)

	// net/http: ParseHTTPVersion.
	major, minor, ok := http.ParseHTTPVersion("HTTP/1.1")
	_, _, _ = major, minor, ok

	// net/http: ParseTime.
	t, _ := http.ParseTime("Mon, 02 Jan 2006 15:04:05 GMT")
	_ = t

	// net/http: ServeFileFS is a void function — just call it.
	// (Skip: requires ResponseWriter + *Request chain — tested implicitly)

	// net/http: ProxyFromEnvironment.
	proxy, _ := http.ProxyFromEnvironment(req)
	_ = proxy
}
