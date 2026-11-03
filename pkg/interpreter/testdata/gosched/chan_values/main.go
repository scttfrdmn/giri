// chan_values: three workers square the jobs they receive and send the
// results back; main sums them. Native Go: the sum is 1+4+9+16+25 = 55, so
// trap indexes out of range. Before #295 channels carried no values.
package main

func trap(n int) {
	xs := make([]int, 55)
	_ = xs[n]
}

func main() {
	jobs, results := make(chan int), make(chan int)
	for w := 0; w < 3; w++ {
		go func() {
			for j := range jobs {
				results <- j * j
			}
		}()
	}
	go func() {
		for i := 1; i <= 5; i++ {
			jobs <- i
		}
		close(jobs)
	}()
	sum := 0
	for i := 0; i < 5; i++ {
		sum += <-results
	}
	trap(sum)
}
