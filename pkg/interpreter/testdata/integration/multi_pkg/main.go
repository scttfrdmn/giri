// multi_pkg verifies that Giri interprets function bodies defined in imported
// user packages, not just the top-level main package (#53).
//
// main calls lib.UnsafeRead which contains a misaligned unsafe.Pointer
// access (Rule 1). Because LoadProgram uses NeedDeps and prog.Build(),
// the SSA for lib is available; execFunction interprets it as-is.
//
// Expected: 1 violation (unsafe Rule 1 from lib.UnsafeRead).
package main

import "github.com/scttfrdmn/giri/pkg/interpreter/testdata/integration/multi_pkg/lib"

func main() {
	buf := []byte{0x00, 0x01, 0x02, 0x03, 0x04}
	_ = lib.UnsafeRead(buf)
}
