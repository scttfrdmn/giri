// race_after_send (#286 p3): G1 writes a, sends; G2 receives and writes a (no
// race). G1 writes b after the send while G2 writes b: a race on b only.
// Native Go (-race): one DATA RACE, on b.
package main

var a, b [4]int

func main() {
	ch := make(chan bool)
	done := make(chan bool)
	go func() {
		for i := range a {
			a[i] = 1
		}
		ch <- true
		b[0] = 1
		done <- true
	}()
	go func() {
		<-ch
		for i := range a {
			a[i] = 2
		}
		b[0] = 2
		done <- true
	}()
	<-done
	<-done
	println(a[0], b[0])
}
