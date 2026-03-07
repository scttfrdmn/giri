// regexp_http_time_utf8_complete exercises additions from v0.85.0:
// regexp: FindStringSubmatchIndex, FindAllStringSubmatchIndex, ReplaceAllFunc;
// net/http: (*Request).Cookie/BasicAuth/UserAgent/Referer,
//           (*Response).Location, ParseCookie/ParseSetCookie (Go 1.23+);
// time: GobEncode, GobDecode;
// unicode/utf8: RuneStart.
// Expected: 0 violations.
package main

import (
	"net/http"
	"regexp"
	"time"
	"unicode/utf8"
)

func main() {
	// regexp: submatch-index variants.
	re := regexp.MustCompile(`(\w+)`)
	idx := re.FindStringSubmatchIndex("hello world")
	_ = idx
	allIdx := re.FindAllStringSubmatchIndex("hello world", -1)
	_ = allIdx

	// regexp: ReplaceAllFunc (byte-slice + callback).
	result := re.ReplaceAllFunc([]byte("hello"), func(b []byte) []byte {
		return b
	})
	_ = result

	// net/http: (*Request) accessor methods.
	req, _ := http.NewRequest("GET", "http://example.com", nil)
	cookie, _ := req.Cookie("session")
	_ = cookie
	user, pass, ok := req.BasicAuth()
	_, _, _ = user, pass, ok
	_ = req.UserAgent()
	_ = req.Referer()

	// net/http: ParseCookie / ParseSetCookie (Go 1.23+).
	cookies, _ := http.ParseCookie("session=abc; token=xyz")
	_ = cookies
	sc, _ := http.ParseSetCookie("session=abc; Path=/; HttpOnly")
	_ = sc

	// net/http: (*Response).Location.
	resp := &http.Response{Header: http.Header{}}
	loc, _ := resp.Location()
	_ = loc

	// time: GobEncode and GobDecode.
	t := time.Now()
	enc, _ := t.GobEncode()
	_ = enc
	var t2 time.Time
	_ = t2.GobDecode(enc)

	// unicode/utf8: RuneStart.
	_ = utf8.RuneStart(0x41) // 'A'
	_ = utf8.RuneStart(0x80) // continuation byte
}
