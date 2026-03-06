// compress_extras verifies that compress/bzip2, compress/flate, and
// compress/lzw are correctly intercepted.
//
// Expected: 0 violations.
package main

import (
	"compress/bzip2"
	"compress/flate"
	"compress/lzw"
)

func main() {
	// compress/bzip2: NewReader.
	br := bzip2.NewReader(nil)
	_ = br

	// compress/flate: NewReader.
	fr := flate.NewReader(nil)
	_ = fr

	// compress/flate: NewWriter.
	fw, err := flate.NewWriter(nil, flate.DefaultCompression)
	_ = err
	if fw == nil {
		var s []int
		_ = s[0] // canary: writer must be non-nil
	}

	// compress/lzw: NewReader.
	lr := lzw.NewReader(nil, lzw.MSB, 8)
	_ = lr

	// compress/lzw: NewWriter.
	lw := lzw.NewWriter(nil, lzw.MSB, 8)
	_ = lw
}
