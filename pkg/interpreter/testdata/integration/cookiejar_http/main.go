// cookiejar_http verifies that net/http/cookiejar is correctly intercepted.
//
// Expected: 0 violations.
package main

import (
	"net/http/cookiejar"
)

func main() {
	// cookiejar: New.
	jar, err := cookiejar.New(nil)
	_ = err
	if jar == nil {
		var s []int
		_ = s[0] // canary: jar must be non-nil
	}

	// cookiejar: Cookies (returns []*http.Cookie).
	cookies := jar.Cookies(nil)
	_ = cookies
}
