// hpack_syncmap exercises x/net/http2/hpack and x/sync/syncmap intercepts
// (issue #208). Expected: 0 violations.
package main

import (
	"bytes"

	"golang.org/x/net/http2/hpack"
	"golang.org/x/sync/syncmap"
)

func main() {
	// hpack: HuffmanEncodeLength
	n := hpack.HuffmanEncodeLength("hello")
	_ = n

	// hpack: HuffmanDecodeToString
	s, err := hpack.HuffmanDecodeToString([]byte{})
	_, _ = s, err

	// hpack: AppendHuffmanString
	dst := hpack.AppendHuffmanString(nil, "test")
	_ = dst

	// hpack: HuffmanDecode
	var buf bytes.Buffer
	nn, err2 := hpack.HuffmanDecode(&buf, []byte{})
	_, _ = nn, err2

	// hpack: NewEncoder + WriteField
	var w bytes.Buffer
	enc := hpack.NewEncoder(&w)
	err3 := enc.WriteField(hpack.HeaderField{Name: ":method", Value: "GET"})
	_ = err3
	enc.SetMaxDynamicTableSizeLimit(4096)

	// hpack: NewDecoder + Write + Close
	dec := hpack.NewDecoder(4096, func(hpack.HeaderField) {})
	_, err4 := dec.Write(w.Bytes())
	_ = err4
	dec.SetMaxDynamicTableSize(4096)
	dec.SetMaxStringLength(1024)
	dec.SetEmitEnabled(true)
	err5 := dec.Close()
	_ = err5

	// syncmap: Store, Load, LoadOrStore, Delete, Range
	var m syncmap.Map
	m.Store("key", "value")
	v, ok := m.Load("key")
	_, _ = v, ok
	actual, loaded := m.LoadOrStore("key2", "val2")
	_, _ = actual, loaded
	m.Delete("key")
	m.Range(func(k, v interface{}) bool {
		return true
	})
	v2, deleted := m.LoadAndDelete("key2")
	_, _ = v2, deleted
}
