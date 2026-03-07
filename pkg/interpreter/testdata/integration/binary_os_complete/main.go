// binary_os_complete exercises additions from v0.77.0:
// encoding/binary: Encode/Decode/Append (Go 1.23 generic bulk helpers);
// os: NewSyscallError, CopyFS (Go 1.23), Expand.
// Expected: 0 violations.
package main

import (
	"encoding/binary"
	"os"
	"testing/fstest"
)

func main() {
	// encoding/binary: Encode (Go 1.23).
	buf := make([]byte, 8)
	n, err := binary.Encode(buf, binary.LittleEndian, uint64(0x0102030405060708))
	_, _ = n, err

	// encoding/binary: Decode (Go 1.23).
	var v uint64
	m, err2 := binary.Decode(buf, binary.LittleEndian, &v)
	_, _ = m, err2

	// encoding/binary: Append (Go 1.23).
	out, err3 := binary.Append(nil, binary.BigEndian, uint32(42))
	_, _ = out, err3

	// os.NewSyscallError.
	sErr := os.NewSyscallError("open", nil)
	_ = sErr

	// os.CopyFS (Go 1.23).
	fsys := fstest.MapFS{
		"hello.txt": {Data: []byte("hello")},
	}
	copyErr := os.CopyFS("/tmp/giri-copyfs-test", fsys)
	_ = copyErr

	// os.Expand.
	expanded := os.Expand("hello $NAME", func(key string) string {
		return ""
	})
	_ = expanded
}
