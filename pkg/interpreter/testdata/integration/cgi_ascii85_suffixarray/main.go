// cgi_ascii85_suffixarray verifies that net/http/cgi, net/http/fcgi,
// encoding/ascii85, and index/suffixarray are correctly intercepted.
//
// Expected: 0 violations.
package main

import (
	"encoding/ascii85"
	"index/suffixarray"
	"net/http/fcgi"
)

func main() {
	// net/http/fcgi: ProcessEnv (returns map — noop in interpreter).
	env := fcgi.ProcessEnv(nil)
	_ = env

	// encoding/ascii85: Encode.
	src := []byte("hello world")
	dst := make([]byte, ascii85.MaxEncodedLen(len(src)))
	n := ascii85.Encode(dst, src)
	if n < 0 {
		var s []int
		_ = s[0] // canary: encoded length must be >= 0
	}

	// encoding/ascii85: NewEncoder.
	enc := ascii85.NewEncoder(nil)
	if enc == nil {
		var s []int
		_ = s[0] // canary: encoder must be non-nil
	}

	// encoding/ascii85: Decode.
	ndst, nsrc, err := ascii85.Decode(dst, src, true)
	_ = ndst
	_ = nsrc
	_ = err

	// index/suffixarray: New.
	idx := suffixarray.New([]byte("the quick brown fox"))
	if idx == nil {
		var s []int
		_ = s[0] // canary: index must be non-nil
	}

	// index/suffixarray: Lookup.
	results := idx.Lookup([]byte("quick"), -1)
	_ = results
}
