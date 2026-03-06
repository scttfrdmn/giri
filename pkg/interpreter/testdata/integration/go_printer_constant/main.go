// go_printer_constant verifies that go/printer, go/constant, go/scanner,
// and go/version are correctly intercepted.
//
// Expected: 0 violations.
package main

import (
	"go/constant"
	"go/scanner"
	"go/token"
	"go/version"
)

func main() {
	// go/constant: constructors and extractors.
	v := constant.MakeInt64(42)
	if v == nil {
		var s []int
		_ = s[0] // canary: constant must be non-nil
	}

	s := constant.MakeString("hello")
	_ = s

	sv := constant.StringVal(s)
	_ = sv

	f := constant.MakeFloat64(3.14)
	_ = f

	// go/constant: BinaryOp.
	sum := constant.BinaryOp(v, token.ADD, constant.MakeInt64(1))
	_ = sum

	// go/scanner: Init + Scan.
	fset := token.NewFileSet()
	file := fset.AddFile("", fset.Base(), 10)
	var sc scanner.Scanner
	sc.Init(file, []byte("hello"), nil, 0)
	pos, tok, lit := sc.Scan()
	_ = pos
	_ = tok
	_ = lit

	// go/version: Compare and IsValid.
	cmp := version.Compare("go1.21", "go1.22")
	_ = cmp

	valid := version.IsValid("go1.21.0")
	_ = valid

	lang := version.Lang("go1.21.4")
	_ = lang
}
