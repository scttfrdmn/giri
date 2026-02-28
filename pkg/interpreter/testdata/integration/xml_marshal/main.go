// xml_marshal verifies that encoding/xml intercepts work (#87).
//
// Expected: 0 violations.
package main

import (
	"encoding/xml"
)

type Person struct {
	XMLName xml.Name `xml:"person"`
	Name    string   `xml:"name"`
	Age     int      `xml:"age"`
}

func main() {
	// xml.Marshal encodes a struct to XML bytes.
	p := Person{Name: "Alice", Age: 30}
	data, err := xml.Marshal(p)
	_ = data // <person><name>Alice</name><age>30</age></person>
	_ = err  // nil

	// xml.MarshalIndent adds indentation.
	data2, err2 := xml.MarshalIndent(p, "", "  ")
	_ = data2
	_ = err2

	// xml.Unmarshal decodes XML bytes into a struct.
	var p2 Person
	err3 := xml.Unmarshal(data, &p2)
	_ = err3 // nil

	// xml.NewDecoder wraps a reader.
	// (just verify it returns a non-nil decoder)
	dec := xml.NewDecoder(nil)
	_ = dec

	// xml.NewEncoder wraps a writer.
	enc := xml.NewEncoder(nil)
	_ = enc
}
