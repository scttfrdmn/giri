// binary_sql_bigint_complete exercises additions from v0.76.0:
// encoding/binary: AppendUint16/32/64 (AppendByteOrder methods, Go 1.21),
// AppendVarint/AppendUvarint (package-level, Go 1.22),
// ReadVarint/ReadUvarint (package-level);
// database/sql: Drivers;
// math/big: MulRange, Binomial.
// Expected: 0 violations.
package main

import (
	"bytes"
	"database/sql"
	"encoding/binary"
	"math/big"
)

func main() {
	// encoding/binary: AppendUint16/32/64 (methods on LittleEndian/BigEndian).
	buf := make([]byte, 0, 24)
	buf = binary.LittleEndian.AppendUint16(buf, 0x1234)
	buf = binary.LittleEndian.AppendUint32(buf, 0x12345678)
	buf = binary.LittleEndian.AppendUint64(buf, 0x123456789abcdef0)
	_ = buf

	// encoding/binary: AppendUvarint / AppendVarint (package-level, Go 1.22).
	var vbuf []byte
	vbuf = binary.AppendUvarint(vbuf, 150)
	vbuf = binary.AppendVarint(vbuf, -1)
	_ = vbuf

	// encoding/binary: ReadVarint / ReadUvarint.
	encoded := []byte{0x96, 0x01} // 150 as uvarint
	r := bytes.NewReader(encoded)
	uv, err := binary.ReadUvarint(r)
	_, _ = uv, err

	r2 := bytes.NewReader([]byte{0x02}) // 1 as varint (zigzag)
	sv, err2 := binary.ReadVarint(r2)
	_, _ = sv, err2

	// database/sql: Drivers.
	drivers := sql.Drivers()
	_ = drivers

	// math/big: MulRange.
	result := new(big.Int).MulRange(1, 10)
	_ = result

	// math/big: Binomial.
	binom := new(big.Int).Binomial(10, 3)
	_ = binom
}
