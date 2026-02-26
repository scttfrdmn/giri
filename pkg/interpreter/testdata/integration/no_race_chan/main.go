package main

// no_race_chan exercises the RaceDetector's channel happens-before tracking (#23).
// Goroutine B (writer, GID 3) writes to *x then signals on ch.
// Goroutine A (reader, GID 2) receives from ch then reads *x.
// The round-robin scheduler picks the higher GID first, so the writer runs
// before the reader; the channel send propagates the writer's vector clock to
// the reader, establishing happens-before → no race.
func main() {
	x := new(int)
	ch := make(chan struct{})
	// Reader: spawned first (GID 2), runs second (round-robin picks GID 3 first)
	go func(p *int, c chan struct{}) {
		<-c    // receive: merges writer's clock → happens-before established
		_ = *p // read after sync: no race
	}(x, ch)
	// Writer: spawned second (GID 3), runs first
	go func(p *int, c chan struct{}) {
		*p = 1
		c <- struct{}{} // signal: propagates writer's clock to channel
	}(x, ch)
}
