// make_valid verifies that valid make() calls produce no violations.
// Expected: 0 violations.
package main

func main() {
	s := make([]int, 5, 10)
	ch := make(chan int, 4)
	m := make(map[string]int)

	s[0] = 1
	ch <- 1
	m["key"] = 1

	_ = <-ch
	_ = s
	_ = m
}
