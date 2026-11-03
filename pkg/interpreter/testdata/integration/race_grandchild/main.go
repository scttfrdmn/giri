// race_grandchild (#286 p4): a goroutine's child reads x while main writes x
// after its go statement. Native Go (-race): DATA RACE.
package main

import "sync"

var x int

func main() {
	x = 1
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		inner := make(chan bool)
		go func() { _ = x; inner <- true }()
		<-inner
	}()
	x = 5
	wg.Wait()
	println(x)
}
