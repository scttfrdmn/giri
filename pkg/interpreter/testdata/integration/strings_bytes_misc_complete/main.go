// strings_bytes_misc_complete exercises additions from v0.91.0:
// strings: SplitAfterSeq, FieldsFuncSeq (Go 1.24+);
// bytes:   SplitSeq, FieldsSeq, Lines, SplitAfterSeq, FieldsFuncSeq (Go 1.24+);
// path/filepath: Localize (Go 1.26);
// errors: AsType[E] (Go 1.26+);
// fmt: FormatString (Go 1.21+).
// Expected: 0 violations.
package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"
)

// myState implements fmt.State for the FormatString test.
type myState struct{}

func (s *myState) Write(b []byte) (n int, err error) { return len(b), nil }
func (s *myState) Width() (wid int, ok bool)         { return 0, false }
func (s *myState) Precision() (prec int, ok bool)    { return 0, false }
func (s *myState) Flag(c int) bool                   { return false }

func main() {
	// strings: SplitAfterSeq (Go 1.24+).
	_ = strings.SplitAfterSeq("a,b,c", ",")

	// strings: FieldsFuncSeq (Go 1.24+).
	_ = strings.FieldsFuncSeq("hello world", unicode.IsSpace)

	// bytes: SplitSeq (Go 1.24+).
	_ = bytes.SplitSeq([]byte("a,b,c"), []byte(","))

	// bytes: FieldsSeq (Go 1.24+).
	_ = bytes.FieldsSeq([]byte("hello world"))

	// bytes: Lines (Go 1.24+).
	_ = bytes.Lines([]byte("line1\nline2\nline3"))

	// bytes: SplitAfterSeq (Go 1.24+).
	_ = bytes.SplitAfterSeq([]byte("a,b,c"), []byte(","))

	// bytes: FieldsFuncSeq (Go 1.24+).
	_ = bytes.FieldsFuncSeq([]byte("hello world"), unicode.IsSpace)

	// path/filepath: Localize (Go 1.26).
	local, err := filepath.Localize("dir/file.txt")
	_, _ = local, err

	// errors: AsType[E] (Go 1.26+).
	var someErr error = &os.PathError{Op: "open", Path: "/tmp/x", Err: os.ErrNotExist}
	pathErr, ok := errors.AsType[*os.PathError](someErr)
	_, _ = pathErr, ok

	// fmt: FormatString (Go 1.21+).
	s := fmt.FormatString(&myState{}, 'v')
	_ = s
}
