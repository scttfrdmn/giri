// fmt_scan verifies that fmt.Scan/Scanf/Scanln/Fscan/Fscanf/Fscanln are
// correctly intercepted.
//
// Expected: 0 violations.
package main

import (
	"fmt"
	"strings"
)

func main() {
	// fmt.Scan — reads from stdin; returns (0, nil).
	var x int
	n, err := fmt.Scan(&x)
	if n < 0 {
		var s []int
		_ = s[0] // canary: n must be >= 0
	}
	_ = err

	// fmt.Scanf — reads from stdin.
	var y float64
	_, _ = fmt.Scanf("%f", &y)

	// fmt.Scanln — reads from stdin.
	var z string
	_, _ = fmt.Scanln(&z)

	// fmt.Fscan — reads from a reader.
	r := strings.NewReader("42")
	var v int
	_, _ = fmt.Fscan(r, &v)

	// fmt.Fscanf — reads from a reader with format.
	r2 := strings.NewReader("3.14")
	var f float64
	_, _ = fmt.Fscanf(r2, "%f", &f)

	// fmt.Fscanln — reads from a reader.
	r3 := strings.NewReader("hello")
	var s2 string
	_, _ = fmt.Fscanln(r3, &s2)

	// Sscan/Sscanf/Sscanln — already covered but verify still working.
	var a, b int
	_, _ = fmt.Sscan("1 2", &a, &b)
	if a < 0 || b < 0 {
		var xs []int
		_ = xs[0] // canary: scan should not produce negative values
	}
}
