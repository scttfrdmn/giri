// race_spawn_parent (#286 p2): main writes x after spawning a goroutine that
// also writes x. Native Go (-race): DATA RACE. Before #296 giri ticked the
// parent's clock before the child copied it, so the parent's later write
// looked ordered before the child's.
package main

var x int

func main() {
	x = 1
	done := make(chan bool)
	go func() { x = 2; done <- true }()
	x = 3
	<-done
	println(x)
}
