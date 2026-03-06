// expvar_tabwriter verifies that expvar, text/tabwriter, and text/scanner
// are correctly intercepted.
//
// Expected: 0 violations.
package main

import (
	"expvar"
	"os"
	"text/scanner"
	"text/tabwriter"
)

func main() {
	// expvar: publish and access variables.
	counter := expvar.NewInt("requests")
	counter.Add(1)
	_ = counter.Value()

	fs := expvar.NewFloat("latency")
	fs.Set(1.5)

	str := expvar.NewString("version")
	str.Set("v0.54.0")

	m := expvar.NewMap("stats")
	_ = m

	// expvar: Get.
	_ = expvar.Get("requests")

	// text/tabwriter: NewWriter and basic output.
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 1, ' ', 0)
	_, _ = w.Write([]byte("col1\tcol2\n"))
	_ = w.Flush()

	// text/scanner: Init and Scan.
	var s scanner.Scanner
	s.Init(nil) // nil reader is safe for intercept
	tok := s.Scan()
	_ = tok
	_ = s.TokenText()
}
