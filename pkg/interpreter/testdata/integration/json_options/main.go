// json_options exercises encoding/json Decoder and Encoder option methods
// added in v0.70.0: UseNumber, DisallowUnknownFields, InputOffset, Buffered,
// SetIndent, SetEscapeHTML.
// Expected: 0 violations.
package main

import (
	"bytes"
	"encoding/json"
	"strings"
)

func main() {
	// Decoder options.
	dec := json.NewDecoder(strings.NewReader(`{"key":"value"}`))
	dec.UseNumber()
	dec.DisallowUnknownFields()
	_ = dec.InputOffset()
	_ = dec.Buffered()
	var v map[string]interface{}
	_ = dec.Decode(&v)

	// Encoder options.
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	_ = enc.Encode(map[string]string{"hello": "world"})
}
