// weak_synctest verifies that weak, structs, testing/synctest, and
// testing/slogtest are correctly intercepted.
//
// Expected: 0 violations.
package main

import (
	"testing/synctest"
	"weak"
)

func main() {
	// weak: Make.
	x := 42
	ptr := weak.Make(&x)
	// weak.Pointer.Value may return nil (GC may have collected).
	val := ptr.Value()
	_ = val

	// testing/synctest: Wait (the only function callable without *testing.T).
	synctest.Wait()
}
