// dns_message exercises x/net/dns/dnsmessage intercepts (issue #207).
// Expected: 0 violations.
package main

import (
	"golang.org/x/net/dns/dnsmessage"
)

func main() {
	// MustNewName
	n := dnsmessage.MustNewName("example.com.")
	_ = n

	// NewName
	n2, err := dnsmessage.NewName("golang.org.")
	_, _ = n2, err

	// Name.String
	s := n.String()
	_ = s

	// Build a message
	msg := dnsmessage.Message{
		Header: dnsmessage.Header{
			ID:               1,
			RecursionDesired: true,
		},
		Questions: []dnsmessage.Question{
			{
				Name:  n,
				Type:  dnsmessage.TypeA,
				Class: dnsmessage.ClassINET,
			},
		},
	}

	// Pack message
	packed, err2 := msg.Pack()
	_, _ = packed, err2

	// AppendPack
	buf := make([]byte, 0, 512)
	buf2, err3 := msg.AppendPack(buf)
	_, _ = buf2, err3

	// Unpack via Parser
	var p dnsmessage.Parser
	hdr, err4 := p.Start(packed)
	_, _ = hdr, err4

	q, err5 := p.Question()
	_, _ = q, err5

	err6 := p.SkipAllQuestions()
	_ = err6

	// AllQuestions
	var p2 dnsmessage.Parser
	_, _ = p2.Start(packed)
	qs, err7 := p2.AllQuestions()
	_, _ = qs, err7
}
