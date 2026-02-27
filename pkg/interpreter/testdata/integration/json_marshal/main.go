// json_marshal verifies that encoding/json intercepts work cleanly (#70).
//
// Expected: 0 violations.
package main

import "encoding/json"

type Point struct {
	X, Y int
}

func main() {
	// Marshal: returns ([]byte, error).
	p := Point{X: 1, Y: 2}
	b, err := json.Marshal(p)
	if err != nil {
		return
	}
	_ = b

	// MarshalIndent
	b2, err2 := json.MarshalIndent(p, "", "  ")
	if err2 != nil {
		return
	}
	_ = b2

	// Unmarshal: returns error.
	var p2 Point
	err3 := json.Unmarshal(b, &p2)
	if err3 != nil {
		return
	}
	_ = p2

	// Valid
	ok := json.Valid(b)
	_ = ok

	// NewDecoder / Decode (opaque reader — intercept fires, returns noop).
	dec := json.NewDecoder(nil)
	_ = dec

	// NewEncoder
	enc := json.NewEncoder(nil)
	_ = enc
}
