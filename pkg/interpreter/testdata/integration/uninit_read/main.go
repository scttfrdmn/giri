package main

// uninit_read exercises uninitialized memory detection (#24).
// Reads from a new(int) allocation before any write has occurred.
// Expected: >= 1 "uninitialized read" violation (requires Config.TrackInit=true).
func main() {
	x := new(int)
	_ = *x // read before any write — uninitialized
}
