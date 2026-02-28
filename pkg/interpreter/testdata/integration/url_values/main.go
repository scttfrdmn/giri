// url_values verifies that net/url.Values and url.ParseQuery intercepts work (#89).
//
// Expected: 0 violations.
package main

import "net/url"

func main() {
	// url.ParseQuery returns a Values map.
	vals, err := url.ParseQuery("name=Alice&age=30&name=Bob")
	_ = err // nil

	// Get returns the first value for a key.
	name := vals.Get("name")
	_ = name // "Alice"

	// Has checks for key existence.
	_ = vals.Has("age") // true

	// Encode serialises the values in key=value format.
	encoded := vals.Encode()
	_ = encoded

	// Set/Add/Del mutate the Values map.
	vals.Set("name", "Carol")
	vals.Add("role", "admin")
	vals.Del("age")

	// url.User and url.UserPassword return a *Userinfo.
	ui := url.User("alice")
	_ = ui

	ui2 := url.UserPassword("alice", "s3cr3t")
	_ = ui2

	// Construct a URL with query string manually.
	u := &url.URL{
		Scheme:   "https",
		Host:     "api.example.com",
		Path:     "/v1/users",
		RawQuery: url.Values{"page": {"1"}}.Encode(),
	}
	_ = u.String()
}
