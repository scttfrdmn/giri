// leak_no_defers: a goroutine waits on a channel nobody sends on; main
// returns. Native Go: the goroutine never resumes, so its deferred call never
// runs. giri reports the leak, and must not run the deferred nil dereference.
package main

func main() {
	ch := make(chan int)
	go func() {
		defer func() {
			var p *int
			*p = 1
		}()
		<-ch
	}()
}
