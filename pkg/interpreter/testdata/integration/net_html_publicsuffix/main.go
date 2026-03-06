// net_html_publicsuffix verifies that golang.org/x/net/html,
// golang.org/x/net/publicsuffix, golang.org/x/net/idna, and
// golang.org/x/net/netutil calls are correctly intercepted.
//
// Expected: 0 violations.
package main

import (
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/idna"
	"golang.org/x/net/publicsuffix"
)

func main() {
	// html: Parse.
	r := strings.NewReader("<html><body>hello</body></html>")
	node, err := html.Parse(r)
	_ = node
	_ = err

	// html: EscapeString / UnescapeString.
	esc := html.EscapeString("<div>")
	_ = esc
	unesc := html.UnescapeString("&lt;div&gt;")
	_ = unesc

	// html: NewTokenizer.
	r2 := strings.NewReader("<p>test</p>")
	tok := html.NewTokenizer(r2)
	_ = tok

	// publicsuffix: PublicSuffix.
	suffix, icann := publicsuffix.PublicSuffix("www.example.com")
	_ = suffix
	_ = icann

	// publicsuffix: EffectiveTLDPlusOne.
	etld, err2 := publicsuffix.EffectiveTLDPlusOne("www.example.com")
	_ = etld
	_ = err2

	// idna: ToASCII.
	ascii, err3 := idna.ToASCII("example.com")
	_ = ascii
	_ = err3

	// idna: ToUnicode.
	uni, err4 := idna.ToUnicode("xn--n3h.ws")
	_ = uni
	_ = err4
}
