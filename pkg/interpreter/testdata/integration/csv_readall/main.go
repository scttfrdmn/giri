// csv_readall verifies that encoding/csv intercepts work (#87).
//
// Expected: 0 violations.
package main

import (
	"encoding/csv"
	"strings"
)

func main() {
	// csv.NewReader wraps an io.Reader.
	r := csv.NewReader(strings.NewReader("a,b,c\n1,2,3\n"))
	_ = r

	// Read returns one record at a time.
	record, err := r.Read()
	_ = record // []string{"a", "b", "c"} or sentinel
	_ = err    // nil

	// ReadAll returns all records.
	r2 := csv.NewReader(strings.NewReader("x,y\n1,2\n"))
	records, err2 := r2.ReadAll()
	_ = records
	_ = err2

	// csv.NewWriter wraps an io.Writer.
	var sb strings.Builder
	w := csv.NewWriter(&sb)
	_ = w

	// Write a single record.
	err3 := w.Write([]string{"hello", "world"})
	_ = err3

	// Flush ensures all buffered output is written.
	w.Flush()
	_ = w.Error()

	// WriteAll writes multiple records.
	w2 := csv.NewWriter(&sb)
	err4 := w2.WriteAll([][]string{{"a", "b"}, {"c", "d"}})
	_ = err4
}
