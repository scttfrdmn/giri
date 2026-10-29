// Package stdimports imports every package in giri's concrete intercept
// table so tests can type-check the table's function names.
package stdimports

import (
	_ "math"
	_ "math/bits"
	_ "sort"
	_ "strconv"
	_ "strings"
	_ "unicode"
	_ "unicode/utf8"
)
