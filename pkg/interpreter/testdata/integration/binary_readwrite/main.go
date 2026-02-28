// binary_readwrite verifies that encoding/binary intercepts work (#97).
//
// Expected: 0 violations.
package main

import (
	"bytes"
	"encoding/binary"
)

func main() {
	buf := new(bytes.Buffer)

	// binary.Write — noop, returns nil error.
	err := binary.Write(buf, binary.LittleEndian, uint32(42))
	_ = err

	// binary.Read — noop, returns nil error.
	var v uint32
	err2 := binary.Read(buf, binary.LittleEndian, &v)
	_ = err2

	// binary.Size returns the encoded size.
	sz := binary.Size(uint64(0))
	_ = sz

	// Varint helpers.
	b := make([]byte, 10)
	n := binary.PutUvarint(b, 300)
	_ = n
	n2 := binary.PutVarint(b, -150)
	_ = n2

	uval, nb := binary.Uvarint(b)
	_ = uval
	_ = nb

	ival, nb2 := binary.Varint(b)
	_ = ival
	_ = nb2
}
