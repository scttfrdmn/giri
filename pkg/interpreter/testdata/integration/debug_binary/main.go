// debug_binary verifies that debug/buildinfo, debug/elf, debug/macho, debug/pe,
// and debug/dwarf are correctly intercepted.
//
// Expected: 0 violations.
package main

import (
	"debug/buildinfo"
	"debug/dwarf"
	"debug/elf"
	"debug/macho"
	"debug/pe"
)

func main() {
	// debug/buildinfo: ReadFile.
	info, err := buildinfo.ReadFile("/dev/null")
	_ = err
	if info == nil {
		// opaque non-nil — this branch won't be taken
	}

	// debug/elf: Open.
	ef, err2 := elf.Open("/dev/null")
	_ = err2
	if ef == nil {
		// opaque — expected
	}

	// debug/macho: Open.
	mf, err3 := macho.Open("/dev/null")
	_ = err3
	if mf == nil {
		// opaque — expected
	}

	// debug/pe: Open.
	pf, err4 := pe.Open("/dev/null")
	_ = err4
	if pf == nil {
		// opaque — expected
	}

	// debug/dwarf: Data constructors.
	// dwarf.New takes 8 []byte section args.
	var empty []byte
	d, err5 := dwarf.New(empty, empty, empty, empty, empty, empty, empty, empty)
	_ = err5
	if d == nil {
		// opaque — expected
	}
}
