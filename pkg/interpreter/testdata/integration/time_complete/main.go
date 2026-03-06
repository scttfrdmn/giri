// time_complete exercises time package methods added in v0.72.0:
// Compare, AppendFormat, Clock, Location, IsDST, ZoneBounds, AddDate,
// Duration.Abs, LoadLocation, FixedZone.
// Expected: 0 violations.
package main

import "time"

func main() {
	t := time.Now()

	// Compare (Go 1.20).
	_ = t.Compare(t)

	// AppendFormat (Go 1.20).
	buf := []byte("prefix:")
	buf = t.AppendFormat(buf, time.RFC3339)
	_ = buf

	// Clock (hour, min, sec).
	h, m, s := t.Clock()
	_, _, _ = h, m, s

	// Location.
	loc := t.Location()
	_ = loc

	// IsDST (Go 1.17).
	_ = t.IsDST()

	// ZoneBounds (Go 1.19).
	start, end := t.ZoneBounds()
	_, _ = start, end

	// AddDate.
	t2 := t.AddDate(1, 0, 0)
	_ = t2

	// Duration.Abs (Go 1.19).
	d := -5 * time.Second
	_ = d.Abs()

	// LoadLocation.
	l, err := time.LoadLocation("UTC")
	_, _ = l, err

	// FixedZone.
	fz := time.FixedZone("EST", -5*60*60)
	_ = fz
}
