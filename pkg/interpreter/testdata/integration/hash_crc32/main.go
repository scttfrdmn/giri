// hash_crc32 verifies that hash/crc32, hash/fnv, and hash/adler32 intercepts work (#98).
//
// Expected: 0 violations.
package main

import (
	"hash/adler32"
	"hash/crc32"
	"hash/fnv"
)

func main() {
	// crc32.NewIEEE returns an opaque hash.Hash32.
	h := crc32.NewIEEE()
	n, err := h.Write([]byte("hello"))
	_ = n
	_ = err
	sum := h.Sum32()
	_ = sum
	h.Reset()
	_ = h.Size()
	_ = h.BlockSize()

	// crc32.ChecksumIEEE computes the CRC directly.
	cs := crc32.ChecksumIEEE([]byte("hello"))
	_ = cs

	// crc32.MakeTable returns opaque.
	tbl := crc32.MakeTable(crc32.IEEE)
	_ = tbl

	// hash/fnv.
	h2 := fnv.New32()
	h2.Write([]byte("world"))
	_ = h2.Sum32()

	h3 := fnv.New64a()
	h3.Write([]byte("world"))
	_ = h3.Sum64()

	// hash/adler32.
	h4 := adler32.New()
	h4.Write([]byte("giri"))
	_ = h4.Sum32()
	_ = adler32.Checksum([]byte("giri"))
}
