// encoding_gob verifies that encoding/gob and encoding/base32 are correctly
// intercepted.
//
// Expected: 0 violations.
package main

import (
	"bytes"
	"encoding/base32"
	"encoding/gob"
)

func main() {
	// encoding/gob: encode and decode.
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	_ = enc.Encode(42)

	dec := gob.NewDecoder(&buf)
	var v int
	_ = dec.Decode(&v)

	// encoding/base32: EncodeToString.
	data := []byte("hello giri")
	encoded := base32.StdEncoding.EncodeToString(data)
	if len(encoded) == 0 {
		var x []int
		_ = x[0] // canary: encoded must be non-empty
	}
}
