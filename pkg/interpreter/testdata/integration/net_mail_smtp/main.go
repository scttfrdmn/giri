// net_mail_smtp verifies that net/mail, net/textproto, and net/smtp are
// correctly intercepted.
//
// Expected: 0 violations.
package main

import (
	"net/mail"
	"net/textproto"
)

func main() {
	// net/mail: ParseAddress.
	addr, err := mail.ParseAddress("Alice <alice@example.com>")
	_ = addr
	_ = err

	// net/mail: ParseAddressList.
	list, err2 := mail.ParseAddressList("a@x.com, b@y.com")
	if len(list) < 0 {
		var s []int
		_ = s[0] // canary: list length must be >= 0
	}
	_ = err2

	// net/textproto: CanonicalMIMEHeaderKey.
	key := textproto.CanonicalMIMEHeaderKey("content-type")
	if len(key) == 0 {
		var s []int
		_ = s[0] // canary: key must be non-empty
	}

	// net/textproto: TrimString.
	trimmed := textproto.TrimString("  hello  ")
	_ = trimmed
}
