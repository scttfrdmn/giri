// time_ticker verifies that time.NewTicker, NewTimer, Sleep, Now, Since,
// Until and Timer/Ticker method intercepts work (#93).
//
// Expected: 0 violations.
package main

import "time"

func main() {
	// Sleep is a noop.
	time.Sleep(10 * time.Millisecond)

	// NewTicker returns an opaque *Ticker; Stop returns bool.
	ticker := time.NewTicker(time.Second)
	ticker.Stop()
	ticker.Reset(time.Second)

	// NewTimer returns an opaque *Timer; Stop returns bool.
	timer := time.NewTimer(time.Second)
	timer.Stop()
	timer.Reset(time.Second)

	// Now returns an opaque time.Time.
	t := time.Now()
	_ = t

	// Since and Until return durations.
	_ = time.Since(t)
	_ = time.Until(t)

	// ParseDuration returns (duration, error).
	d, err := time.ParseDuration("1s")
	_ = d
	_ = err

	// Parse returns (time.Time, error).
	t2, err2 := time.Parse(time.RFC3339, "2006-01-02T15:04:05Z")
	_ = t2
	_ = err2
}
